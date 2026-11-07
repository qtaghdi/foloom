package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
)

func TestDOCXRecoveryPersistsAcrossWorkspaceRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recovery")
	newAPI := func() *Server {
		api := NewWithTokens(nil, "browser-token", "mcp-token")
		api.recoveryDir = dir
		t.Cleanup(func() { _ = api.Close() })
		return api
	}
	api := newAPI()
	request := documentWorkspaceRequest(t, api)
	list := request("GET", recoveryRoute, "", nil, true)
	if list.Code != 200 || list.Body.String() != "[]\n" {
		t.Fatal(list.Code, list.Body.String())
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("store created before consent", err)
	}
	source := request("POST", "/api/documents/new-docx", "", map[string]string{"name": "보관"}, true)
	var opened openedDocument
	if source.Code != http.StatusCreated || json.Unmarshal(source.Body.Bytes(), &opened) != nil {
		t.Fatal(source.Code, source.Body.String())
	}
	download := func(id string) string { return "/api/download?document=" + id + "&session=" + id + "&revision=1" }
	expected := request("GET", download(opened.ID), "", nil, true).Body.Bytes()
	input := savedFileInput{SessionID: opened.ID, BaseRevision: 1}
	if w := request("POST", recoveryRoute, "https://evil.example", input, true); w.Code != 403 {
		t.Fatal("cross-origin recovery consent", w.Code)
	}
	if w := request("POST", recoveryRoute, "", input, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := api.Close(); err != nil {
		t.Fatal(err)
	}

	request = documentWorkspaceRequest(t, newAPI())
	if w := request("GET", recoveryRoute, "", nil, false); w.Code != 401 {
		t.Fatal("unauthenticated recovery list", w.Code)
	}
	list = request("GET", recoveryRoute, "", nil, true)
	var entries []recoveryEntry
	if list.Code != 200 || json.Unmarshal(list.Body.Bytes(), &entries) != nil || len(entries) != 1 || entries[0].ID != opened.ID {
		t.Fatal(list.Code, list.Body.String())
	}
	openPath := recoveryRoute + "/" + opened.ID + "/open"
	reopened := request("POST", openPath, "", nil, true)
	var recovered openedDocument
	if reopened.Code != 201 || json.Unmarshal(reopened.Body.Bytes(), &recovered) != nil || recovered.ID == opened.ID || recovered.Origin != "recovery" || recovered.Path != "" {
		t.Fatal(reopened.Code, reopened.Body.String())
	}
	if w := request("GET", download(recovered.ID), "", nil, true); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), expected) {
		t.Fatal("reopened bytes differ from checkpoint", w.Code)
	}
	if w := request("DELETE", recoveryRoute+"/"+opened.ID, "", nil, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", openPath, "", nil, true); w.Code != 404 {
		t.Fatal("deleted recovery still accessible", w.Code)
	}
}

func TestDOCXRecoveryExpiresAndRejectsDamagedContent(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	entry := recoveryEntry{ID: "00000000000000000000000000000000", Name: "test.docx", UpdatedAt: now}
	if err := writeRecovery(root, entry, []byte("checkpoint")); err != nil {
		t.Fatal(err)
	}
	path := recoveryPath(root, entry.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRecovery(root, entry.ID, true); err == nil {
		t.Fatal("damaged content accepted")
	}
	if _, count, err := scanRecovery(root, now.Add(recoveryLifetime)); err != nil || count != 0 {
		t.Fatal("expired record retained", count, err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("expired record still on disk", err)
	}
}

func TestDOCXRecoveryRetainsDraftAfterSourceReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.docx")
	if _, err := docx.Create(context.Background(), path, []docx.InitialBlock{{Type: "paragraph", Text: "원본"}}); err != nil {
		t.Fatal(err)
	}
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	api.recoveryDir = filepath.Join(t.TempDir(), "recovery")
	request := documentWorkspaceRequest(t, api)
	open := request("POST", "/api/documents", "", map[string]string{"path": path}, true)
	var source openedDocument
	if open.Code != 200 || json.Unmarshal(open.Body.Bytes(), &source) != nil {
		t.Fatal(open.Code, open.Body.String())
	}
	var base docx.Snapshot
	w := request("GET", "/api/docx?document="+source.ID, "", nil, true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &base) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	input := docx.ApplyInput{SessionID: base.ID, BaseRevision: base.Revision, Operations: []docx.Operation{{
		Type: "replace_run_text", TargetID: base.Blocks[0].Paragraph.Runs[0].ID, ExpectedText: "원본", Text: "복구할 초안",
	}}}
	if w = request("PUT", recoveryRoute, "", input, true); w.Code != 404 {
		t.Fatal("updated without consent", w.Code)
	}
	if w = request("POST", recoveryRoute, "", savedFileInput{SessionID: base.ID, BaseRevision: base.Revision}, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	external := []byte("external replacement")
	if err := os.WriteFile(path, external, 0600); err != nil {
		t.Fatal(err)
	}
	if w = request("PUT", recoveryRoute, "", input, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/docx/operations?document="+source.ID, "", input, true); w.Code != 409 {
		t.Fatal("source write did not detect replacement", w.Code)
	}
	input.Operations[0].ExpectedText = "stale text"
	if w = request("PUT", recoveryRoute, "", input, true); w.Code != 400 {
		t.Fatal("invalid update replaced checkpoint", w.Code)
	}
	w = request("POST", recoveryRoute+"/"+source.ID+"/open", "", nil, true)
	var opened openedDocument
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &opened) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/docx?document="+opened.ID, "", nil, true)
	var recovered docx.Snapshot
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &recovered) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if recovered.ID == base.ID || recovered.Revision != 1 || recovered.Blocks[0].Paragraph.Runs[0].Text != "복구할 초안" {
		t.Fatal("draft was not reopened in its own session")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(external, actual) {
		t.Fatal("recovery wrote to original", err)
	}
	if w = request("DELETE", recoveryRoute+"/"+source.ID, "", nil, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("PUT", recoveryRoute, "", input, true); w.Code != 404 {
		t.Fatal("background update restored deleted consent", w.Code)
	}
}
