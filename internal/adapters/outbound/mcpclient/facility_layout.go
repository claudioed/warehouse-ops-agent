package mcpclient

import (
	"context"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// FacilityLayout implements ports.FacilityLayoutClient by calling
// facility-layout's published list_sites, get_site_layout, and
// get_zone_grid MCP tools.
type FacilityLayout struct {
	session *Session
}

// NewFacilityLayout builds a FacilityLayout client for the given connection
// config.
func NewFacilityLayout(cfg Config) *FacilityLayout {
	cfg.Name = "facility-layout"
	return &FacilityLayout{session: New(cfg)}
}

var _ ports.FacilityLayoutClient = (*FacilityLayout)(nil)

func (c *FacilityLayout) ListSites(ctx context.Context) (ports.SitesResult, error) {
	var out ports.SitesResult
	err := c.session.callTool(ctx, "list_sites", map[string]any{}, &out)
	return out, err
}

func (c *FacilityLayout) GetSiteLayout(ctx context.Context, siteCode string) (ports.SiteLayout, error) {
	var out ports.SiteLayout
	err := c.session.callTool(ctx, "get_site_layout", map[string]any{"siteCode": siteCode}, &out)
	return out, err
}

func (c *FacilityLayout) GetZoneGrid(ctx context.Context, zoneId string) (ports.ZoneGrid, error) {
	var out ports.ZoneGrid
	err := c.session.callTool(ctx, "get_zone_grid", map[string]any{"zoneId": zoneId}, &out)
	return out, err
}
