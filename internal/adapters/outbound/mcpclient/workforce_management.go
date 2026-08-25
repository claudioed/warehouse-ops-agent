package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// WorkforceManagement implements ports.WorkforceManagementClient by calling
// workforce-management's published get_staffing_gap and propose_path_heads
// MCP tools.
type WorkforceManagement struct {
	session *Session
}

// NewWorkforceManagement builds a WorkforceManagement client for the given
// connection config.
func NewWorkforceManagement(cfg Config) *WorkforceManagement {
	cfg.Name = "workforce-management"
	return &WorkforceManagement{session: New(cfg)}
}

var _ ports.WorkforceManagementClient = (*WorkforceManagement)(nil)

func (c *WorkforceManagement) GetStaffingGap(ctx context.Context, buildingId, shiftId, pathId string) (ports.StaffingGap, error) {
	var out ports.StaffingGap
	err := c.session.callTool(ctx, "get_staffing_gap", map[string]any{
		"buildingId": buildingId,
		"shiftId":    shiftId,
		"pathId":     pathId,
	}, &out)
	return out, err
}

func (c *WorkforceManagement) ProposePathHeads(ctx context.Context, buildingId, pathId string, charge, plannedRate float64) (ports.ProposedHeads, error) {
	var out ports.ProposedHeads
	err := c.session.callTool(ctx, "propose_path_heads", map[string]any{
		"buildingId":  buildingId,
		"pathId":      pathId,
		"charge":      charge,
		"plannedRate": plannedRate,
	}, &out)
	return out, err
}
