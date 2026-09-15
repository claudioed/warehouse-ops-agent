package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type getProcessPathTestIn struct {
	PathId string `json:"pathId"`
}

type listProcessPathsTestIn struct {
	ActiveOnly *bool `json:"activeOnly,omitempty"`
}

type listProcessPathsTestOut struct {
	Paths []ports.ProcessPath `json:"paths"`
}

// newProcessPathManagementTestUpstream serves a real Streamable-HTTP MCP
// server exposing get_process_path and list_process_paths,
// unauthenticated, so ProcessPathManagement is exercised over the wire
// exactly as it would be against process-path-management's cmd/pathmgmt.
func newProcessPathManagementTestUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "process-path-management-test", Version: "0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "get_process_path"}, func(_ context.Context, _ *mcp.CallToolRequest, in getProcessPathTestIn) (*mcp.CallToolResult, ports.ProcessPath, error) {
		if in.PathId == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "pathId is required"}}}, ports.ProcessPath{}, nil
		}
		return nil, ports.ProcessPath{
			PathId:               in.PathId,
			MatchPrefix:          "pick",
			Direct:               true,
			RequiredCapabilities: []string{"SCAN"},
			Status:               "ACTIVE",
			Active:               true,
			CreatedAt:            "2026-01-01T00:00:00Z",
			UpdatedAt:            "2026-01-01T00:00:00Z",
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "list_process_paths"}, func(_ context.Context, _ *mcp.CallToolRequest, in listProcessPathsTestIn) (*mcp.CallToolResult, listProcessPathsTestOut, error) {
		activeOnly := true
		if in.ActiveOnly != nil {
			activeOnly = *in.ActiveOnly
		}
		paths := []ports.ProcessPath{
			{PathId: "PICK", MatchPrefix: "pick", Direct: true, Status: "ACTIVE", Active: true},
		}
		if !activeOnly {
			paths = append(paths, ports.ProcessPath{PathId: "OLD", MatchPrefix: "old", Direct: false, Status: "DEACTIVATED", Active: false})
		}
		return nil, listProcessPathsTestOut{Paths: paths}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(h)
}

func TestProcessPathManagement_GetProcessPath(t *testing.T) {
	up := newProcessPathManagementTestUpstream(t)
	defer up.Close()

	c := NewProcessPathManagement(Config{Endpoint: up.URL})
	out, err := c.GetProcessPath(context.Background(), "PICK")
	if err != nil {
		t.Fatalf("GetProcessPath: %v", err)
	}
	if out.PathId != "PICK" || out.MatchPrefix != "pick" || !out.Direct || !out.Active {
		t.Fatalf("unexpected process path: %+v", out)
	}
}

func TestProcessPathManagement_GetProcessPath_ToolError(t *testing.T) {
	up := newProcessPathManagementTestUpstream(t)
	defer up.Close()

	c := NewProcessPathManagement(Config{Endpoint: up.URL})
	if _, err := c.GetProcessPath(context.Background(), ""); err == nil {
		t.Fatal("expected an error for a tool-level failure")
	}
}

func TestProcessPathManagement_ListProcessPaths_ActiveOnly(t *testing.T) {
	up := newProcessPathManagementTestUpstream(t)
	defer up.Close()

	c := NewProcessPathManagement(Config{Endpoint: up.URL})
	out, err := c.ListProcessPaths(context.Background(), true)
	if err != nil {
		t.Fatalf("ListProcessPaths: %v", err)
	}
	if len(out.Paths) != 1 || out.Paths[0].PathId != "PICK" {
		t.Fatalf("unexpected active-only paths: %+v", out.Paths)
	}
}

func TestProcessPathManagement_ListProcessPaths_IncludeInactive(t *testing.T) {
	up := newProcessPathManagementTestUpstream(t)
	defer up.Close()

	c := NewProcessPathManagement(Config{Endpoint: up.URL})
	out, err := c.ListProcessPaths(context.Background(), false)
	if err != nil {
		t.Fatalf("ListProcessPaths: %v", err)
	}
	if len(out.Paths) != 2 {
		t.Fatalf("unexpected all paths: %+v", out.Paths)
	}
}

func TestProcessPathManagement_UnreachableUpstream(t *testing.T) {
	up := newProcessPathManagementTestUpstream(t)
	up.Close() // closed before use: any call must surface a connection error.

	c := NewProcessPathManagement(Config{Endpoint: up.URL})
	if _, err := c.GetProcessPath(context.Background(), "PICK"); err == nil {
		t.Fatal("an unreachable upstream endpoint must surface as an error")
	}
}

func TestProcessPathManagement_ImplementsPort(t *testing.T) {
	var _ ports.ProcessPathManagementClient = NewProcessPathManagement(Config{Endpoint: "http://example.invalid"})
}
