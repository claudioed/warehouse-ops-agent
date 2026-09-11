package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type getOrderTestIn struct {
	OrderId string `json:"orderId"`
}

// newOrderManagementTestUpstream serves a real Streamable-HTTP MCP server
// exposing get_order, unauthenticated, so OrderManagement is exercised
// over the wire exactly as it would be against order-management's cmd/mcp.
func newOrderManagementTestUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "order-management-test", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_order"}, func(_ context.Context, _ *mcp.CallToolRequest, in getOrderTestIn) (*mcp.CallToolResult, ports.Order, error) {
		if in.OrderId == "missing" {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "order not found"}}}, ports.Order{}, nil
		}
		reservationId := "res-1"
		return nil, ports.Order{
			Id:                   in.OrderId,
			Status:               "ALLOCATED",
			AllowPartialShipment: true,
			Lines: []ports.OrderLine{
				{LineNo: 1, SKU: "SKU-1", Quantity: 2, PathId: "pick-zone-a", GiftWrap: false, Status: "ALLOCATED", ReservationId: &reservationId},
			},
		}, nil
	})
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(h)
}

func TestOrderManagement_GetOrder(t *testing.T) {
	up := newOrderManagementTestUpstream(t)
	defer up.Close()

	c := NewOrderManagement(Config{Endpoint: up.URL})
	out, err := c.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if out.Id != "order-1" || out.Status != "ALLOCATED" || !out.AllowPartialShipment {
		t.Fatalf("unexpected order: %+v", out)
	}
	if len(out.Lines) != 1 || out.Lines[0].SKU != "SKU-1" || out.Lines[0].ReservationId == nil || *out.Lines[0].ReservationId != "res-1" {
		t.Fatalf("unexpected lines: %+v", out.Lines)
	}
}

func TestOrderManagement_GetOrder_ToolError(t *testing.T) {
	up := newOrderManagementTestUpstream(t)
	defer up.Close()

	c := NewOrderManagement(Config{Endpoint: up.URL})
	if _, err := c.GetOrder(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error for a tool-level failure")
	}
}

func TestOrderManagement_GetOrder_UnreachableUpstream(t *testing.T) {
	up := newOrderManagementTestUpstream(t)
	up.Close() // closed before use: any call must surface a connection error.

	c := NewOrderManagement(Config{Endpoint: up.URL})
	if _, err := c.GetOrder(context.Background(), "order-1"); err == nil {
		t.Fatal("an unreachable upstream endpoint must surface as an error")
	}
}

func TestOrderManagement_ImplementsPort(t *testing.T) {
	var _ ports.OrderManagementMCPClient = NewOrderManagement(Config{Endpoint: "http://example.invalid"})
}
