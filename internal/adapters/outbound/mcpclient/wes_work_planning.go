package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// WesWorkPlanning implements ports.WesWorkPlanningClient by calling
// wes-work-planning's published get_backlog_telemetry and
// get_rebalance_recommendation MCP tools.
type WesWorkPlanning struct {
	session *Session
}

// NewWesWorkPlanning builds a WesWorkPlanning client for the given
// connection config.
func NewWesWorkPlanning(cfg Config) *WesWorkPlanning {
	cfg.Name = "wes-work-planning"
	return &WesWorkPlanning{session: New(cfg)}
}

var _ ports.WesWorkPlanningClient = (*WesWorkPlanning)(nil)

func (c *WesWorkPlanning) GetBacklogTelemetry(ctx context.Context, pathId string) (ports.BacklogTelemetry, error) {
	var out ports.BacklogTelemetry
	err := c.session.callTool(ctx, "get_backlog_telemetry", map[string]any{"pathId": pathId}, &out)
	return out, err
}

func (c *WesWorkPlanning) GetRebalanceRecommendation(ctx context.Context, pathId string) (ports.RebalanceRecommendation, error) {
	var out ports.RebalanceRecommendation
	err := c.session.callTool(ctx, "get_rebalance_recommendation", map[string]any{"pathId": pathId}, &out)
	return out, err
}
