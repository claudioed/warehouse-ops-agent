package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// LaborPerformance implements ports.LaborPerformanceClient by calling
// labor-performance's published get_associate_scorecard,
// get_task_type_performance, and get_labor_standard MCP tools.
type LaborPerformance struct {
	session *Session
}

// NewLaborPerformance builds a LaborPerformance client for the given
// connection config.
func NewLaborPerformance(cfg Config) *LaborPerformance {
	cfg.Name = "labor-performance"
	return &LaborPerformance{session: New(cfg)}
}

var _ ports.LaborPerformanceClient = (*LaborPerformance)(nil)

func (c *LaborPerformance) GetAssociateScorecard(ctx context.Context, associateId string) (ports.AssociateScorecard, error) {
	var out ports.AssociateScorecard
	err := c.session.callTool(ctx, "get_associate_scorecard", map[string]any{"associateId": associateId}, &out)
	return out, err
}

func (c *LaborPerformance) GetTaskTypePerformance(ctx context.Context, taskType string) (ports.TaskTypePerformance, error) {
	var out ports.TaskTypePerformance
	err := c.session.callTool(ctx, "get_task_type_performance", map[string]any{"taskType": taskType}, &out)
	return out, err
}

func (c *LaborPerformance) GetLaborStandard(ctx context.Context, taskType string) (ports.LaborStandard, error) {
	var out ports.LaborStandard
	err := c.session.callTool(ctx, "get_labor_standard", map[string]any{"taskType": taskType}, &out)
	return out, err
}

// GetTaskTypeUtilization calls get_task_type_utilization. windowSeconds is
// forwarded as-is: the tool itself treats a non-positive or omitted value
// as "apply its own default" (1h), so this client never substitutes a
// default of its own.
func (c *LaborPerformance) GetTaskTypeUtilization(ctx context.Context, taskType string, windowSeconds int64) (ports.TaskTypeUtilization, error) {
	var out ports.TaskTypeUtilization
	args := map[string]any{"taskType": taskType}
	if windowSeconds > 0 {
		args["windowSeconds"] = windowSeconds
	}
	err := c.session.callTool(ctx, "get_task_type_utilization", args, &out)
	return out, err
}
