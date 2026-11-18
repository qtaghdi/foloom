package httpapi

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/foloom/internal/formats/docx"
)

func TestDOCXMCPTracksParagraphMerge(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "merge.docx")
	if _, err := docx.Create(ctx, path, []docx.InitialBlock{
		{Type: "paragraph", Text: "First "},
		{Type: "paragraph", Text: "Second"},
	}); err != nil {
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
		Operations: []docx.Operation{{Type: "track_paragraph_merge", TargetID: initial.Blocks[0].ID}},
	}})
	if err != nil || result.IsError {
		t.Fatalf("MCP tracked merge: %v %+v", err, result)
	}
	after, err := api.docxSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mark := after.Blocks[0].Paragraph.BreakRevision
	if len(after.Blocks) != 2 || after.Revision != initial.Revision+1 || mark == nil ||
		!mark.Editable || mark.Kind != "tracked_delete" || mark.Author != "AI" {
		t.Fatalf("shared DOCX session did not expose the tracked merge: %+v", mark)
	}
}
