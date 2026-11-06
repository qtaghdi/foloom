package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/qtaghdi/foloom/internal/formats/docx"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func uploadRequest(t *testing.T, api *Server, names []string, content []byte, authorized bool, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range names {
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()
	request := httptest.NewRequest("POST", "/api/documents/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", origin)
	if authorized {
		request.AddCookie(&http.Cookie{Name: "xlsx_session", Value: securityBrowserToken})
	}
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	return response
}

func TestUploadWorkingCopyIsolationAndCleanup(t *testing.T) {
	api, source := newSecurityServer(t)
	defer api.Close()
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	response := uploadRequest(t, api, []string{"로컬 문서.xlsx"}, original, true, "http://example.com")
	if response.Code != 200 {
		t.Fatalf("upload: %d %s", response.Code, response.Body)
	}
	var opened openedDocument
	if err := json.Unmarshal(response.Body.Bytes(), &opened); err != nil {
		t.Fatal(err)
	}
	if !opened.Uploaded || opened.Path != "" || opened.Name != "로컬 문서.xlsx" {
		t.Fatalf("metadata: %+v", opened)
	}
	retained := api.documents[opened.ID]
	path := retained.server.session.SourcePath()
	dir := retained.cleanupDir
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("working directory is not private", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("working file is not private", err)
	}
	write := `{"baseRevision":1,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"upload edit"}]}`
	if got := workspaceRequest(api, "POST", "/api/operations?document="+opened.ID, write); got.Code != 200 {
		t.Fatal(got.Body)
	}
	after, _ := os.ReadFile(source)
	if !bytes.Equal(original, after) {
		t.Fatal("source was overwritten")
	}
	saved, _ := os.ReadFile(path)
	if bytes.Equal(saved, original) {
		t.Fatal("working copy did not save")
	}
	if got := workspaceRequest(api, "DELETE", "/api/documents/"+opened.ID, ""); got.Code != 204 {
		t.Fatal(got.Body)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("working directory survived close", err)
	}
	response = uploadRequest(t, api, []string{"second.xlsx"}, original, true, "http://example.com")
	json.Unmarshal(response.Body.Bytes(), &opened)
	dir = api.documents[opened.ID].cleanupDir
	api.Close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("working directory survived shutdown", err)
	}
}

func TestUploadRejectsUnauthenticatedMalformedAndOversizedFiles(t *testing.T) {
	api, source := newSecurityServer(t)
	defer api.Close()
	content, _ := os.ReadFile(source)
	for _, test := range []struct {
		name   string
		names  []string
		data   []byte
		auth   bool
		origin string
	}{
		{"authentication", []string{"a.xlsx"}, content, false, "http://example.com"},
		{"origin", []string{"a.xlsx"}, content, true, "https://evil.example"},
		{"unsupported", []string{"a.exe"}, content, true, "http://example.com"},
		{"invalid package", []string{"a.docx"}, []byte("not a docx"), true, "http://example.com"},
		{"multiple parts", []string{"a.xlsx", "b.xlsx"}, content, true, "http://example.com"},
		{"empty", []string{"a.xlsx"}, nil, true, "http://example.com"},
		{"oversized docx", []string{"a.docx"}, make([]byte, (32<<20)+1), true, "http://example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := uploadRequest(t, api, test.names, test.data, test.auth, test.origin)
			if response.Code < 400 {
				t.Fatal("invalid upload accepted", response.Body)
			}
		})
	}
	if len(api.documents) != 1 {
		t.Fatal("invalid uploads registered sessions")
	}
}

func TestUploadDOCXPreservesBytes(t *testing.T) {
	api, _ := newSecurityServer(t)
	defer api.Close()
	path := filepath.Join(t.TempDir(), "source.docx")
	if _, err := docx.Create(context.Background(), path, []docx.InitialBlock{{Type: "paragraph", Text: "한글 문서"}}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	uploaded := uploadRequest(t, api, []string{"문서.docx"}, content, true, "http://example.com")
	if uploaded.Code != 200 {
		t.Fatal(uploaded.Body)
	}
	var opened openedDocument
	json.Unmarshal(uploaded.Body.Bytes(), &opened)
	file, err := os.Open(api.documents[opened.ID].server.docxSession.SourcePath())
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got, _ := io.ReadAll(file)
	if !bytes.Equal(content, got) {
		t.Fatal("upload changed package bytes")
	}
}
