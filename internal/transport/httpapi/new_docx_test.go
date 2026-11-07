package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
)

func TestNewDOCXNameValidation(t *testing.T) {
	for _, name := range []string{"../escape", "/tmp/name", `C:\\document`, "CON.docx", "bad\x00name", ".hidden", "wrong.pdf", strings.Repeat("한", 100)} {
		if _, err := docxName(name); err == nil {
			t.Fatal("invalid name accepted", name)
		}
	}
	for _, pair := range [][2]string{{"메모", "메모.docx"}, {"회의.최종", "회의.최종.docx"}, {"한글", "한글.docx"}, {"report.DOCX", "report.DOCX"}} {
		got, err := docxName(pair[0])
		if err != nil || got != pair[1] {
			t.Fatalf("name %q: %q %v", pair[0], got, err)
		}
	}
}
func TestNewDOCXWorkspaceCreatesEditablePrivateCopy(t *testing.T) {
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	request := documentWorkspaceRequest(t, api)
	input := map[string]string{"name": "한글 메모"}
	if w := request("POST", "/api/documents/new-docx", "", input, false); w.Code != 401 {
		t.Fatal("unauthenticated create accepted", w.Code)
	}
	if w := request("POST", "/api/documents/new-docx", "https://evil.example", input, true); w.Code != 403 {
		t.Fatal("cross-origin create accepted", w.Code)
	}
	w := request("POST", "/api/documents/new-docx", "", input, true)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	var opened openedDocument
	if err := json.Unmarshal(w.Body.Bytes(), &opened); err != nil {
		t.Fatal(err)
	}
	if opened.Name != "한글 메모.docx" || opened.Path != "" || !opened.Uploaded || opened.Origin != "new" {
		t.Fatalf("invalid working copy descriptor %+v", opened)
	}
	registered := api.documents[opened.ID]
	path := registered.server.docxSession.SourcePath()
	dir := registered.cleanupDir
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("working copy permissions", err)
	}
	v, err := registered.server.docxSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p := v.Blocks[0].Paragraph
	if v.Revision != 1 || len(v.Blocks) != 1 || !p.StructureEditable || p.Runs[0].Text != "" || !p.Runs[0].Editable {
		t.Fatal("new document not editable", v)
	}
	active, err := api.docxSnapshot(context.Background())
	if err != nil || active.ID != v.ID {
		t.Fatal("new document not available to MCP", err)
	}
	if w := request("POST", "/api/documents/new-docx", "", map[string]string{"name": "한글 메모.docx"}, true); w.Code != 409 {
		t.Fatal("duplicate name accepted", w.Code)
	}
	edit := docx.ApplyInput{SessionID: v.ID, BaseRevision: v.Revision, Operations: []docx.Operation{{Type: "replace_run_text", TargetID: p.Runs[0].ID, ExpectedText: "", Text: "새 문서 작성"}}}
	w = request("POST", "/api/docx/operations?document="+v.ID, "", edit, true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request("GET", "/api/download?document="+v.ID+"&session="+v.ID+"&revision=2", "", nil, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, w.Body.Bytes()) {
		t.Fatal("download differs from saved document", err)
	}
	w = request("DELETE", "/api/documents/"+v.ID, "", nil, true)
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	if _, err = os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("closed working copy not removed", err)
	}
}

func documentWorkspaceRequest(t *testing.T, api *Server) func(string, string, string, any, bool) *httptest.ResponseRecorder {
	t.Helper()
	return func(method, path, origin string, body any, authenticated bool) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&b).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, path, &b)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "browser-token"})
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, r)
		return w
	}
}
