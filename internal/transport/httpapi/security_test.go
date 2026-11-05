package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
	"github.com/xuri/excelize/v2"
)

const (
	securityBrowserToken = "browser-secret-token"
	securityMCPToken     = "mcp-secret-token"
	securityCellContent  = "private-cell-content"
)

func newSecurityServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "security.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", securityCellContent); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	session, err := xlsx.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return NewWithTokens(session, securityBrowserToken, securityMCPToken), path
}

func TestBrowserRoutesRejectMissingInvalidAndCrossRoleCredentials(t *testing.T) {
	server, _ := newSecurityServer(t)
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/", ""},
		{http.MethodGet, "/?token=" + securityMCPToken, ""},
		{http.MethodGet, "/assets/not-found.js", ""},
		{http.MethodGet, "/api/workbook", ""},
		{http.MethodGet, "/api/download?document=test&session=test&revision=1", ""},
		{http.MethodPost, "/api/save?document=test", `{ "sessionId": "test", "baseRevision": 1 }`},
		{http.MethodGet, "/api/search?query=private", ""},
		{http.MethodGet, "/api/range?sheet=Sheet1&range=A1", ""},
		{http.MethodGet, "/api/sheet-objects?sheet=Sheet1", ""},
		{http.MethodGet, "/api/events", ""},
		{http.MethodPost, "/api/operations", `{"baseRevision":1,"operations":[]}`},
		{http.MethodPost, "/api/history/undo", `{"baseRevision":1}`},
		{http.MethodPost, "/api/presence", `{"sheet":"Sheet1","range":"A1","state":"selecting"}`},
	}
	credentials := []struct {
		name   string
		cookie *http.Cookie
	}{
		{"missing", nil},
		{"invalid", &http.Cookie{Name: "xlsx_session", Value: "wrong-token"}},
		{"MCP token", &http.Cookie{Name: "xlsx_session", Value: securityMCPToken}},
	}
	for _, route := range routes {
		for _, credential := range credentials {
			t.Run(route.method+" "+route.path+" "+credential.name, func(t *testing.T) {
				response := httptest.NewRecorder()
				request := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				if credential.cookie != nil {
					request.AddCookie(credential.cookie)
				}
				server.Handler().ServeHTTP(response, request)
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
				}
				assertNoSensitiveResponse(t, response)
			})
		}
	}
}

func TestTokenizedBrowserURLSetsStrictCookieAndRemovesQuery(t *testing.T) {
	server, _ := newSecurityServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?token="+securityBrowserToken, nil)
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("tokenized browser response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Value != securityBrowserToken || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge <= 0 {
		t.Fatalf("browser session cookie = %#v", cookie)
	}
}

func TestMCPRouteRejectsMissingInvalidAndBrowserCredentials(t *testing.T) {
	server, _ := newSecurityServer(t)
	for name, authorization := range map[string]string{
		"missing":       "",
		"invalid":       "Bearer wrong-token",
		"browser token": "Bearer " + securityBrowserToken,
		"wrong scheme":  "Basic " + securityMCPToken,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
			request.Header.Set("Authorization", authorization)
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			if response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("WWW-Authenticate = %q", response.Header().Get("WWW-Authenticate"))
			}
			assertNoSensitiveResponse(t, response)
		})
	}
}

func TestRejectedOriginDoesNotMutateWorkbook(t *testing.T) {
	server, path := newSecurityServer(t)
	beforeBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeRevision := server.session.Snapshot().Revision
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/operations", strings.NewReader(`{
		"baseRevision":1,
		"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"new-private-value"}]
	}`))
	request.Host = "127.0.0.1:8765"
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "xlsx_session_8765", Value: securityBrowserToken})
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	afterBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeBytes, afterBytes) || server.session.Snapshot().Revision != beforeRevision {
		t.Fatal("rejected cross-origin write changed the workbook")
	}
	assertNoSensitiveResponse(t, response)
}

func TestErrorPayloadDoesNotEchoCredentialsOrCellContent(t *testing.T) {
	server, _ := newSecurityServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/operations", strings.NewReader(`{
		"baseRevision":1,
		"operations":[{"type":"set_cell","sheet":"Missing","cell":"A1","value":"private-cell-content"}]
	}`))
	request.Host = "127.0.0.1:8765"
	request.Header.Set("Origin", "http://127.0.0.1:8765")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+securityMCPToken)
	request.AddCookie(&http.Cookie{Name: "xlsx_session_8765", Value: securityBrowserToken})
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	assertNoSensitiveResponse(t, response)
}

func TestSecurityHeadersCoverSuccessAndFailureResponses(t *testing.T) {
	server, _ := newSecurityServer(t)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/healthz", nil),
		httptest.NewRequest(http.MethodGet, "/api/workbook", nil),
	}
	requests[1].AddCookie(&http.Cookie{Name: "xlsx_session", Value: securityBrowserToken})
	for _, request := range requests {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		for name, want := range map[string]string{
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
			"Referrer-Policy":              "no-referrer",
			"Cross-Origin-Resource-Policy": "same-origin",
			"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
		} {
			if got := response.Header().Get(name); got != want {
				t.Fatalf("%s = %q, want %q", name, got, want)
			}
		}
		policy := response.Header().Get("Content-Security-Policy")
		for _, directive := range []string{"default-src 'self'", "base-uri 'none'", "object-src 'none'", "frame-ancestors 'none'", "form-action 'none'", "connect-src 'self'"} {
			if !strings.Contains(policy, directive) {
				t.Fatalf("Content-Security-Policy %q is missing %q", policy, directive)
			}
		}
	}
}

func TestExpiredAndInvalidLaunchLinksDoNotSetSessionCookies(t *testing.T) {
	server, _ := newSecurityServer(t)
	server.launchTokens["expired"] = time.Now().Add(-time.Second)
	for _, token := range []string{"expired", "unknown", securityBrowserToken, securityMCPToken} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/launch/"+token, nil)
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("launch %q status = %d, want %d", token, response.Code, http.StatusUnauthorized)
		}
		if len(response.Result().Cookies()) != 0 {
			t.Fatalf("launch %q set cookies: %#v", token, response.Result().Cookies())
		}
		assertNoSensitiveResponse(t, response)
	}
}

func assertNoSensitiveResponse(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	content := response.Body.String()
	for _, secret := range []string{securityBrowserToken, securityMCPToken, securityCellContent, "new-private-value"} {
		if strings.Contains(content, secret) {
			t.Fatalf("response exposed sensitive value %q: %s", secret, content)
		}
	}
}

func TestBrowserSessionsStayIndependentAcrossPorts(t *testing.T) {
	workbook, _ := newSecurityServer(t)
	const docxToken = "another-browser-secret"
	document := NewWithTokens(nil, docxToken, "another-mcp-secret")
	t.Cleanup(func() { _ = document.Close() })
	snapshot, err := document.createDOCX(t.Context(), docxCreateInput{
		Path:   filepath.Join(t.TempDir(), "sample.docx"),
		Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "original"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	bookHTTP := httptest.NewServer(workbook.Handler())
	defer bookHTTP.Close()
	docHTTP := httptest.NewServer(document.Handler())
	defer docHTTP.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	request := func(method, base, path string, body []byte, status int) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, base+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if method == http.MethodPost {
			req.Header.Set("Origin", base)
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s %s returned %d, want %d", method, path, response.StatusCode, status)
		}
	}
	request(http.MethodGet, bookHTTP.URL, "/?token="+securityBrowserToken, nil, http.StatusOK)
	request(http.MethodGet, docHTTP.URL, "/api/docx", nil, http.StatusUnauthorized)
	request(http.MethodGet, docHTTP.URL, "/docx?token="+docxToken, nil, http.StatusOK)
	request(http.MethodPost, bookHTTP.URL, "/api/operations", []byte(`{"baseRevision":1,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"updated"}]}`), http.StatusOK)
	run := snapshot.Blocks[0].Paragraph.Runs[0]
	input, err := json.Marshal(docx.ApplyInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision,
		Operations: []docx.Operation{{Type: "replace_run_text", TargetID: run.ID, ExpectedText: run.Text, Text: "updated"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request(http.MethodPost, docHTTP.URL, "/api/docx/operations", input, http.StatusOK)
}
