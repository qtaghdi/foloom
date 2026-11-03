package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
)

const (
	appResourceURI     = "ui://document-workspace/xlsx/v1.html"
	serverInstructions = "For DOCX use create_docx, open_docx, get_docx and apply_docx_operations. Open the returned browserUrl for direct editing. DOCX supports plain body and simple table text runs; inspect warnings and use current sessionId, revision, target IDs and expectedText. For XLSX, if no workbook is open, call open_workbook with an absolute XLSX path. Always open the returned short-lived browserUrl in the host browser surface so the user can inspect and directly edit the workbook; do not substitute an inline image or MCP App preview, and do not repeat the URL in chat. Call get_workbook before reads or writes and use read_range only for focused inspection. Pass the latest revision to apply_operations, and refresh after a conflict. Use update_presence before visible AI edits."
)

type openWorkbookInput struct {
	Path string `json:"path,omitempty" jsonschema:"Absolute path to the local XLSX workbook. Omit only when a workbook is already open."`
}

type openWorkbookResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Sheets     []string `json:"sheets"`
	Revision   uint64   `json:"revision"`
	BrowserURL string   `json:"browserUrl,omitempty"`
}

type readRangeInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
	Range string `json:"range" jsonschema:"A1-style range, for example A1:H40"`
}

type sheetInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
}

type eventPollInput struct {
	AfterSequence uint64 `json:"afterSequence,omitempty" jsonschema:"Last processed event sequence, or zero for all retained events"`
}

type eventPollResponse struct {
	Events   []xlsx.Event  `json:"events"`
	Workbook xlsx.Snapshot `json:"workbook"`
}

func (s *Server) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcpServer() }, nil)
}

// RunStdio serves the same MCP tools over stdin and stdout for local desktop
// hosts. Nothing except protocol messages may be written to stdout while it runs.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer().Run(ctx, &mcp.StdioTransport{})
}

// RunStdioWithBrowser keeps stdout protocol-only while serving the authenticated
// browser fallback on a loopback listener owned by the same process.
func (s *Server) RunStdioWithBrowser(ctx context.Context, addr string) error {
	if err := requireLoopbackAddress(addr); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen for browser fallback: %w", err)
	}
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	s.launchMu.Lock()
	s.browserBaseURL = "http://" + listener.Addr().String()
	s.launchMu.Unlock()

	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpErrors := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		httpErrors <- err
	}()
	stdioErrors := make(chan error, 1)
	go func() {
		stdioErrors <- s.RunStdio(runContext)
	}()

	var runErr error
	select {
	case runErr = <-stdioErrors:
	case err := <-httpErrors:
		if err != nil {
			runErr = fmt.Errorf("serve browser fallback: %w", err)
		}
		cancel()
	case <-ctx.Done():
		runErr = ctx.Err()
	}
	cancel()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil && runErr == nil {
		runErr = fmt.Errorf("stop browser fallback: %w", err)
	}
	return runErr
}

func requireLoopbackAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse browser fallback address: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("browser fallback must bind to a loopback address, got %q", host)
	}
	return nil
}

func (s *Server) mcpServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "document-workspace", Version: "0.1.0"},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	appOnlyMeta := mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
	closedWorld := false
	destructive := true
	nonDestructive := false
	server.AddPrompt(&mcp.Prompt{
		Name:        "document-xlsx",
		Title:       "XLSX 문서 작업",
		Description: "지정한 XLSX를 필요한 범위만 읽고 검토 가능한 작업 묶음으로 안전하게 수정합니다.",
		Arguments: []*mcp.PromptArgument{
			{Name: "path", Title: "XLSX 절대 경로", Required: true},
			{Name: "task", Title: "요청할 작업", Required: true},
		},
	}, func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		path := strings.TrimSpace(request.Params.Arguments["path"])
		task := strings.TrimSpace(request.Params.Arguments["task"])
		if path == "" || task == "" {
			return nil, fmt.Errorf("path and task are required")
		}
		text := fmt.Sprintf("Use the document-workspace MCP tools for this request. Open %q, then open the returned browserUrl in the host browser surface so the user can directly edit the workbook. Do not use an inline image or MCP App preview as the editing surface, and do not repeat the short-lived URL in chat. Inspect only the workbook metadata and ranges needed for the task, summarize the proposed changes, and use the latest revision for one coherent apply_operations batch after the host presents any required approval. Do not read the entire workbook unless the task requires it. Re-read the changed range and report the saved revision when complete. Task: %s", path, task)
		return &mcp.GetPromptResult{
			Description: "안전한 XLSX 편집 workflow",
			Messages:    []*mcp.PromptMessage{{Role: mcp.Role("user"), Content: &mcp.TextContent{Text: text}}},
		}, nil
	})
	server.AddResource(&mcp.Resource{
		URI:         appResourceURI,
		Name:        "document-workspace-xlsx",
		Title:       "Workbook Editor",
		Description: "Interactive spreadsheet editor for the open XLSX workbook.",
		MIMEType:    "text/html;profile=mcp-app",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		body, err := staticFS.ReadFile("static/app.html")
		if err != nil {
			return nil, fmt.Errorf("read embedded MCP App: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      appResourceURI,
			MIMEType: "text/html;profile=mcp-app",
			Text:     string(body),
			Meta:     mcp.Meta{"ui": map[string]any{"prefersBorder": false}},
		}}}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_workbook",
		Title:       "Open Workbook in Browser",
		Description: "Open an absolute local XLSX path as the active workbook, or reopen the current workbook when path is omitted. Always open the returned short-lived browserUrl in the host browser surface for direct user editing; do not use an inline image or MCP App preview as the editing surface.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input openWorkbookInput) (*mcp.CallToolResult, openWorkbookResponse, error) {
		if path := strings.TrimSpace(input.Path); path != "" {
			s.openMu.Lock()
			defer s.openMu.Unlock()
			if !filepath.IsAbs(path) {
				return nil, openWorkbookResponse{}, fmt.Errorf("workbook path must be absolute")
			}
			if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
				return nil, openWorkbookResponse{}, fmt.Errorf("workbook path must use the .xlsx extension")
			}
			next, err := xlsx.Open(path)
			if err != nil {
				return nil, openWorkbookResponse{}, err
			}
			if err := s.replaceSession(next); err != nil {
				return nil, openWorkbookResponse{}, fmt.Errorf("close previous workbook: %w", err)
			}
		}
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, openWorkbookResponse{}, err
		}
		defer release()
		snapshot := session.Snapshot()
		browserURL, err := s.issueBrowserLaunchURL()
		return nil, openWorkbookResponse{
			ID:         snapshot.ID,
			Name:       snapshot.Name,
			Sheets:     snapshot.Sheets,
			Revision:   snapshot.Revision,
			BrowserURL: browserURL,
		}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "restore_history",
		Title:       "Undo or Redo Workbook Change",
		Description: "Undo or redo one committed workbook revision. Pass the latest baseRevision and direction.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input historyInput) (*mcp.CallToolResult, applyResponse, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, applyResponse{}, err
		}
		defer release()
		snapshot, err := session.RestoreHistory(input.BaseRevision, "ai", input.Direction)
		return nil, applyResponse{Workbook: snapshot, Applied: 1}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "restore_user_history",
		Title:       "Undo or Redo User Workbook Change",
		Description: "Undo or redo one committed workbook revision for the MCP App user.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input historyInput) (*mcp.CallToolResult, applyResponse, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, applyResponse{}, err
		}
		defer release()
		snapshot, err := session.RestoreHistory(input.BaseRevision, "human", input.Direction)
		return nil, applyResponse{Workbook: snapshot, Applied: 1}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workbook",
		Title:       "Get Workbook",
		Description: "Get the open workbook name, sheets, revision, and formula calculation policy before reading or editing it.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, xlsx.Snapshot, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, xlsx.Snapshot{}, err
		}
		defer release()
		return nil, session.Snapshot(), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_range",
		Title:       "Read Range",
		Description: "Read cell values and formulas from an A1-style range. Formula values are cached source values or unavailable; the server does not calculate formulas.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input readRangeInput) (*mcp.CallToolResult, xlsx.Range, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, xlsx.Range{}, err
		}
		defer release()
		returnValue, err := session.ReadRange(input.Sheet, input.Range)
		return nil, returnValue, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_events",
		Title:       "Get Workbook Events",
		Description: "Read retained workbook and presence events after a sequence number for MCP App synchronization.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input eventPollInput) (*mcp.CallToolResult, eventPollResponse, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, eventPollResponse{}, err
		}
		defer release()
		events, snapshot := session.EventsAfter(input.AfterSequence)
		return nil, eventPollResponse{Events: events, Workbook: snapshot}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sheet_objects",
		Title:       "Get Sheet Objects",
		Description: "Read bounded image data and chart previews for MCP App rendering.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sheetInput) (*mcp.CallToolResult, xlsx.SheetObjects, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, xlsx.SheetObjects{}, err
		}
		defer release()
		objects, err := session.ReadSheetObjects(input.Sheet)
		return nil, objects, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_operations",
		Title:       "Apply Workbook Operations",
		Description: "Apply validated cell, formula, or rectangular paste edits to the open xlsx. Formula expressions are stored and native recalculation is requested; browser results are previews. Pass the latest baseRevision to prevent overwriting concurrent edits.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input applyInput) (*mcp.CallToolResult, applyResponse, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, applyResponse{}, err
		}
		defer release()
		snapshot, err := session.Apply(input.BaseRevision, "ai", input.Operations)
		return nil, applyResponse{Workbook: snapshot, Applied: len(input.Operations)}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_user_operations",
		Title:       "Apply User Workbook Operations",
		Description: "Apply browser-originated workbook operations from the MCP App.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input applyInput) (*mcp.CallToolResult, applyResponse, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, applyResponse{}, err
		}
		defer release()
		snapshot, err := session.Apply(input.BaseRevision, "human", input.Operations)
		return nil, applyResponse{Workbook: snapshot, Applied: len(input.Operations)}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_presence",
		Title:       "Update AI Presence",
		Description: "Move the AI cursor or selection without changing workbook data or its revision.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input xlsx.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, map[string]bool{"updated": false}, err
		}
		defer release()
		err = session.UpdatePresence("ai", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_user_presence",
		Title:       "Update User Presence",
		Description: "Publish the MCP App user's current selection without changing workbook data.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input xlsx.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		session, release, err := s.acquireSession()
		if err != nil {
			return nil, map[string]bool{"updated": false}, err
		}
		defer release()
		err = session.UpdatePresence("human", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	s.addDOCXTools(server)
	return server
}
