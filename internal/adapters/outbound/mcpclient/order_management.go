package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// OrderManagement implements ports.OrderManagementMCPClient by calling
// order-management's published get_order MCP tool.
type OrderManagement struct {
	session *Session
}

// NewOrderManagement builds an OrderManagement client for the given
// connection config.
func NewOrderManagement(cfg Config) *OrderManagement {
	cfg.Name = "order-management"
	return &OrderManagement{session: New(cfg)}
}

var _ ports.OrderManagementMCPClient = (*OrderManagement)(nil)

func (c *OrderManagement) GetOrder(ctx context.Context, orderId string) (ports.Order, error) {
	var out ports.Order
	err := c.session.callTool(ctx, "get_order", map[string]any{"orderId": orderId}, &out)
	return out, err
}
