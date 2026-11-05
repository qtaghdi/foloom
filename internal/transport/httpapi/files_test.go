package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDocumentSaveConfirmationAndDownload(t *testing.T) {
	for _, format := range []string{"xlsx", "docx"} {
		t.Run(format, func(t *testing.T) {
			api, sourcePath := newSecurityServer(t)
			defer api.Close()
			if format == "docx" {
				var err error
				sourcePath, err = filepath.Abs("../../../testdata/docx/compatibility/generated-basic.docx")
				if err != nil {
					t.Fatal(err)
				}
			}
			original, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "한글 \"보고서\"."+format)
			if err = os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			document := openWorkspaceFile(t, api, path)
			saveBody := fmt.Sprintf(`{"sessionId":%q,"baseRevision":1}`, document.ID)
			saved := workspaceRequest(api, "POST", "/api/save?document="+document.ID, saveBody)
			var receipt savedFileReceipt
			if err = json.Unmarshal(saved.Body.Bytes(), &receipt); err != nil || saved.Code != 200 || receipt.ID != document.ID || receipt.Revision != 1 {
				t.Fatalf("save receipt: %d %v %s", saved.Code, err, saved.Body)
			}
			route := fmt.Sprintf("/api/download?document=%s&session=%s&revision=1", document.ID, document.ID)
			response := workspaceRequest(api, "GET", route, "")
			if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), original) {
				t.Fatalf("download: %d %s", response.Code, response.Body)
			}
			disposition, params, err := mime.ParseMediaType(response.Header().Get("Content-Disposition"))
			if err != nil || disposition != "attachment" || params["filename"] != filepath.Base(path) {
				t.Fatalf("filename: %v %q", err, response.Header().Get("Content-Disposition"))
			}
			expected := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			if format == "docx" {
				expected = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			}
			if response.Header().Get("Content-Type") != expected || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Document-ID") != document.ID || response.Header().Get("X-Document-Revision") != "1" {
				t.Fatal("download headers incorrect")
			}
			if head := workspaceRequest(api, "HEAD", route, ""); head.Code != 200 || head.Body.Len() != 0 {
				t.Fatal("HEAD returned bytes")
			}
			for _, path := range []string{route, "/api/save?document=" + document.ID} {
				recorder := httptest.NewRecorder()
				method := "GET"
				if path[5:9] == "save" {
					method = "POST"
				}
				api.Handler().ServeHTTP(recorder, httptest.NewRequest(method, path, bytes.NewBufferString(saveBody)))
				if recorder.Code != 401 {
					t.Fatalf("unauthenticated download/save: %d", recorder.Code)
				}
			}
			if response = workspaceRequest(api, "GET", route+"0", ""); response.Code != 409 {
				t.Fatalf("stale revision: %d", response.Code)
			}
			if response = workspaceRequest(api, "GET", "/api/download?document="+document.ID+"&session=other&revision=1", ""); response.Code != 400 {
				t.Fatal("mismatched session accepted")
			}
			os.WriteFile(path, []byte("external replacement"), 0600)
			if response = workspaceRequest(api, "GET", route, ""); response.Code != 409 {
				t.Fatal("external replacement was downloaded")
			}
			if response = workspaceRequest(api, "POST", "/api/save?document="+document.ID, saveBody); response.Code != 409 {
				t.Fatal("external replacement reported saved")
			}
			workspaceRequest(api, "DELETE", "/api/documents/"+document.ID, "")
			if response = workspaceRequest(api, "GET", route, ""); response.Code != 404 {
				t.Fatal("closed document fell back")
			}
		})
	}
}
func TestMCPConfirmsSavedWorkbookRevision(t *testing.T) {
	api, _ := newSecurityServer(t)
	defer api.Close()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := api.mcpServer().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "save-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	snapshot := api.session.Snapshot()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "confirm_workbook_save", Arguments: savedFileInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision}})
	if err != nil || result.IsError {
		t.Fatalf("save confirmation: %+v %v", result, err)
	}
	if api.session.Snapshot().Revision != snapshot.Revision {
		t.Fatal("confirmation advanced revision")
	}
	wrong, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "confirm_workbook_save", Arguments: savedFileInput{SessionID: snapshot.ID, BaseRevision: snapshot.Revision + 1}})
	if err != nil || !wrong.IsError {
		t.Fatal("MCP confirmed wrong revision")
	}
}
