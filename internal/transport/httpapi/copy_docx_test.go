package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
)

func TestCopyDOCXUsesCommittedBytesAndIndependentSession(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "original.docx")
	if _, err := docx.Create(ctx, path, []docx.InitialBlock{{Type: "paragraph", Text: "원본"}}); err != nil {
		t.Fatal(err)
	}
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	request := documentWorkspaceRequest(t, api)
	open := request("POST", "/api/documents", "", map[string]string{"path": path}, true)
	var source openedDocument
	if open.Code != 200 || json.Unmarshal(open.Body.Bytes(), &source) != nil {
		t.Fatal(open.Code, open.Body.String())
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	copyFile := func(revision uint64, name string) (int, []byte) {
		w := request("POST", "/api/documents/copy-docx", "", map[string]any{
			"name": name, "sessionId": source.ID, "baseRevision": revision,
		}, true)
		return w.Code, w.Body.Bytes()
	}
	if status, _ := copyFile(2, "복사"); status != http.StatusConflict {
		t.Fatal("stale copy accepted", status)
	}
	status, body := copyFile(1, "복사")
	var copied openedDocument
	if status != http.StatusCreated || json.Unmarshal(body, &copied) != nil {
		t.Fatal(status, string(body))
	}
	if copied.ID == source.ID || copied.Path != "" || copied.Name != "복사.docx" || copied.Origin != "copy" {
		t.Fatalf("invalid copy descriptor: %+v", copied)
	}
	download := request("GET", "/api/download?document="+copied.ID+"&session="+copied.ID+"&revision=1", "", nil, true)
	if download.Code != http.StatusOK || !bytes.Equal(original, download.Body.Bytes()) {
		t.Fatal("copy did not preserve the saved bytes", download.Code)
	}
	if status, _ := copyFile(1, "복사.DOCX"); status != http.StatusConflict {
		t.Fatal("duplicate name accepted", status)
	}
	if err := os.WriteFile(path, []byte("external replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if status, _ := copyFile(1, "다른 복사"); status != http.StatusConflict {
		t.Fatal("externally changed source accepted", status)
	}
}
