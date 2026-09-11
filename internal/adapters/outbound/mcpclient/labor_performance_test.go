package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type associateScorecardTestIn struct {
	AssociateId string `json:"associateId"`
}

type taskTypePerformanceTestIn struct {
	TaskType string `json:"taskType"`
}

type laborStandardTestIn struct {
	TaskType string `json:"taskType"`
}

// newLaborPerformanceTestUpstream serves a real Streamable-HTTP MCP
// server exposing all three labor-performance tools, unauthenticated, so
// LaborPerformance is exercised over the wire exactly as it would be
// against labor-performance's cmd/labor's MCP adapter.
func newLaborPerformanceTestUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "labor-performance-test", Version: "0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "get_associate_scorecard"}, func(_ context.Context, _ *mcp.CallToolRequest, in associateScorecardTestIn) (*mcp.CallToolResult, ports.AssociateScorecard, error) {
		if in.AssociateId == "" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "associateId is required"}}}, ports.AssociateScorecard{}, nil
		}
		pct := 87.5
		return nil, ports.AssociateScorecard{
			AssociateId:       in.AssociateId,
			TaskCount:         10,
			MeanEfficiencyPct: &pct,
			ByTaskType: map[string]ports.TaskTypeBreakdown{
				"PICK": {TaskCount: 10, MeanEfficiencyPct: &pct},
			},
			Trend:        "STABLE",
			CoachingFlag: false,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "get_task_type_performance"}, func(_ context.Context, _ *mcp.CallToolRequest, in taskTypePerformanceTestIn) (*mcp.CallToolResult, ports.TaskTypePerformance, error) {
		pct := 91.2
		secs := 42.5
		return nil, ports.TaskTypePerformance{
			TaskType:          in.TaskType,
			TaskCount:         100,
			MeanEfficiencyPct: &pct,
			MeanActualSeconds: &secs,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{Name: "get_labor_standard"}, func(_ context.Context, _ *mcp.CallToolRequest, in laborStandardTestIn) (*mcp.CallToolResult, ports.LaborStandard, error) {
		return nil, ports.LaborStandard{
			TaskType:        in.TaskType,
			ExpectedSeconds: 45,
			EffectiveFrom:   "2026-01-01T00:00:00Z",
		}, nil
	})

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(h)
}

func TestLaborPerformance_GetAssociateScorecard(t *testing.T) {
	up := newLaborPerformanceTestUpstream(t)
	defer up.Close()

	c := NewLaborPerformance(Config{Endpoint: up.URL})
	out, err := c.GetAssociateScorecard(context.Background(), "assoc-1")
	if err != nil {
		t.Fatalf("GetAssociateScorecard: %v", err)
	}
	if out.AssociateId != "assoc-1" || out.TaskCount != 10 || out.Trend != "STABLE" {
		t.Fatalf("unexpected scorecard: %+v", out)
	}
	if out.MeanEfficiencyPct == nil || *out.MeanEfficiencyPct != 87.5 {
		t.Fatalf("unexpected mean efficiency: %+v", out.MeanEfficiencyPct)
	}
	if b, ok := out.ByTaskType["PICK"]; !ok || b.TaskCount != 10 {
		t.Fatalf("unexpected byTaskType: %+v", out.ByTaskType)
	}
}

func TestLaborPerformance_GetAssociateScorecard_ToolError(t *testing.T) {
	up := newLaborPerformanceTestUpstream(t)
	defer up.Close()

	c := NewLaborPerformance(Config{Endpoint: up.URL})
	if _, err := c.GetAssociateScorecard(context.Background(), ""); err == nil {
		t.Fatal("expected an error for a tool-level failure")
	}
}

func TestLaborPerformance_GetTaskTypePerformance(t *testing.T) {
	up := newLaborPerformanceTestUpstream(t)
	defer up.Close()

	c := NewLaborPerformance(Config{Endpoint: up.URL})
	out, err := c.GetTaskTypePerformance(context.Background(), "PICK")
	if err != nil {
		t.Fatalf("GetTaskTypePerformance: %v", err)
	}
	if out.TaskType != "PICK" || out.TaskCount != 100 {
		t.Fatalf("unexpected performance: %+v", out)
	}
	if out.MeanEfficiencyPct == nil || *out.MeanEfficiencyPct != 91.2 {
		t.Fatalf("unexpected mean efficiency: %+v", out.MeanEfficiencyPct)
	}
	if out.MeanActualSeconds == nil || *out.MeanActualSeconds != 42.5 {
		t.Fatalf("unexpected mean actual seconds: %+v", out.MeanActualSeconds)
	}
}

func TestLaborPerformance_GetLaborStandard(t *testing.T) {
	up := newLaborPerformanceTestUpstream(t)
	defer up.Close()

	c := NewLaborPerformance(Config{Endpoint: up.URL})
	out, err := c.GetLaborStandard(context.Background(), "PACK")
	if err != nil {
		t.Fatalf("GetLaborStandard: %v", err)
	}
	if out.TaskType != "PACK" || out.ExpectedSeconds != 45 || out.EffectiveFrom == "" {
		t.Fatalf("unexpected standard: %+v", out)
	}
	if out.EffectiveTo != nil {
		t.Fatalf("expected no EffectiveTo, got %v", *out.EffectiveTo)
	}
}

func TestLaborPerformance_UnreachableUpstream(t *testing.T) {
	up := newLaborPerformanceTestUpstream(t)
	up.Close() // closed before use: any call must surface a connection error.

	c := NewLaborPerformance(Config{Endpoint: up.URL})
	if _, err := c.GetLaborStandard(context.Background(), "PICK"); err == nil {
		t.Fatal("an unreachable upstream endpoint must surface as an error")
	}
}

func TestLaborPerformance_ImplementsPort(t *testing.T) {
	var _ ports.LaborPerformanceClient = NewLaborPerformance(Config{Endpoint: "http://example.invalid"})
}
