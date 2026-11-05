package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type savedFileInput struct {
	SessionID    string `json:"sessionId" jsonschema:"Exact document session ID returned by the open or get tool"`
	BaseRevision uint64 `json:"baseRevision" jsonschema:"Current committed revision"`
}
type savedFileReceipt struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
	Name     string `json:"name"`
}

func (s *Server) savedFile(ctx context.Context, input savedFileInput) (savedFileReceipt, []byte, string, error) {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	var content []byte
	var name, contentType string
	var err error
	if s.session != nil {
		content, err = s.session.SavedFile(ctx, input.SessionID, input.BaseRevision)
		name = filepath.Base(s.session.SourcePath())
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	} else if s.docxSession != nil {
		content, err = s.docxSession.SavedFile(ctx, input.SessionID, input.BaseRevision)
		name = filepath.Base(s.docxSession.SourcePath())
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	} else {
		err = fmt.Errorf("document is closed or unknown")
	}
	return savedFileReceipt{ID: input.SessionID, Revision: input.BaseRevision, Name: name}, content, contentType, err
}
func (s *Server) fileRoutes() {
	s.mux.Handle("POST /api/save", s.requireSession(http.HandlerFunc(s.confirmSave)))
	s.mux.Handle("GET /api/download", s.requireSession(http.HandlerFunc(s.downloadFile)))
}
func (s *Server) confirmSave(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input savedFileInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid save confirmation"))
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("expected one save confirmation"))
		return
	}
	if input.SessionID == "" || input.SessionID != r.URL.Query().Get("document") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("explicit document and session IDs must match"))
		return
	}
	receipt, _, _, err := s.savedFile(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, receipt)
}
func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.ParseUint(r.URL.Query().Get("revision"), 10, 64)
	id := r.URL.Query().Get("session")
	if err != nil || id == "" || id != r.URL.Query().Get("document") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("explicit document, session and revision are required"))
		return
	}
	receipt, content, contentType, err := s.savedFile(r.Context(), savedFileInput{SessionID: id, BaseRevision: revision})
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": receipt.Name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Document-ID", receipt.ID)
	w.Header().Set("X-Document-Revision", strconv.FormatUint(receipt.Revision, 10))
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}
func (s *Server) addFileTools(server *mcp.Server) {
	closedWorld := false
	mcp.AddTool(server, &mcp.Tool{Name: "confirm_workbook_save", Title: "통합 문서 저장 확인", Description: "Verify that the specified XLSX session revision is durably saved and its source file has not changed externally. Does not write or advance the revision. Browser downloads are available in the authenticated browser editor.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input savedFileInput) (*mcp.CallToolResult, savedFileReceipt, error) {
		workbook, release, err := s.acquireWorkbookTarget(input.SessionID, false)
		if err != nil {
			return nil, savedFileReceipt{}, err
		}
		defer release()
		_, err = workbook.SavedFile(ctx, input.SessionID, input.BaseRevision)
		return nil, savedFileReceipt{ID: input.SessionID, Revision: input.BaseRevision, Name: filepath.Base(workbook.SourcePath())}, err
	})
}
