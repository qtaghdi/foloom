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
func (s *Server) restoreDOCX(ctx context.Context, input docx.HistoryInput, actor string) (docx.Snapshot, error) {
	if !s.scoped && input.SessionID != "" {
		s.sessionMu.RLock()
		document := s.documents[input.SessionID]
		s.sessionMu.RUnlock()
		if document != nil && document.Kind == "docx" {
			return document.server.restoreDOCX(ctx, input, actor)
		}
	}
	session, release, err := s.acquireDOCX()
	if err != nil {
		return docx.Snapshot{}, err
	}
	defer release()
	return session.RestoreHistory(ctx, input, actor)
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
	mcp.AddTool(server, &mcp.Tool{Name: "apply_docx_operations", Title: "DOCX 편집", Description: "Atomically edit a DOCX using the operation schema and the current sessionId, baseRevision, and applicable IDs from get_docx. " +
		"Use replace_run_text or replace_cell_text with exact expectedText for text. LF inserts an inline break. " +
		"Use replace_paragraphs for a contiguous editable sibling range with source IDs and exact paragraph texts joined by LF. " +
		"Paragraph plans support styles, direction, spacing, tab stops, and list references. Define styles with define_style. " +
		"Use define_list to create a nine-level list with kind bullet, decimal, upperRoman, lowerRoman, upperLetter, or lowerLetter. Set compound to true on a fresh numbered list to include upper-level counters. Restart a supported list level with sourceId, level, and start. Use restart_compound_sublevel with targetId, endTargetId, and listStart for contiguous terminal compound children marked compoundRestart. " +
		"Use list {key, level} to apply a new definition in the same batch, {id, level} to continue a list from snapshot.lists, or {id:0, level:0} to remove numbering. " +
		"Use update_section for paper settings and supported header or footer text, PAGE, and NUMPAGES fields, including compactPageNumbers to combine the two fields, or update_boundary for supported page and section breaks, including section_even_page and section_odd_page. " +
		"Use update_table, update_image, and update_math for supported tables, inline pictures, and equations. Use update_table with table {action: insert_caption, caption} and a table ID with tableCaptionInsertable to add one numbered caption. " +
		"Use update_math with math {action: insert_matrix, matrix: [[a, b], [c, d]]} and an editable paragraph ID, or math {action: edit_matrix_cell, matrixCell: {row, column, expectedText, text}} and a supported matrix run ID. Rectangular matrices support two through four rows and columns. Matrix coordinates start at 1. " +
		"Use update_reference for supported links, bookmarks, footnotes, endnotes, and date fields. " +
		"Use insert_line with an editable body paragraph ID and lineSize {widthEmu, heightEmu} to append an inline diagonal line. Use resize_line with a supported line run ID and lineSize to update both dimensions. Use delete_line with a supported line run ID marked deleteEditable. Use insert_rectangle for a basic inline shape, update_rectangle_colors to set its fill and outline, resize_rectangle to set both dimensions, and delete_rectangle to remove a supported rectangle. Use insert_textbox for an inline text box. Use insert_control_text to append a plain text control to an editable body paragraph. Use insert_control_date with an editable body paragraph ID and text for a valid YYYY-MM-DD date. Use update_control_date alone with a run ID marked dateControl, expectedText for the current date, and text for the new YYYY-MM-DD date. Use update_control_choice alone with a run ID marked choiceControl, expectedText for its current display value and text for a different listed option. Use unwrap_control alone with a control run ID marked unwrapEditable to remove a plain text control while keeping its runs. Use update_control_text or update_textbox_text to edit supported object text, and update_chart_title or resize_chart for supported local charts. Use insert_chart with an editable body paragraph ID and chartInsert {kind, title, points: [{label, value}]} for a self-contained pie, bar, or line chart. Omit kind for a pie chart. Use update_chart_value with chartValue {point, expectedValue, value} for a chart with literal chartValues. Use delete_chart with a chart run ID to remove one supported inline placement while retaining its package parts. Use move_chart_up or move_chart_down alone with a chart run ID and the corresponding moveChartUp or moveChartDown capability to move a dedicated chart paragraph across an eligible neighboring paragraph. " +
		"Use add_comment to annotate text, or resolve_comment and reopen_comment with an eligible imported comment ID. " +
		"Use track_run_text to record a simple text replacement or deletion, or track_paragraph_alignment with a body paragraph ID and alignment (start, center, end, justify). Use track_paragraph_break with the active body run, exact current text, and text before the split. Use track_paragraph_merge with the first paragraph ID to record deletion of the boundary to the next paragraph. Use accept_revision or reject_revision on supported tracked changes. " +
		"Keep edits within the editable targets returned by get_docx. Do not overlap operations. Refresh the document after a conflict.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input docx.ApplyInput) (*mcp.CallToolResult, docx.Snapshot, error) {
		snapshot, err := s.applyDOCX(ctx, input, "ai")
		return nil, snapshot, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "restore_docx_history", Title: "DOCX 실행 취소·다시 실행", Description: "Restore one complete accepted DOCX batch from the shared chronological human/AI history. Inspect history, sessionId and current revision first. direction is undo or redo. New writes discard redo. Refresh after conflicts.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld}}, func(ctx context.Context, _ *mcp.CallToolRequest, input docx.HistoryInput) (*mcp.CallToolResult, docx.Snapshot, error) {
		snapshot, err := s.restoreDOCX(ctx, input, "ai")
		return nil, snapshot, err
	})
}
func (s *Server) docxRoutes() {
	s.mux.Handle("POST /api/docx/export", s.requireSession(http.HandlerFunc(s.exportDOCXDraft)))
	s.mux.HandleFunc("GET /docx", s.docxIndex)
	s.mux.Handle("GET /api/docx", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := s.docxSnapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})))
	s.mux.Handle("GET /api/docx/picture", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, release, err := s.acquireDOCX()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer release()
		raw, mime, err := session.Picture(r.Context(), r.URL.Query().Get("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(raw)
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
	s.mux.Handle("POST /api/docx/history", s.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input docx.HistoryInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid DOCX history request: %w", err))
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(w, http.StatusBadRequest, fmt.Errorf("DOCX history request must contain one JSON value"))
			return
		}
		snapshot, err := s.restoreDOCX(r.Context(), input, "human")
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
