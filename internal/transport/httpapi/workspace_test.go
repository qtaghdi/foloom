package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/qtaghdi/foloom/internal/formats/xlsx"
)

func workspaceRequest(api *Server, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.AddCookie(&http.Cookie{Name: "xlsx_session", Value: securityBrowserToken})
	request.Header.Set("Origin", "http://example.com")
	api.Handler().ServeHTTP(response, request)
	return response
}
func openWorkspaceFile(t *testing.T, api *Server, path string) openedDocument {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"path": path})
	response := workspaceRequest(api, "POST", "/api/documents", string(body))
	if response.Code != 200 {
		t.Fatalf("open document: %d %s", response.Code, response.Body)
	}
	var document openedDocument
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	return document
}
func TestWorkspaceDocumentIsolationAndClosedIDs(t *testing.T) {
	api, firstPath := newSecurityServer(t)
	defer api.Close()
	first := openWorkspaceFile(t, api, firstPath)
	content, _ := os.ReadFile(firstPath)
	secondPath := filepath.Join(t.TempDir(), "other.xlsx")
	if err := os.WriteFile(secondPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	second := openWorkspaceFile(t, api, secondPath)
	if first.ID == second.ID {
		t.Fatal("files share a session")
	}
	alias := filepath.Join(t.TempDir(), "alias.xlsx")
	if err := os.Symlink(secondPath, alias); err != nil {
		t.Fatal(err)
	}
	if repeated := openWorkspaceFile(t, api, alias); repeated.ID != second.ID {
		t.Fatal("duplicate path reopened")
	}
	write := `{"baseRevision":1,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"second only"}]}`
	if response := workspaceRequest(api, "POST", "/api/operations", write); response.Code != 400 && response.Code != 409 {
		t.Fatalf("ambiguous write accepted: %d", response.Code)
	}
	response := workspaceRequest(api, "POST", "/api/operations?document="+second.ID, write)
	if response.Code != 200 {
		t.Fatalf("scoped write: %d %s", response.Code, response.Body)
	}

	explicit := fmt.Sprintf(`{"documentId":%q,"baseRevision":2,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"second only"}]}`, second.ID)
	if response := workspaceRequest(api, "POST", "/api/operations", explicit); response.Code != 200 {
		t.Fatalf("explicit target write: %d %s", response.Code, response.Body)
	}
	mismatch := fmt.Sprintf(`{"documentId":%q,"baseRevision":3,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"wrong document"}]}`, first.ID)
	if response := workspaceRequest(api, "POST", "/api/operations?document="+second.ID, mismatch); response.Code != 409 {
		t.Fatalf("mismatched target accepted: %d", response.Code)
	}
	response = workspaceRequest(api, "GET", "/api/range?document="+first.ID+"&sheet=Sheet1&range=A1", "")
	var cells xlsx.Range
	if err := json.Unmarshal(response.Body.Bytes(), &cells); err != nil || cells.Rows[0][0].Value != securityCellContent {
		t.Fatalf("first document changed: %+v %v", cells, err)
	}
	if response := workspaceRequest(api, "GET", "/api/range?document="+second.ID+"&sheet=Sheet1&range=A1", ""); !bytes.Contains(response.Body.Bytes(), []byte("second only")) {
		t.Fatal("second document did not change")
	}
	if response := workspaceRequest(api, "DELETE", "/api/documents/"+second.ID, ""); response.Code != 204 {
		t.Fatalf("close status: %d", response.Code)
	}
	for _, method := range []string{"GET", "POST"} {
		path := "/api/workbook?document=" + second.ID
		if method == "POST" {
			path = "/api/operations?document=" + second.ID
		}
		if response := workspaceRequest(api, method, path, write); response.Code != 404 {
			t.Fatalf("closed ID fell back: %d", response.Code)
		}
	}
	if response := workspaceRequest(api, "GET", "/api/workbook?document="+first.ID, ""); response.Code != 200 {
		t.Fatal("closing other file closed first")
	}
}
func TestWorkspaceMixedFormatsLimitAndAuthentication(t *testing.T) {
	api, path := newSecurityServer(t)
	defer api.Close()
	docxPath, _ := filepath.Abs("../../../testdata/docx/compatibility/generated-basic.docx")
	document := openWorkspaceFile(t, api, docxPath)
	if document.Kind != "docx" {
		t.Fatalf("kind: %s", document.Kind)
	}
	if response := workspaceRequest(api, "GET", "/api/docx?document="+document.ID, ""); response.Code != 200 {
		t.Fatalf("DOCX read: %d", response.Code)
	}
	content, _ := os.ReadFile(path)
	for i := 2; i < maxOpenDocuments; i++ {
		next := filepath.Join(t.TempDir(), fmt.Sprintf("file%d.xlsx", i))
		if err := os.WriteFile(next, content, 0600); err != nil {
			t.Fatal(err)
		}
		openWorkspaceFile(t, api, next)
	}
	overflow := filepath.Join(t.TempDir(), "overflow.xlsx")
	os.WriteFile(overflow, content, 0600)
	body, _ := json.Marshal(map[string]string{"path": overflow})
	if response := workspaceRequest(api, "POST", "/api/documents", string(body)); response.Code != 409 {
		t.Fatalf("limit: %d", response.Code)
	}
	for _, route := range []string{"/api/documents", "/api/docx?document=" + document.ID} {
		response := httptest.NewRecorder()
		api.Handler().ServeHTTP(response, httptest.NewRequest("GET", route, nil))
		if response.Code != 401 {
			t.Fatalf("missing auth: %d", response.Code)
		}
	}
	request := httptest.NewRequest("POST", "/api/documents", bytes.NewReader(body))
	request.AddCookie(&http.Cookie{Name: "xlsx_session", Value: securityBrowserToken})
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatalf("cross origin: %d", response.Code)
	}
}
func TestWorkspaceFramePolicy(t *testing.T) {
	api, _ := newSecurityServer(t)
	defer api.Close()
	for path, want := range map[string]string{"/": "DENY", "/?document=example&embedded=1": "SAMEORIGIN"} {
		response := workspaceRequest(api, "GET", path, "")
		if response.Header().Get("X-Frame-Options") != want {
			t.Fatalf("frame policy: %s", response.Header().Get("X-Frame-Options"))
		}
	}
}

func TestClosedWorkspaceDoesNotReopenSessions(t *testing.T) {
	api, path := newSecurityServer(t)
	if err := api.Close(); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"path": path})
	response := workspaceRequest(api, "POST", "/api/documents", string(body))
	if response.Code != 409 {
		t.Fatalf("closed workspace reopened: %d", response.Code)
	}
}
