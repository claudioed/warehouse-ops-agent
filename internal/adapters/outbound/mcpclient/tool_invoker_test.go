package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type gapIn struct {
	PathId string `json:"pathId"`
}
type gapOut struct {
	PathId       string `json:"pathId"`
	PlannedHeads int    `json:"plannedHeads"`
}

// newTestUpstream serves a real Streamable-HTTP MCP server with two tools,
// unauthenticated, so the invoker is exercised over the wire exactly as it
// would be against a context's cmd/mcp.
func newTestUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-upstream", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_staffing_gap", Description: "Planned vs active heads."}, func(_ context.Context, _ *mcp.CallToolRequest, in gapIn) (*mcp.CallToolResult, gapOut, error) {
		if in.PathId == "" {
			return nil, gapOut{}, nil
		}
		return nil, gapOut{PathId: in.PathId, PlannedHeads: 4}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "assign_labor", Description: "WRITE tool, must never be reachable."}, func(_ context.Context, _ *mcp.CallToolRequest, in gapIn) (*mcp.CallToolResult, gapOut, error) {
		t.Fatal("write tool was invoked")
		return nil, gapOut{}, nil
	})
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(h)
}

func TestToolInvoker_AllowListAndWire(t *testing.T) {
	up := newTestUpstream(t)
	defer up.Close()
	inv := NewToolInvoker(
		map[string]*Session{
			"workforce-management": New(Config{Name: "workforce-management", Endpoint: up.URL}),
			"unset":                New(Config{Name: "unset", Endpoint: ""}),
		},
		map[string][]string{"workforce-management": {"get_staffing_gap"}},
	)
	ctx := context.Background()

	out, err := inv.Invoke(ctx, "workforce-management", "get_staffing_gap", map[string]any{"pathId": "pick"})
	if err != nil || !strings.Contains(out, `"plannedHeads":4`) {
		t.Fatalf("invoke: out=%s err=%v", out, err)
	}
	if _, err := inv.Invoke(ctx, "workforce-management", "assign_labor", nil); err == nil || !strings.Contains(err.Error(), "allow-list") {
		t.Fatalf("write tool must be refused before any network call, got %v", err)
	}
	if _, err := inv.Invoke(ctx, "unset", "get_staffing_gap", nil); err == nil || !strings.Contains(err.Error(), "unknown upstream") {
		t.Fatalf("unset upstream must be unknown, got %v", err)
	}
	if _, err := inv.Invoke(ctx, "nope", "x", nil); err == nil {
		t.Fatal("unknown upstream must error")
	}

	specs, errs := inv.Specs(ctx)
	if len(errs) != 0 {
		t.Fatalf("specs errors: %v", errs)
	}
	if len(specs) != 1 || specs[0].Name != "get_staffing_gap" || specs[0].Upstream != "workforce-management" || specs[0].Description == "" {
		t.Fatalf("specs must contain only the allow-listed tool with its upstream description: %+v", specs)
	}
	if specs[0].InputSchema["type"] != "object" {
		t.Fatalf("input schema must be the upstream's JSON schema: %+v", specs[0].InputSchema)
	}
	// Cached: a second call must not re-list.
	again, _ := inv.Specs(ctx)
	if &again[0] != &specs[0] {
		t.Fatal("specs must be cached after a successful discovery")
	}
}

func TestToolInvoker_UpstreamErrorIsSurfaced(t *testing.T) {
	up := newTestUpstream(t)
	up.Close() // closed before use: any call must surface a connection error.
	inv := NewToolInvoker(map[string]*Session{"wfm": New(Config{Name: "wfm", Endpoint: up.URL})}, map[string][]string{"wfm": {"get_staffing_gap"}})
	if _, err := inv.Invoke(context.Background(), "wfm", "get_staffing_gap", map[string]any{"pathId": "pick"}); err == nil {
		t.Fatal("an unreachable upstream endpoint must surface as an error")
	}
	if _, errs := inv.Specs(context.Background()); len(errs) != 1 {
		t.Fatalf("discovery failure must be reported, got %v", errs)
	}
}
