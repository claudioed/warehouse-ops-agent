package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// ProcessPathManagement implements ports.ProcessPathManagementClient by
// calling process-path-management's published get_process_path and
// list_process_paths MCP tools.
type ProcessPathManagement struct {
	session *Session
}

// NewProcessPathManagement builds a ProcessPathManagement client for the
// given connection config.
func NewProcessPathManagement(cfg Config) *ProcessPathManagement {
	cfg.Name = "process-path-management"
	return &ProcessPathManagement{session: New(cfg)}
}

var _ ports.ProcessPathManagementClient = (*ProcessPathManagement)(nil)

func (c *ProcessPathManagement) GetProcessPath(ctx context.Context, pathId string) (ports.ProcessPath, error) {
	var out ports.ProcessPath
	err := c.session.callTool(ctx, "get_process_path", map[string]any{"pathId": pathId}, &out)
	return out, err
}

func (c *ProcessPathManagement) ListProcessPaths(ctx context.Context, activeOnly bool) (ports.ProcessPathList, error) {
	var out ports.ProcessPathList
	err := c.session.callTool(ctx, "list_process_paths", map[string]any{"activeOnly": activeOnly}, &out)
	return out, err
}
