package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qtaghdi/document-workspace/internal/formats/docx"
	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
)

const maxOpenDocuments = 8

type openedDocument struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Uploaded   bool   `json:"uploaded,omitempty"`
	Origin     string `json:"origin,omitempty"`
	cleanupDir string
	server     *Server
}

func canonicalDocumentPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("document path must be absolute")
	}
	return filepath.EvalSymlinks(filepath.Clean(path))
}

// openMu serializes registration, duplicate detection, close and legacy opens.
func (s *Server) findDocument(path string) *openedDocument {
	canonical, err := canonicalDocumentPath(path)
	if err != nil {
		return nil
	}
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	for _, document := range s.documents {
		if document.Path == canonical {
			return document
		}
	}
	return nil
}
func (s *Server) checkDocumentLimit() error {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	if s.closed {
		return fmt.Errorf("workspace is closed")
	}
	if len(s.documents) >= maxOpenDocuments {
		return fmt.Errorf("열린 파일은 최대 %d개입니다. 파일 하나를 닫은 뒤 다시 여세요", maxOpenDocuments)
	}
	return nil
}
func (s *Server) retainedWorkbook(path string) (*xlsx.Session, error) {
	if retained := s.findDocument(path); retained != nil && retained.Kind == "xlsx" {
		return retained.server.session, nil
	}
	if err := s.checkDocumentLimit(); err != nil {
		return nil, err
	}
	return xlsx.Open(path)
}
func (s *Server) registerDocument(path string, workbook *xlsx.Session, document *docx.Session, cleanupDirs ...string) (*openedDocument, error) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("workspace is closed")
	}
	for _, opened := range s.documents {
		if opened.Path == path {
			return opened, nil
		}
	}
	if len(s.documents) >= maxOpenDocuments {
		return nil, fmt.Errorf("open document limit reached")
	}
	child := &Server{session: workbook, docxSession: document, scoped: true, browserToken: s.browserToken, mcpToken: s.mcpToken, mux: http.NewServeMux()}
	child.routes()
	opened := &openedDocument{Path: path, server: child, Origin: "path"}
	if workbook != nil {
		snapshot := workbook.Snapshot()
		opened.ID = snapshot.ID
		opened.Name = snapshot.Name
		opened.Kind = "xlsx"
	} else {
		opened.ID = document.ID()
		opened.Name = filepath.Base(document.SourcePath())
		opened.Kind = "docx"
	}
	if len(cleanupDirs) > 0 {
		opened.Uploaded, opened.cleanupDir, opened.Path, opened.Origin = true, cleanupDirs[0], "", "upload"
		if len(cleanupDirs) > 1 {
			opened.Origin = cleanupDirs[1]
		}
	}
	s.documents[opened.ID] = opened
	return opened, nil
}

// A scoped request never falls back to the active document. Closed and unknown
// IDs fail, including reads, writes, MCP calls and SSE reconnects.
func (s *Server) serveDocument(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("document")
	if id == "" || r.URL.Path == "/" || r.URL.Path == "/docx" || strings.HasPrefix(r.URL.Path, "/launch/") || strings.HasPrefix(r.URL.Path, "/assets/") {
		s.mux.ServeHTTP(w, r)
		return
	}
	if !s.validSessionCookie(r) && !(r.URL.Path == "/mcp" && tokenMatches(r.Header.Get("Authorization"), "Bearer "+s.mcpToken)) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.sessionMu.RLock()
	document := s.documents[id]
	s.sessionMu.RUnlock()
	if document == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("document is closed or unknown"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/documents") {
		http.NotFound(w, r)
		return
	}
	document.server.mux.ServeHTTP(w, r)
}

func (s *Server) workspaceRoutes() {
	s.mux.Handle("GET /api/documents", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.sessionMu.RLock()
		documents := make([]*openedDocument, 0, len(s.documents))
		for _, document := range s.documents {
			documents = append(documents, document)
		}
		s.sessionMu.RUnlock()
		sort.Slice(documents, func(i, j int) bool { return documents[i].Path < documents[j].Path })
		writeJSON(w, http.StatusOK, struct {
			Documents []*openedDocument `json:"documents"`
			MaxOpen   int               `json:"maxOpen"`
		}{documents, maxOpenDocuments})
	})))
	s.mux.Handle("POST /api/documents/new-docx", s.requireSession(http.HandlerFunc(s.newDOCX)))
	s.mux.Handle("POST /api/documents/copy-docx", s.requireSession(http.HandlerFunc(s.copyDOCX)))
	s.mux.Handle("POST /api/documents/upload", s.requireSession(http.HandlerFunc(s.uploadDocument)))
	s.mux.Handle("POST /api/documents", s.requireSession(http.HandlerFunc(s.openDocument)))
	s.mux.Handle("DELETE /api/documents/{id}", s.requireSession(http.HandlerFunc(s.closeDocument)))
}
func (s *Server) openDocument(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input struct {
		Path string `json:"path"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid document request"))
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, fmt.Errorf("expected one document request"))
		return
	}
	path, err := canonicalDocumentPath(input.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("파일의 절대 경로와 접근 권한을 확인하세요"))
		return
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if opened := s.findDocument(path); opened != nil {
		writeJSON(w, http.StatusOK, opened)
		return
	}
	if err := s.checkDocumentLimit(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	opened, err := s.openDocumentPath(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, opened)
}

// Callers hold openMu through validation, opening and registration.
func (s *Server) openDocumentPath(ctx context.Context, path string, cleanupDirs ...string) (*openedDocument, error) {
	var err error
	var opened *openedDocument
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx":
		workbook, openErr := xlsx.Open(path)
		err = openErr
		if err == nil {
			opened, err = s.registerDocument(path, workbook, nil, cleanupDirs...)
			if err != nil {
				_ = workbook.Close()
			}
		}
	case ".docx":
		document, openErr := docx.Open(ctx, path)
		err = openErr
		if err == nil {
			opened, err = s.registerDocument(path, nil, document, cleanupDirs...)
		}
	default:
		err = fmt.Errorf("XLSX 또는 DOCX 파일을 선택하세요")
	}
	if err != nil {
		return nil, err
	}
	return opened, nil
}
func (s *Server) closeDocument(w http.ResponseWriter, r *http.Request) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	s.sessionMu.Lock()
	document := s.documents[r.PathValue("id")]
	if document != nil {
		delete(s.documents, document.ID)
		if s.session == document.server.session {
			s.session = nil
		}
		if s.docxSession == document.server.docxSession {
			s.docxSession = nil
		}
	}
	s.sessionMu.Unlock()
	if document == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("document is closed or unknown"))
		return
	}
	_ = document.server.Close()
	if document.cleanupDir != "" {
		_ = os.RemoveAll(document.cleanupDir)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Target choice and acquisition share one lock so a concurrent open cannot
// replace an implicitly selected workbook between validation and mutation.
func (s *Server) acquireWorkbookTarget(id string, writing bool) (*xlsx.Session, func(), error) {
	s.sessionMu.RLock()
	if id != "" && !s.scoped {
		document := s.documents[id]
		if document == nil || document.Kind != "xlsx" {
			s.sessionMu.RUnlock()
			return nil, func() {}, fmt.Errorf("unknown XLSX documentId")
		}
		session, release, err := document.server.acquireSession()
		s.sessionMu.RUnlock()
		return session, release, err
	}
	if writing && !s.scoped {
		count := 0
		for _, document := range s.documents {
			if document.Kind == "xlsx" {
				count++
			}
		}
		if count > 1 {
			s.sessionMu.RUnlock()
			return nil, func() {}, fmt.Errorf("multiple workbooks are open; pass documentId from get_workbook")
		}
	}
	if s.session == nil {
		s.sessionMu.RUnlock()
		return nil, func() {}, errWorkbookNotOpen
	}
	if id != "" && s.session.Snapshot().ID != id {
		s.sessionMu.RUnlock()
		return nil, func() {}, fmt.Errorf("documentId does not match this editor")
	}
	return s.session, s.sessionMu.RUnlock, nil
}
