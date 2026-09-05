// Package http is warehouse-ops-agent's inbound REST adapter: chi router,
// one handler for the daily brief, and DTOs. Domain/application structs
// never leak across this boundary.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/riandyrn/otelchi"
	otelchimetric "github.com/riandyrn/otelchi/metric"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// Handlers holds every use case the inbound HTTP adapter depends on.
type Handlers struct {
	DailyBrief *usecases.DailyBrief

	// FlowBalanceAdvisory is the E1 correlation use case (T2). Nil is a
	// valid value in tests that only exercise the daily-brief route;
	// getFlowBalanceException responds 503 rather than panicking when
	// nil (see its body).
	FlowBalanceAdvisory *usecases.FlowBalanceAdvisory

	// OrderLifecycle is the console-bff read model for the
	// warehouse-console shell's Order Lifecycle screen. Nil is a valid
	// value (same 503-not-panic convention as FlowBalanceAdvisory) for
	// any deployment that hasn't wired the four REST client upstreams.
	OrderLifecycle *usecases.OrderLifecycle

	// ConsoleReports is the console-bff's second capability: the WMS and
	// WES chart dashboards. Nil is a valid value (same 503-not-panic
	// convention as the fields above) for any deployment that hasn't
	// wired the seven analytics REST upstreams.
	ConsoleReports *usecases.ConsoleReports
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
	r.Use(corsMiddleware())

	r.Get("/healthz", healthz)
	r.Get("/daily-brief", h.getDailyBrief)
	r.Get("/flow-balance/{pathId}", h.getFlowBalanceException)
	r.Get("/console/orders/{id}/lifecycle", h.getOrderLifecycle)
	r.Get("/console/reports/wms", h.getWMSDashboard)
	r.Get("/console/reports/wes", h.getWESDashboard)

	return r
}

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// corsMiddleware allows the warehouse-console browser SPA to call this
// service's API (including the upcoming console-bff routes) directly from
// the browser. Static-bearer-key auth, not cookies, so credentials are
// never needed here. CORS_ALLOWED_ORIGINS overrides the local-dev default
// (comma-separated) for staging/prod deployments.
func corsMiddleware() func(http.Handler) http.Handler {
	origins := []string{"http://localhost:5173"}
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		origins = strings.Split(v, ",")
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	})
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

// --- console-bff: Order Lifecycle -------------------------------------

// getOrderLifecycle handles GET /console/orders/{id}/lifecycle. It never
// panics on an unwired OrderLifecycle (503, matching
// getFlowBalanceException's convention), and an order-management 404
// (the order genuinely doesn't exist) is reported as 404 rather than a
// generic 500 -- everything else, per usecases.OrderLifecycle.Execute's
// contract, degrades to a partial response rather than an error.
func (h *Handlers) getOrderLifecycle(w http.ResponseWriter, r *http.Request) {
	if h.OrderLifecycle == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "order lifecycle not configured"})
		return
	}
	orderId := chi.URLParam(r, "id")

	result, err := h.OrderLifecycle.Execute(r.Context(), orderId)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toOrderLifecycleDTO(result))
}

// orderLifecycleDTO's field names/shapes are hand-kept in sync with
// warehouse-console's src/features/order-lifecycle/types.ts OrderLifecycle
// interface -- the console is this endpoint's only consumer today.
type orderLifecycleDTO struct {
	OrderId         string               `json:"orderId"`
	OrderManagement *orderStageDTO       `json:"orderManagement"`
	Inventory       *inventoryStageDTO   `json:"inventory"`
	Planning        *planningStageDTO    `json:"planning"`
	Fulfillment     *fulfillmentStageDTO `json:"fulfillment"`
}

type orderStageDTO struct {
	Status               string         `json:"status"`
	AllowPartialShipment bool           `json:"allowPartialShipment"`
	PromiseDate          *string        `json:"promiseDate"`
	Lines                []orderLineDTO `json:"lines"`
	ReceivedAt           *string        `json:"receivedAt"`
}

type orderLineDTO struct {
	LineNo   int    `json:"lineNo"`
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
}

type inventoryStageDTO struct {
	Reservations []reservationLineDTO `json:"reservations"`
}

type reservationLineDTO struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
}

type planningStageDTO struct {
	WorkUnits []workUnitLineDTO `json:"workUnits"`
}

type workUnitLineDTO struct {
	WorkUnitId string `json:"workUnitId"`
	PathId     string `json:"pathId"`
	Status     string `json:"status"`
	Fragile    bool   `json:"fragile"`
	GiftWrap   bool   `json:"giftWrap"`
}

type fulfillmentStageDTO struct {
	Tasks         []taskLineDTO `json:"tasks"`
	PackageSealed bool          `json:"packageSealed"`
	LabelApplied  bool          `json:"labelApplied"`
}

type taskLineDTO struct {
	TaskId            string  `json:"taskId"`
	TaskType          string  `json:"taskType"`
	Status            string  `json:"status"`
	StationId         *string `json:"stationId"`
	WeightDiscrepancy bool    `json:"weightDiscrepancy"`
	LeaseExpiredCount int     `json:"leaseExpiredCount"`
}

func toOrderLifecycleDTO(r usecases.OrderLifecycleResult) orderLifecycleDTO {
	dto := orderLifecycleDTO{OrderId: r.OrderId}

	if r.OrderManagement != nil {
		lines := make([]orderLineDTO, 0, len(r.OrderManagement.Lines))
		for _, l := range r.OrderManagement.Lines {
			lines = append(lines, orderLineDTO{
				LineNo:   l.LineNo,
				SKU:      l.SKU,
				Quantity: l.Quantity,
				Status:   l.Status,
			})
		}
		dto.OrderManagement = &orderStageDTO{
			Status:               r.OrderManagement.Status,
			AllowPartialShipment: r.OrderManagement.AllowPartialShipment,
			PromiseDate:          r.OrderManagement.PromiseDate,
			Lines:                lines,
		}
	}

	if r.Inventory != nil {
		reservations := make([]reservationLineDTO, 0, len(r.Inventory))
		for _, res := range r.Inventory {
			reservations = append(reservations, reservationLineDTO{
				SKU:      res.SKU,
				Quantity: res.Quantity,
				Status:   res.Status,
			})
		}
		dto.Inventory = &inventoryStageDTO{Reservations: reservations}
	}

	if r.Planning != nil {
		units := make([]workUnitLineDTO, 0, len(r.Planning))
		for _, wu := range r.Planning {
			units = append(units, workUnitLineDTO{
				WorkUnitId: wu.Id,
				PathId:     wu.PathId,
				Status:     wu.State,
				Fragile:    false, // wes's WorkUnitDTO doesn't carry a fragile flag today; see PR notes.
				GiftWrap:   wu.GiftWrap,
			})
		}
		dto.Planning = &planningStageDTO{WorkUnits: units}
	}

	if r.Fulfillment != nil {
		tasks := make([]taskLineDTO, 0, len(r.Fulfillment))
		sealed := false
		for _, t := range r.Fulfillment {
			tasks = append(tasks, taskLineDTO{
				TaskId:    t.Id,
				TaskType:  t.Type,
				Status:    t.Status,
				StationId: t.LeaseStationId,
			})
			if t.Type == "SLAM" && t.Status == "COMPLETED" {
				sealed = true
			}
		}
		dto.Fulfillment = &fulfillmentStageDTO{
			Tasks:         tasks,
			PackageSealed: sealed,
			LabelApplied:  sealed,
		}
	}

	return dto
}

// --- console-bff: WMS / WES report dashboards -------------------------

// getWMSDashboard handles GET /console/reports/wms?from=&to=.
func (h *Handlers) getWMSDashboard(w http.ResponseWriter, r *http.Request) {
	h.serveDashboard(w, r, func(ctx context.Context, from, to time.Time) usecases.DashboardResult {
		return h.ConsoleReports.ExecuteWMS(ctx, from, to)
	})
}

// getWESDashboard handles GET /console/reports/wes?from=&to=.
func (h *Handlers) getWESDashboard(w http.ResponseWriter, r *http.Request) {
	h.serveDashboard(w, r, func(ctx context.Context, from, to time.Time) usecases.DashboardResult {
		return h.ConsoleReports.ExecuteWES(ctx, from, to)
	})
}

// serveDashboard is the shared request handling for both dashboards:
// the same optional-window parsing, the same 503-when-unwired
// convention, and the same envelope.
//
// Unlike the seven upstreams these fan out to -- where from and to are
// REQUIRED RFC3339 params -- both are OPTIONAL here, defaulting to a
// 24-hour trailing window ending now. This is a screen a human opens
// without necessarily knowing what window to ask for; making them state
// one before seeing anything would be a worse default than showing them
// the last day. A param that IS supplied must still be well-formed: a
// malformed timestamp is the caller's bug and gets a 400, never a
// silently substituted default that would answer a question nobody asked.
func (h *Handlers) serveDashboard(w http.ResponseWriter, r *http.Request, execute func(context.Context, time.Time, time.Time) usecases.DashboardResult) {
	if h.ConsoleReports == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "console reports not configured"})
		return
	}

	from, err := parseOptionalTime(r.URL.Query().Get("from"), "from")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"), "to")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	resolvedFrom, resolvedTo := h.ConsoleReports.ResolveWindow(from, to)
	if !resolvedTo.After(resolvedFrom) {
		// Rejected here rather than fanned out: an inverted window would
		// make every upstream either 400 or return nothing, degrading
		// all sections at once and reporting "the contexts are down"
		// when in fact the request was wrong.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'to' must be after 'from'"})
		return
	}

	result := execute(r.Context(), resolvedFrom, resolvedTo)
	writeJSON(w, http.StatusOK, toDashboardDTO(result))
}

// parseOptionalTime parses an RFC3339 timestamp, returning nil (not an
// error) when the param is absent so the use case can apply its default
// window.
func parseOptionalTime(raw, name string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("query parameter '%s' must be an RFC3339 timestamp", name)
	}
	return &t, nil
}

// dashboardDTO is the wire envelope both /console/reports/wms and
// /console/reports/wes return -- deliberately IDENTICAL in shape, so the
// console renders either dashboard with one component that switches only
// on each section's chartKind.
type dashboardDTO struct {
	From        string             `json:"from"`
	To          string             `json:"to"`
	GeneratedAt string             `json:"generatedAt"`
	Sections    []reportSectionDTO `json:"sections"`
}

// reportSectionDTO is one panel. error and freshnessLagSeconds are
// pointers so they serialise as explicit JSON null rather than being
// omitted: the console distinguishes "available, no staleness known"
// from "unavailable, here is why", and an absent key would collapse both
// into undefined.
type reportSectionDTO struct {
	Id                  string           `json:"id"`
	Title               string           `json:"title"`
	SourceContext       string           `json:"sourceContext"`
	ChartKind           string           `json:"chartKind"`
	Available           bool             `json:"available"`
	Error               *string          `json:"error"`
	FreshnessLagSeconds *float64         `json:"freshnessLagSeconds"`
	Series              []seriesPointDTO `json:"series"`
}

// seriesPointDTO's shape is the contract with warehouse-ui-kit: its
// FunnelChart, BarChart and LineChart all take a {label, value}[] `data`
// prop, so all three chartKinds share this one point type.
type seriesPointDTO struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

func toDashboardDTO(r usecases.DashboardResult) dashboardDTO {
	sections := make([]reportSectionDTO, 0, len(r.Sections))
	for _, s := range r.Sections {
		sections = append(sections, toReportSectionDTO(s))
	}
	return dashboardDTO{
		From:        r.From.UTC().Format(time.RFC3339),
		To:          r.To.UTC().Format(time.RFC3339),
		GeneratedAt: r.GeneratedAt.UTC().Format(time.RFC3339),
		Sections:    sections,
	}
}

func toReportSectionDTO(s usecases.ReportSection) reportSectionDTO {
	// Built with make(..., 0, n) so an empty or degraded section
	// marshals as [] rather than null -- a chart component binding to
	// null is a client-side crash, binding to [] is an honest empty
	// chart. Same rule labor-performance's own reports adapter applies.
	series := make([]seriesPointDTO, 0, len(s.Series))
	for _, p := range s.Series {
		series = append(series, seriesPointDTO{Label: p.Label, Value: p.Value})
	}

	dto := reportSectionDTO{
		Id:                  s.Id,
		Title:               s.Title,
		SourceContext:       s.SourceContext,
		ChartKind:           s.ChartKind,
		Available:           s.Available,
		FreshnessLagSeconds: s.FreshnessLagSeconds,
		Series:              series,
	}
	if s.Error != "" {
		errMsg := s.Error
		dto.Error = &errMsg
	}
	return dto
}
