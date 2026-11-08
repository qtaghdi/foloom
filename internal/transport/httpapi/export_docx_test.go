package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
)

func TestDOCXDraftExportNeverSavesTheSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.docx")
	session, err := docx.Create(context.Background(), path, []docx.InitialBlock{{Type: "paragraph", Text: "saved"}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	base, err := api.OpenDOCX(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	request := documentWorkspaceRequest(t, api)
	payload := map[string]any{
		"sessionId": base.ID, "baseRevision": base.Revision, "name": "exported.docx",
		"operations": []docx.Operation{{Type: "replace_run_text", TargetID: base.Blocks[0].Paragraph.Runs[0].ID, ExpectedText: "saved", Text: "unsaved"}},
	}
	w := request("POST", "/api/docx/export?document="+base.ID, "", payload, true)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("X-Document-ID") != base.ID || w.Header().Get("X-Document-Revision") != "1" {
		t.Fatal("wrong export revision")
	}
	exported := filepath.Join(t.TempDir(), "exported.docx")
	if err := os.WriteFile(exported, w.Body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	copy, err := docx.Open(context.Background(), exported)
	if err != nil {
		t.Fatal(err)
	}
	content, err := copy.Snapshot(context.Background())
	if err != nil || content.Blocks[0].Paragraph.Runs[0].Text != "unsaved" {
		t.Fatal("draft not exported", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, before) {
		t.Fatal("export saved original", err)
	}
	current, err := api.docxSnapshot(context.Background())
	if err != nil || current.Revision != base.Revision || current.History.Undo != 0 {
		t.Fatal("export changed source revision", err)
	}
	old, err := session.Snapshot(context.Background())
	if err != nil || old.Blocks[0].Paragraph.Runs[0].Text != "saved" {
		t.Fatal("source contents changed", err)
	}
	payload["baseRevision"] = 2
	if w = request("POST", "/api/docx/export?document="+base.ID, "", payload, true); w.Code != http.StatusConflict {
		t.Fatal("stale export accepted", w.Code)
	}
	if w = request("POST", "/api/docx/export", "", payload, true); w.Code != http.StatusBadRequest {
		t.Fatal("ambiguous source accepted", w.Code)
	}
	if w = request("POST", "/api/docx/export?document="+base.ID, "https://evil.example", payload, true); w.Code != http.StatusForbidden {
		t.Fatal("cross-origin export accepted", w.Code)
	}
}
