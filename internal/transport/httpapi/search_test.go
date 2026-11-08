package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/foloom/internal/formats/xlsx"
)

func TestSearchHTTPAndMCP(t *testing.T) {
	server, _ := newSecurityServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/search?query=private", nil)
	request.AddCookie(&http.Cookie{Name: "xlsx_session", Value: securityBrowserToken})
	server.Handler().ServeHTTP(response, request)
	var result xlsx.SearchResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Hits) != 1 {
		t.Fatalf("HTTP search: %d %v", response.Code, err)
	}
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.mcpServer().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "search-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	found, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_workbook", Arguments: map[string]any{"query": "private"}})
	if err != nil || found.IsError {
		t.Fatalf("MCP search: %+v %v", found, err)
	}
	encoded, _ := json.Marshal(found.StructuredContent)
	if err := json.Unmarshal(encoded, &result); err != nil || len(result.Hits) != 1 {
		t.Fatalf("MCP result: %v", err)
	}
}
