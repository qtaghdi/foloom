package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/foloom/internal/formats/docx"
)

type docxOpenInput struct {
	Path string `json:"path" jsonschema:"Absolute local .docx path"`
}
type docxCreateInput struct {
	Path   string              `json:"path" jsonschema:"New absolute .docx destination. Existing files are never overwritten."`
	Blocks []docx.InitialBlock `json:"blocks" jsonschema:"Ordered paragraph and rectangular table blocks"`
}
type docxOpenResponse struct {
	Document   docx.Snapshot `json:"document"`
	BrowserURL string        `json:"browserUrl,omitempty"`
}

// DOCX has its own concrete session until two adapters demonstrate a useful
// shared contract. XLSX routes and active workbook semantics remain unchanged.
func (s *Server) OpenDOCX(ctx context.Context, path string) (docx.Snapshot, error) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if s.scoped {
		return docx.Snapshot{}, fmt.Errorf("open files through the workspace endpoint")
	}
	if retained := s.findDocument(path); retained != nil && retained.Kind == "docx" {
		s.sessionMu.Lock()
		s.docxSession = retained.server.docxSession
		s.sessionMu.Unlock()
		return retained.server.docxSnapshot(ctx)
	}
	if err := s.checkDocumentLimit(); err != nil {
		return docx.Snapshot{}, err
	}
	next, err := docx.Open(ctx, path)
	if err != nil {
		return docx.Snapshot{}, err
	}
	if _, err := s.registerDocument(next.SourcePath(), nil, next); err != nil {
		return docx.Snapshot{}, err
	}
	s.sessionMu.Lock()
	s.docxSession = next
	s.sessionMu.Unlock()
	return next.Snapshot(ctx)
}
func (s *Server) createDOCX(ctx context.Context, input docxCreateInput) (docx.Snapshot, error) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if s.scoped {
		return docx.Snapshot{}, fmt.Errorf("create files through the workspace endpoint")
	}
	if err := s.checkDocumentLimit(); err != nil {
		return docx.Snapshot{}, err
	}
	next, err := docx.Create(ctx, input.Path, input.Blocks)
	if err != nil {
		return docx.Snapshot{}, err
	}
	if _, err := s.registerDocument(next.SourcePath(), nil, next); err != nil {
		return docx.Snapshot{}, err
	}
	s.sessionMu.Lock()
	s.docxSession = next
	s.sessionMu.Unlock()
	return next.Snapshot(ctx)
}
func (s *Server) acquireDOCX() (*docx.Session, func(), error) {
	s.sessionMu.RLock()
	if s.docxSession == nil {
		s.sessionMu.RUnlock()
		return nil, func() {}, fmt.Errorf("DOCX is not open; call open_docx or create_docx first")
	}
	return s.docxSession, s.sessionMu.RUnlock, nil
}
func (s *Server) docxSnapshot(ctx context.Context) (docx.Snapshot, error) {
	session, release, err := s.acquireDOCX()
	if err != nil {
		return docx.Snapshot{}, err
	}
	defer release()
	return session.Snapshot(ctx)
}
func (s *Server) applyDOCX(ctx context.Context, input docx.ApplyInput, actor string) (docx.Snapshot, error) {
	if !s.scoped && input.SessionID != "" {
		s.sessionMu.RLock()
		document := s.documents[input.SessionID]
		s.sessionMu.RUnlock()
		if document != nil && document.Kind == "docx" {
			return document.server.applyDOCX(ctx, input, actor)
		}
	}
	session, release, err := s.acquireDOCX()
	if err != nil {
		return docx.Snapshot{}, err
	}
	defer release()
	return session.Apply(ctx, input, actor)
}
func (s *Server) docxLaunchURL(id string) (string, error) {
	url, err := s.issueBrowserLaunchURL()
	if url != "" {
		url += "?format=docx"
		url += "&document=" + id
	}
	return url, err
}
func (s *Server) addDOCXTools(server *mcp.Server) {
	closedWorld, destructive, nonDestructive := false, true, false
	mcp.AddTool(server, &mcp.Tool{Name: "open_docx", Title: "DOCX 열기", Description: "Open an absolute local DOCX path. Open browserUrl for direct editing. Read warnings and inspect run IDs before editing.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input docxOpenInput) (*mcp.CallToolResult, docxOpenResponse, error) {
		snapshot, err := s.OpenDOCX(ctx, input.Path)
		if err != nil {
			return nil, docxOpenResponse{}, err
		}
		url, err := s.docxLaunchURL(snapshot.ID)
		return nil, docxOpenResponse{Document: snapshot, BrowserURL: url}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "create_docx", Title: "DOCX 생성", Description: "Create a new local DOCX with paragraph and simple rectangular table blocks. Never overwrites an existing destination. Open browserUrl for editing.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input docxCreateInput) (*mcp.CallToolResult, docxOpenResponse, error) {
		snapshot, err := s.createDOCX(ctx, input)
		if err != nil {
			return nil, docxOpenResponse{}, err
		}
		url, err := s.docxLaunchURL(snapshot.ID)
		return nil, docxOpenResponse{Document: snapshot, BrowserURL: url}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_docx", Title: "DOCX 확인", Description: "Inspect the active DOCX revision, session ID, ordered body paragraphs, tables, editable run IDs and compatibility warnings.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, docx.Snapshot, error) {
		snapshot, err := s.docxSnapshot(ctx)
		return nil, snapshot, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "apply_docx_operations", Title: "DOCX 텍스트 편집", Description: "Atomically replace plain body or simple table run text while preserving unrelated OOXML. Pass current sessionId, baseRevision, editable targetId and exact expectedText. Refresh after conflicts. Tabs and line breaks, merged/nested tables, fields and tracked changes are unsupported.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input docx.ApplyInput) (*mcp.CallToolResult, docx.Snapshot, error) {
		snapshot, err := s.applyDOCX(ctx, input, "ai")
		return nil, snapshot, err
	})
}
func (s *Server) docxRoutes() {
	s.mux.HandleFunc("GET /docx", s.docxIndex)
	s.mux.Handle("GET /api/docx", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := s.docxSnapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})))
	s.mux.Handle("POST /api/docx/operations", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input docx.ApplyInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid DOCX request: %w", err))
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("DOCX request must contain one JSON value"))
			return
		}
		snapshot, err := s.applyDOCX(r.Context(), input, "human")
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, docx.ErrRevisionConflict) || errors.Is(err, docx.ErrExternalChange) || errors.Is(err, docx.ErrWriterBusy) {
				status = http.StatusConflict
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})))
	s.mux.Handle("GET /api/docx/events", s.requireSession(http.HandlerFunc(s.docxEvents)))
}
func (s *Server) docxIndex(w http.ResponseWriter, r *http.Request) {
	if token := r.URL.Query().Get("token"); token != "" {
		if !tokenMatches(token, s.browserToken) {
			http.Error(w, "invalid session token", http.StatusUnauthorized)
			return
		}
		s.setBrowserSessionCookie(w, r)
		http.Redirect(w, r, "/docx", http.StatusSeeOther)
		return
	}
	if !s.validSessionCookie(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := staticFS.ReadFile("static/docx.html")
	if err != nil {
		http.Error(w, "DOCX UI unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}
func (s *Server) docxEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	after := uint64(0)
	value := r.Header.Get("Last-Event-ID")
	if value == "" {
		value = r.URL.Query().Get("after")
	}
	if value != "" {
		var err error
		after, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	_, release, err := s.acquireDOCX()
	if err == nil {
		release()
	}
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		session, release, err := s.acquireDOCX()
		if err != nil {
			return
		}
		if session.ID() != r.URL.Query().Get("session") {
			events, _ := session.EventsAfter(r.Context(), ^uint64(0))
			release()
			for _, event := range events {
				data, _ := json.Marshal(event)
				_, _ = fmt.Fprintf(w, "event: docx\ndata: %s\n\n", data)
			}
			flusher.Flush()
			return
		}
		events, err := session.EventsAfter(r.Context(), after)
		release()
		if err != nil {
			return
		}
		for _, event := range events {
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: docx\ndata: %s\n\n", event.Sequence, data)
			after = event.Sequence
		}
		_, _ = io.WriteString(w, ": keepalive\n\n")
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
