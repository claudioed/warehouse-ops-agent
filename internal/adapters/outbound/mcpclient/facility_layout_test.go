package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type estimateTravelDistanceTestIn struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// newFacilityLayoutTravelTestUpstream serves a real Streamable-HTTP MCP
// server exposing only estimate_travel_distance, unauthenticated, so
// FacilityLayout.EstimateTravelDistance is exercised over the wire
// exactly as it would be against facility-layout's own cmd/facility MCP
// adapter (ADR 0017 there).
func newFacilityLayoutTravelTestUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "facility-layout-test", Version: "0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "estimate_travel_distance"}, func(_ context.Context, _ *mcp.CallToolRequest, in estimateTravelDistanceTestIn) (*mcp.CallToolResult, ports.TravelDistance, error) {
		if in.From == "" || in.To == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "from and to are both required"}}}, ports.TravelDistance{}, nil
		}
		return nil, ports.TravelDistance{
			MetresM:   72.5,
			Estimated: true,
			Route: []ports.TravelNode{
				{AisleID: "A07", Bay: "01"},
				{AisleID: "A09", Bay: "03"},
			},
		}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(h)
}

func TestFacilityLayout_EstimateTravelDistance(t *testing.T) {
	up := newFacilityLayoutTravelTestUpstream(t)
	defer up.Close()

	c := NewFacilityLayout(Config{Endpoint: up.URL})
	out, err := c.EstimateTravelDistance(context.Background(), "WH1-STOR-AMB-A07-01-01-A", "WH1-STOR-AMB-A09-03-01-A")
	if err != nil {
		t.Fatalf("EstimateTravelDistance: %v", err)
	}
	if out.MetresM != 72.5 || !out.Estimated {
		t.Fatalf("unexpected travel distance: %+v", out)
	}
	if len(out.Route) != 2 || out.Route[0].AisleID != "A07" || out.Route[1].AisleID != "A09" {
		t.Fatalf("unexpected route: %+v", out.Route)
	}
}

func TestFacilityLayout_EstimateTravelDistance_ToolError(t *testing.T) {
	up := newFacilityLayoutTravelTestUpstream(t)
	defer up.Close()

	c := NewFacilityLayout(Config{Endpoint: up.URL})
	if _, err := c.EstimateTravelDistance(context.Background(), "", "WH1-STOR-AMB-A09-03-01-A"); err == nil {
		t.Fatal("expected an error for a tool-level failure (missing from)")
	}
}

func TestFacilityLayout_EstimateTravelDistance_UnreachableUpstream(t *testing.T) {
	up := newFacilityLayoutTravelTestUpstream(t)
	up.Close() // closed before use: any call must surface a connection error.

	c := NewFacilityLayout(Config{Endpoint: up.URL})
	if _, err := c.EstimateTravelDistance(context.Background(), "WH1-STOR-AMB-A07-01-01-A", "WH1-STOR-AMB-A09-03-01-A"); err == nil {
		t.Fatal("an unreachable upstream endpoint must surface as an error")
	}
}
