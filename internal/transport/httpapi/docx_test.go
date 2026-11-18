package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/foloom/internal/formats/docx"
)

func docxClient(t *testing.T, api *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, e := api.mcpServer().Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { ss.Close() })
	cs, e := mcp.NewClient(&mcp.Implementation{Name: "docx-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}
func TestDOCXMCPAndBrowserShareCommits(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	api.browserBaseURL = "http://127.0.0.1:9876"
	client := docxClient(t, api)
	call := func(name string, input any) *mcp.CallToolResult {
		t.Helper()
		r, e := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	if !call("get_docx", map[string]any{}).IsError {
		t.Fatal("unopened DOCX read succeeded")
	}
	path := filepath.Join(t.TempDir(), "new.docx")
	result := call("create_docx", docxCreateInput{Path: path, Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "original"}, {Type: "table", Rows: [][]string{{"cell"}}}}})
	if result.IsError {
		t.Fatalf("create failed: %+v", result)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var opened docxOpenResponse
	if e := json.Unmarshal(encoded, &opened); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(opened.BrowserURL, "?format=docx") {
		t.Fatal("DOCX launch URL missing")
	}
	token := strings.TrimPrefix(opened.BrowserURL, api.browserBaseURL)
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", token, nil))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/docx?document="+opened.Document.ID {
		t.Fatal("launch did not redirect to DOCX")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing browser cookie")
	}
	request := func(method, route, origin string, input any) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		if input != nil {
			json.NewEncoder(&body).Encode(input)
		}
		r := httptest.NewRequest(method, route, &body)
		r.AddCookie(cookies[0])
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, r)
		return w
	}
	snap := opened.Document
	run := snap.Blocks[0].Paragraph.Runs[0]
	input := docx.ApplyInput{SessionID: snap.ID, BaseRevision: snap.Revision, Operations: []docx.Operation{{Type: "replace_run_text", TargetID: run.ID, ExpectedText: run.Text, Text: "AI saved"}}}
	if call("apply_docx_operations", input).IsError {
		t.Fatal("AI commit failed")
	}
	r := request("GET", "/api/docx", "", nil)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if e := json.Unmarshal(r.Body.Bytes(), &snap); e != nil {
		t.Fatal(e)
	}
	if snap.Revision != 2 || snap.Blocks[0].Paragraph.Runs[0].Text != "AI saved" {
		t.Fatal("browser did not see MCP commit")
	}
	if r = request("POST", "/api/docx/operations", "", input); r.Code != 409 {
		t.Fatal("stale browser write accepted")
	}
	cell := snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0]
	input = docx.ApplyInput{SessionID: snap.ID, BaseRevision: 2, Operations: []docx.Operation{{Type: "replace_cell_text", TargetID: cell.ID, ExpectedText: cell.Text, Text: "human saved"}}}
	if r = request("POST", "/api/docx/operations", "https://evil.example", input); r.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	r = request("POST", "/api/docx/operations", "", input)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	result = call("get_docx", map[string]any{})
	encoded, _ = json.Marshal(result.StructuredContent)
	json.Unmarshal(encoded, &snap)
	if snap.Revision != 3 || snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "human saved" {
		t.Fatal("MCP did not see human commit")
	}
	reopened, e := docx.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	disk, e := reopened.Snapshot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if disk.Blocks[0].Paragraph.Runs[0].Text != "AI saved" {
		t.Fatal("commit did not reach disk")
	}

	history := docx.HistoryInput{SessionID: snap.ID, BaseRevision: 3, Direction: "undo"}
	if r = request("POST", "/api/docx/history", "https://evil.example", history); r.Code != 403 {
		t.Fatal("cross-origin history accepted")
	}
	r = request("POST", "/api/docx/history", "", history)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if err := json.Unmarshal(r.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Revision != 4 || snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "cell" || snap.History.RedoActor != "human" {
		t.Fatalf("undo mismatch: %+v", snap)
	}
	if r = request("POST", "/api/docx/history", "", history); r.Code != 409 {
		t.Fatal("stale history accepted")
	}
	history.BaseRevision = 4
	history.Direction = "redo"
	if call("restore_docx_history", history).IsError {
		t.Fatal("MCP redo failed")
	}
	r = request("GET", "/api/docx", "", nil)
	if err := json.Unmarshal(r.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Revision != 5 || snap.Blocks[1].Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "human saved" {
		t.Fatal("browser did not see MCP redo")
	}
	history.BaseRevision = 5
	history.SessionID = "wrong-document"
	if !call("restore_docx_history", history).IsError {
		t.Fatal("foreign session history accepted")
	}
	result = call("create_docx", docxCreateInput{Path: path, Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "overwrite"}}})
	if !result.IsError {
		t.Fatal("create overwrote file")
	}
	if call("open_docx", docxOpenInput{Path: "relative.docx"}).IsError == false {
		t.Fatal("relative path accepted")
	}
	page := request("GET", "/docx", "", nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "DOCX") {
		t.Fatal("browser entry missing")
	}
}
func TestDOCXUnauthorizedAndMalformedRequests(t *testing.T) {
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.docx")
	_, e := api.createDOCX(ctx, docxCreateInput{Path: path, Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "original"}}})
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, route := range []string{"/docx", "/api/docx", "/api/docx/events", "/api/docx/operations", "/api/docx/history"} {
		method := "GET"
		if strings.HasSuffix(route, "operations") || strings.HasSuffix(route, "history") {
			method = "POST"
		}
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, httptest.NewRequest(method, route, strings.NewReader(`{}`)))
		if w.Code != 401 {
			t.Fatalf("unauthenticated %s returned %d", route, w.Code)
		}
	}
	for _, body := range []string{`{} {}`, `{"unknown":true}`, `{`, strings.Repeat(" ", 1<<20) + `{}`} {
		r := httptest.NewRequest("POST", "/api/docx/operations", strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "browser-token"})
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("malformed input returned %d", w.Code)
		}
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("negative request changed document")
	}
}

func TestDOCXSSECommitAndSessionResync(t *testing.T) {
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	path := filepath.Join(t.TempDir(), "events.docx")
	snapshot, err := api.createDOCX(context.Background(), docxCreateInput{Path: path, Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "before"}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/docx/events?session="+snapshot.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: browserSessionCookieName(request.Host), Value: "browser-token"})
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("SSE status = %d", response.StatusCode)
	}
	run := snapshot.Blocks[0].Paragraph.Runs[0]
	_, err = api.applyDOCX(ctx, docx.ApplyInput{SessionID: snapshot.ID, BaseRevision: 1, Operations: []docx.Operation{{Type: "replace_run_text", TargetID: run.ID, ExpectedText: run.Text, Text: "after"}}}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(response.Body)
	found := false
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data: ") {
			var event docx.Event
			if err := json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.Type != "docx.committed" || event.Sequence != 1 || event.Revision != 2 || event.Actor != "ai" {
				t.Fatalf("invalid event: %+v", event)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no SSE commit: %v", scanner.Err())
	}
	switchedPath := filepath.Join(t.TempDir(), "switched.docx")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(switchedPath, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = api.OpenDOCX(ctx, switchedPath); err != nil {
		t.Fatal(err)
	}
	found = false
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data: ") {
			var event docx.Event
			json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), "data: ")), &event)
			if event.Type != "docx.reload" {
				t.Fatalf("session switch did not resync: %+v", event)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no SSE resync: %v", scanner.Err())
	}
}

func TestDOCXParagraphPlanThroughMCPAndHTTP(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "paragraphs.docx")
	_, err := docx.Create(ctx, path, []docx.InitialBlock{{Type: "paragraph", Text: "before"}})
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	if _, err := api.OpenDOCX(ctx, path); err != nil {
		t.Fatal(err)
	}
	initial, err := api.docxSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := initial.Blocks[0].Paragraph
	input := docx.ApplyInput{SessionID: initial.ID, BaseRevision: initial.Revision, Operations: []docx.Operation{{Type: "replace_paragraphs", TargetID: p.ID, EndTargetID: p.ID, ExpectedText: "before", Paragraphs: []docx.ParagraphContent{
		{SourceID: p.ID, Runs: []docx.RunContent{{SourceID: p.Runs[0].ID, Text: "first"}}},
		{SourceID: p.ID, Runs: []docx.RunContent{{SourceID: p.Runs[0].ID, Text: "second"}}},
	}}}}
	client := docxClient(t, api)
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: input})
	if err != nil || result.IsError {
		t.Fatalf("MCP paragraph edit: %v %+v", err, result)
	}
	// The authenticated HTTP read observes the same revision and actual structure.
	req := httptest.NewRequest("GET", "/api/docx", nil)
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName(req.Host), Value: api.browserToken})
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Fatal(recorder.Body.String())
	}
	var after docx.Snapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Revision != initial.Revision+1 || len(after.Blocks) != 2 || after.Blocks[1].Paragraph.Runs[0].Text != "second" {
		t.Fatal("HTTP did not observe structured MCP commit")
	}
}

func TestDOCXMCPStyleDefinitionAndApplicationWithoutTextFields(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	path := filepath.Join(t.TempDir(), "styles.docx")
	opened, err := api.createDOCX(ctx, docxCreateInput{Path: path, Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "heading"}}})
	if err != nil {
		t.Fatal(err)
	}
	p := opened.Blocks[0].Paragraph
	client := docxClient(t, api)
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: map[string]any{"sessionId": opened.ID, "baseRevision": opened.Revision, "operations": []any{
		map[string]any{"type": "define_style", "styleDefinition": map[string]any{"id": "CustomHeading", "name": "제목", "kind": "paragraph", "run": map[string]any{"bold": true}}},
		map[string]any{"type": "replace_paragraphs", "targetId": p.ID, "endTargetId": p.ID, "expectedText": "heading", "paragraphs": []any{map[string]any{"sourceId": p.ID, "format": map[string]any{"styleId": "CustomHeading"}, "runs": []any{map[string]any{"sourceId": p.Runs[0].ID, "text": "heading"}}}}},
	}}})
	if err != nil || result.IsError {
		t.Fatalf("MCP style batch failed: %+v %v", result, err)
	}
	snapshot, err := api.docxSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 || !snapshot.Blocks[0].Paragraph.Runs[0].Bold || snapshot.Blocks[0].Paragraph.StyleID != "CustomHeading" {
		t.Fatal("MCP style batch was not shared", snapshot)
	}
}

func TestDOCXSectionSettingsThroughMCPAndHTTP(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	opened, err := api.createDOCX(ctx, docxCreateInput{Path: filepath.Join(t.TempDir(), "sections.docx"), Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "content"}}})
	if err != nil {
		t.Fatal(err)
	}
	client := docxClient(t, api)
	section := map[string]any{
		"width": 16838, "height": 11906, "orientation": "landscape", "columns": 2,
		"pageNumberStart": 25, "pageNumberFormat": "upperRoman", "headerDistance": 480, "footerDistance": 0,
	}
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: map[string]any{
		"sessionId": opened.ID, "baseRevision": opened.Revision,
		"operations": []any{map[string]any{"type": "update_section", "sectionId": opened.Sections[0].ID, "section": section}},
	}})
	if err != nil || result.IsError {
		t.Fatalf("section MCP: %v %+v", err, result)
	}
	req := httptest.NewRequest("GET", "/api/docx", nil)
	req.AddCookie(&http.Cookie{Name: browserSessionCookieName(req.Host), Value: api.browserToken})
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, req)
	var after docx.Snapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &after); err != nil || recorder.Code != 200 {
		t.Fatalf("HTTP read: %s", recorder.Body.String())
	}
	if after.Revision != opened.Revision+1 || after.Sections[0].Columns != 2 || after.Sections[0].Page.Width != 16838 || after.Blocks[0].Paragraph.Runs[0].Text != "content" {
		t.Fatal("section settings not shared", after)
	}
	if n := after.Sections[0]; n.PageNumberStart == nil || *n.PageNumberStart != 25 || n.PageNumberFormat != "upperRoman" {
		t.Fatal("section numbering not shared", n)
	}
	if n := after.Sections[0]; n.HeaderDistance == nil || *n.HeaderDistance != 480 || n.FooterDistance == nil || *n.FooterDistance != 0 {
		t.Fatal("section header and footer distances not shared", n)
	}
}

func TestDOCXMCPHeaderAndFooterContracts(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	opened, err := api.createDOCX(ctx, docxCreateInput{Path: filepath.Join(t.TempDir(), "headers.docx"), Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "body"}}})
	if err != nil {
		t.Fatal(err)
	}
	client := docxClient(t, api)
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: map[string]any{"sessionId": opened.ID, "baseRevision": opened.Revision, "operations": []any{map[string]any{"type": "update_section", "sectionId": opened.Sections[0].ID, "section": map[string]any{"firstPage": true, "headerFooters": []any{map[string]any{"kind": "header", "variant": "first", "text": "title header"}, map[string]any{"kind": "footer", "variant": "default", "text": "footer"}}}}}}})
	if err != nil || result.IsError {
		t.Fatalf("header MCP: %v %+v", err, result)
	}
	snapshot, err := api.docxSnapshot(ctx)
	if err != nil || !snapshot.Sections[0].FirstPage {
		t.Fatal("first page flag missing", err)
	}
	found := false
	for _, info := range snapshot.Sections[0].HeaderFooters {
		if info.Kind == "header" && info.Variant == "first" && info.Text == "title header" && info.Editable {
			found = true
		}
	}
	if !found || snapshot.Blocks[0].Paragraph.Runs[0].Text != "body" {
		t.Fatal("header metadata mismatch", snapshot)
	}
}

func TestDOCXPictureRequiresSessionAndCurrentRevision(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	opened, err := api.createDOCX(ctx, docxCreateInput{Path: filepath.Join(t.TempDir(), "picture.docx"), Blocks: []docx.InitialBlock{{Type: "paragraph", Text: "body"}}})
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 1))); err != nil {
		t.Fatal(err)
	}
	client := docxClient(t, api)
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: map[string]any{
		"sessionId": opened.ID, "baseRevision": opened.Revision, "operations": []any{map[string]any{
			"type": "update_image", "targetId": opened.Blocks[0].ID, "image": map[string]any{"action": "insert", "base64": base64.StdEncoding.EncodeToString(raw.Bytes())},
		}},
	}})
	if err != nil || result.IsError {
		t.Fatalf("MCP picture: %v %+v", err, result)
	}
	snapshot, err := api.docxSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot.Blocks[0].Paragraph.Runs[1].ID
	request := func(id string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/docx/picture?id="+url.QueryEscape(id), nil)
		if auth {
			req.AddCookie(&http.Cookie{Name: browserSessionCookieName(req.Host), Value: api.browserToken})
		}
		r := httptest.NewRecorder()
		api.Handler().ServeHTTP(r, req)
		return r
	}
	if response := request(id, false); response.Code != 401 {
		t.Fatalf("unauthenticated image: %d", response.Code)
	}
	if response := request(id, true); response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(raw.Bytes(), response.Body.Bytes()) {
		t.Fatalf("picture response: %d %s", response.Code, response.Header().Get("Content-Type"))
	}
	if response := request(opened.Blocks[0].Paragraph.Runs[0].ID, true); response.Code != 404 {
		t.Fatalf("stale picture: %d", response.Code)
	}
}

func TestDOCXMCPListsSupportedOperationFamilies(t *testing.T) {
	ctx := context.Background()
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	client := docxClient(t, api)
	result, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range result.Tools {
		if tool.Name != "apply_docx_operations" {
			continue
		}
		for _, term := range []string{"baseRevision", "upperLetter", "NUMPAGES", "resolve_comment", "track_run_text"} {
			if !strings.Contains(tool.Description, term) {
				t.Fatalf("DOCX tool description omits %q", term)
			}
		}
		return
	}
	t.Fatal("DOCX operations tool is missing from the MCP list")
}

func TestDOCXMCPInsertsPlainContentControl(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "controls.docx")
	if _, err := docx.Create(ctx, path, []docx.InitialBlock{{Type: "paragraph", Text: "Prefix"}}); err != nil {
		t.Fatal(err)
	}
	api := NewWithTokens(nil, "browser-token", "mcp-token")
	defer api.Close()
	initial, err := api.OpenDOCX(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	client := docxClient(t, api)
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "apply_docx_operations", Arguments: docx.ApplyInput{
		SessionID: initial.ID, BaseRevision: initial.Revision,
		Operations: []docx.Operation{{Type: "insert_control_text", TargetID: initial.Blocks[0].ID, Text: "Control value"}},
	}})
	if err != nil || result.IsError {
		t.Fatalf("MCP content control insertion: %v %+v", err, result)
	}
	after, err := api.docxSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runs := after.Blocks[0].Paragraph.Runs
	if after.Revision != initial.Revision+1 || len(runs) != 2 || runs[0].Text != "Prefix" ||
		runs[1].Object == nil || runs[1].Object.Kind != "control" || runs[1].Object.Text != "Control value" || !runs[1].Object.Editable {
		t.Fatal("shared DOCX session did not expose the inserted control")
	}
}
