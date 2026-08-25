// Package http is warehouse-ops-agent's inbound REST adapter: chi router,
// one handler for the daily brief, and DTOs. Domain/application structs
// never leak across this boundary.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	otelchimetric "github.com/riandyrn/otelchi/metric"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
)

// Handlers holds every use case the inbound HTTP adapter depends on.
type Handlers struct {
	DailyBrief *usecases.DailyBrief

	// FlowBalanceAdvisory is the E1 correlation use case (T2). Nil is a
	// valid value in tests that only exercise the daily-brief route;
	// getFlowBalanceException responds 503 rather than panicking when
	// nil (see its body).
	FlowBalanceAdvisory *usecases.FlowBalanceAdvisory
}

// NewRouter wires the daily-brief endpoint. serviceName names the server in
// the OTel span/metric attributes, mirroring the five sibling contexts'
// inbound/http.NewRouter convention.
func NewRouter(h *Handlers, serviceName string) *chi.Mux {
	r := chi.NewRouter()

	metricCfg := otelchimetric.NewBaseConfig(serviceName)

	r.Use(middleware.RequestID)
	r.Use(otelchi.Middleware(serviceName, otelchi.WithChiRoutes(r)))
	r.Use(otelchimetric.NewServerRequestDuration(metricCfg))
	r.Use(otelchimetric.NewServerActiveRequests(metricCfg))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthz)
	r.Get("/daily-brief", h.getDailyBrief)
	r.Get("/flow-balance/{pathId}", h.getFlowBalanceException)

	return r
}

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) getDailyBrief(w http.ResponseWriter, r *http.Request) {
	brief := h.DailyBrief.Execute(r.Context())
	writeJSON(w, http.StatusOK, toDailyBriefDTO(brief))
}

// getFlowBalanceException handles GET /flow-balance/{pathId}?buildingId=&shiftId=.
// buildingId and shiftId are required query params (the workforce-management
// staffing-gap lookup needs them); a missing FlowBalanceAdvisory (not wired
// by the composition root) responds 503 rather than a nil-pointer panic.
func (h *Handlers) getFlowBalanceException(w http.ResponseWriter, r *http.Request) {
	if h.FlowBalanceAdvisory == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "flow balance advisory not configured"})
		return
	}
	pathId := chi.URLParam(r, "pathId")
	buildingId := r.URL.Query().Get("buildingId")
	shiftId := r.URL.Query().Get("shiftId")

	decision, err := h.FlowBalanceAdvisory.Execute(r.Context(), buildingId, shiftId, pathId)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toFlowBalanceExceptionDTO(decision))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// --- DTOs ------------------------------------------------------------------

type dailyBriefDTO struct {
	GeneratedAt    time.Time          `json:"generatedAt"`
	Sites          []siteBriefDTO     `json:"sites"`
	OpenExceptions []openExceptionDTO `json:"openExceptions"`
}

type siteBriefDTO struct {
	SiteCode string         `json:"siteCode"`
	SiteName string         `json:"siteName"`
	Paths    []pathBriefDTO `json:"paths"`
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

type openExceptionDTO struct {
	Kind     string   `json:"kind"`
	SiteCode string   `json:"siteCode"`
	PathId   string   `json:"pathId"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence"`
}

type flowBalanceEvidenceDTO struct {
	Source string `json:"source"`
	Detail string `json:"detail"`
}

type flowBalanceExceptionDTO struct {
	PathId            string                   `json:"pathId"`
	RecommendedAction string                   `json:"recommendedAction"`
	ProposedHeads     int                      `json:"proposedHeads,omitempty"`
	Rationale         string                   `json:"rationale"`
	Partial           bool                     `json:"partial"`
	MissingSignals    []string                 `json:"missingSignals,omitempty"`
	Evidence          []flowBalanceEvidenceDTO `json:"evidence"`
}

func toFlowBalanceExceptionDTO(d policy.Decision) flowBalanceExceptionDTO {
	evidence := make([]flowBalanceEvidenceDTO, 0, len(d.Evidence))
	for _, e := range d.Evidence {
		evidence = append(evidence, flowBalanceEvidenceDTO{Source: e.Source, Detail: e.Detail})
	}
	return flowBalanceExceptionDTO{
		PathId:            d.PathId,
		RecommendedAction: string(d.RecommendedAction),
		ProposedHeads:     d.ProposedHeads,
		Rationale:         d.Rationale,
		Partial:           d.Partial,
		MissingSignals:    d.MissingSignals,
		Evidence:          evidence,
	}
}

func toDailyBriefDTO(b policy.DailyBrief) dailyBriefDTO {
	dto := dailyBriefDTO{
		GeneratedAt:    b.GeneratedAt,
		OpenExceptions: toOpenExceptionDTOs(b.OpenExceptions),
	}
	for _, s := range b.Sites {
		dto.Sites = append(dto.Sites, toSiteBriefDTO(s))
	}
	return dto
}

func toSiteBriefDTO(s policy.SiteBrief) siteBriefDTO {
	dto := siteBriefDTO{SiteCode: s.SiteCode, SiteName: s.SiteName}
	for _, p := range s.Paths {
		dto.Paths = append(dto.Paths, toPathBriefDTO(p))
	}
	return dto
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
