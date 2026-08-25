// Package mcp is warehouse-ops-agent's inbound Model Context Protocol
// adapter: it exposes this agent's daily-brief read model to the AI
// ecosystem as a second driving adapter over the same application-layer
// use case the HTTP adapter uses. It is built on the official MCP Go SDK
// and served over Streamable HTTP, reusing the exact 6-file adapter
// pattern proven on fulfillment-execution (the mcp-go-inbound-adapter
// skill/template).
//
// Per ADR-0008 and the MCP governance charter, this package depends
// inward on the application layer (use cases) and the domain (policy)
// only — never on an outbound adapter. The composition root (cmd/agent)
// wires the concrete outbound MCP clients into the use case. Tool handlers
// call the use case; domain structs never leak across the tool boundary —
// only the DTOs in mapping.go do.
package mcp

import (
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
)

// mapping.go holds the tool-boundary DTOs mirroring
// internal/adapters/inbound/http's DTOs, kept as a separate, independent
// mapping rather than shared types so the HTTP and MCP surfaces can evolve
// independently without coupling one inbound adapter's shape to the
// other's.

type backlogFactDTO struct {
	BacklogDepth       int  `json:"backlogDepth"`
	WIP                int  `json:"wip"`
	OverAlarmThreshold bool `json:"overAlarmThreshold"`
}

type staffingFactDTO struct {
	PlannedHeads int  `json:"plannedHeads"`
	ActiveHeads  int  `json:"activeHeads"`
	Understaffed bool `json:"understaffed"`
}

type queueFactDTO struct {
	Depth int `json:"depth"`
}

type stuckTasksFactDTO struct {
	Count int `json:"count"`
}

type pathBriefDTO struct {
	PathId      string             `json:"pathId"`
	ProcessPath string             `json:"processPath"`
	Backlog     *backlogFactDTO    `json:"backlog,omitempty"`
	Staffing    *staffingFactDTO   `json:"staffing,omitempty"`
	Queue       *queueFactDTO      `json:"queue,omitempty"`
	Stuck       *stuckTasksFactDTO `json:"stuck,omitempty"`
	Unavailable []string           `json:"unavailable,omitempty"`
	Exceptions  []openExceptionDTO `json:"exceptions,omitempty"`
}

type siteBriefDTO struct {
	SiteCode string         `json:"siteCode"`
	SiteName string         `json:"siteName"`
	Paths    []pathBriefDTO `json:"paths"`
}

type openExceptionDTO struct {
	Kind     string   `json:"kind"`
	SiteCode string   `json:"siteCode"`
	PathId   string   `json:"pathId"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence"`
}

func toSiteBriefDTOs(sites []policy.SiteBrief) []siteBriefDTO {
	out := make([]siteBriefDTO, 0, len(sites))
	for _, s := range sites {
		dto := siteBriefDTO{SiteCode: s.SiteCode, SiteName: s.SiteName}
		for _, p := range s.Paths {
			dto.Paths = append(dto.Paths, toPathBriefDTO(p))
		}
		out = append(out, dto)
	}
	return out
}

func toPathBriefDTO(p policy.PathBrief) pathBriefDTO {
	dto := pathBriefDTO{
		PathId:      p.Target.PathId,
		ProcessPath: p.Target.ProcessPath,
		Unavailable: p.Unavailable,
		Exceptions:  toOpenExceptionDTOs(p.Exceptions),
	}
	if p.Backlog != nil {
		dto.Backlog = &backlogFactDTO{
			BacklogDepth:       p.Backlog.BacklogDepth,
			WIP:                p.Backlog.WIP,
			OverAlarmThreshold: p.Backlog.OverAlarmThreshold,
		}
	}
	if p.Staffing != nil {
		dto.Staffing = &staffingFactDTO{
			PlannedHeads: p.Staffing.PlannedHeads,
			ActiveHeads:  p.Staffing.ActiveHeads,
			Understaffed: p.Staffing.Understaffed,
		}
	}
	if p.Queue != nil {
		dto.Queue = &queueFactDTO{Depth: p.Queue.Depth}
	}
	if p.Stuck != nil {
		dto.Stuck = &stuckTasksFactDTO{Count: p.Stuck.Count}
	}
	return dto
}

func toOpenExceptionDTOs(exceptions []policy.OpenException) []openExceptionDTO {
	out := make([]openExceptionDTO, 0, len(exceptions))
	for _, e := range exceptions {
		out = append(out, openExceptionDTO{
			Kind:     string(e.Kind),
			SiteCode: e.SiteCode,
			PathId:   e.PathId,
			Severity: string(e.Severity),
			Summary:  e.Summary,
			Evidence: e.Evidence,
		})
	}
	return out
}
