package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
)

// tracerName is the OTel instrumentation scope for MCP tool spans.
const tracerName = "github.com/claudioed/warehouse-ops-agent/internal/adapters/inbound/mcp"

// Deps is everything the MCP tools need, injected by the composition root.
// It carries the same use case the HTTP adapter uses; the adapter never
// constructs an outbound adapter itself.
type Deps struct {
	// DailyBrief is the existing E3 read-model use case, reused unchanged.
	DailyBrief *usecases.DailyBrief
}

// --- get_daily_brief -----------------------------------------------------

type dailyBriefInput struct{}

type dailyBriefOutput struct {
	Sites          []siteBriefDTO     `json:"sites"`
	OpenExceptions []openExceptionDTO `json:"openExceptions"`
}

func (d Deps) getDailyBrief(ctx context.Context, _ dailyBriefInput) (dailyBriefOutput, error) {
	brief := d.DailyBrief.Execute(ctx)
	return dailyBriefOutput{
		Sites:          toSiteBriefDTOs(brief.Sites),
		OpenExceptions: toOpenExceptionDTOs(brief.OpenExceptions),
	}, nil
}

// --- list_open_exceptions -------------------------------------------------

// listOpenExceptionsInput's Severity, when non-empty, filters the result to
// exceptions at or above that minimum severity. Untrusted model input: an
// unknown value is rejected, never silently defaulted to "return
// everything" or "return nothing".
type listOpenExceptionsInput struct {
	Severity string `json:"severity,omitempty" jsonschema:"optional minimum severity filter: info, warning, or critical; omit to return every open exception"`
}

type listOpenExceptionsOutput struct {
	Count      int                `json:"count"`
	Exceptions []openExceptionDTO `json:"exceptions"`
}

// severityRank orders severities worst-first so a minimum-severity filter
// can be expressed as "rank <= requested rank".
func severityRank(s string) (int, bool) {
	switch s {
	case "critical":
		return 0, true
	case "warning":
		return 1, true
	case "info":
		return 2, true
	default:
		return 0, false
	}
}

func (d Deps) listOpenExceptions(ctx context.Context, in listOpenExceptionsInput) (listOpenExceptionsOutput, error) {
	minRank := 2 // default: include info and above, i.e. everything.
	if in.Severity != "" {
		r, ok := severityRank(in.Severity)
		if !ok {
			return listOpenExceptionsOutput{}, invalidSeverityErr(in.Severity)
		}
		minRank = r
	}

	brief := d.DailyBrief.Execute(ctx)
	all := toOpenExceptionDTOs(brief.OpenExceptions)

	filtered := make([]openExceptionDTO, 0, len(all))
	for _, e := range all {
		if r, ok := severityRank(e.Severity); ok && r <= minRank {
			filtered = append(filtered, e)
		}
	}
	return listOpenExceptionsOutput{Count: len(filtered), Exceptions: filtered}, nil
}

// --- registration -----------------------------------------------------------

// registerTools adds every tool to the server, each wrapped so its handler
// runs inside an OTel span named "mcp.tool <name>" and is gated by the
// session's scope. Both tools are read-only and require ScopeRead — this
// agent has no write tool at all.
func (d Deps) registerTools(server *mcp.Server, scopeOf func(context.Context) Scope) {
	readOnly := true

	addTool(server, scopeOf, ScopeRead, &mcp.Tool{
		Name:        "get_daily_brief",
		Description: "Return the full synthesized daily operational brief: every monitored site's paths with their backlog, staffing, queue, and stuck-task facts, plus the correlated open exceptions across all paths, ranked critical-first.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly},
	}, d.getDailyBrief)

	addTool(server, scopeOf, ScopeRead, &mcp.Tool{
		Name:        "list_open_exceptions",
		Description: "List the daily brief's correlated open exceptions, optionally filtered to a minimum severity (info, warning, or critical). Each exception carries its full evidence trail.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly},
	}, d.listOpenExceptions)
}

// addTool registers one scope-gated tool. It centralises the cross-cutting
// concerns every tool shares: a span per call, scope enforcement against
// the tool's required minimum scope, and mapping a handler error onto the
// span before returning it.
func addTool[In, Out any](
	server *mcp.Server,
	scopeOf func(context.Context) Scope,
	required Scope,
	tool *mcp.Tool,
	handle func(context.Context, In) (Out, error),
) {
	mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		ctx, span := otel.Tracer(tracerName).Start(ctx, "mcp.tool "+tool.Name,
			trace.WithAttributes(
				attribute.String("mcp.tool.name", tool.Name),
				attribute.String("mcp.tool.required_scope", string(required)),
			),
		)
		defer span.End()

		if !scopeAllows(scopeOf(ctx), required) {
			err := unauthorizedErr(tool.Name, required)
			span.SetStatus(codes.Error, "unauthorized")
			return nil, zero, err
		}

		out, err := handle(ctx, in)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, zero, err
		}
		return nil, out, nil
	})
}
