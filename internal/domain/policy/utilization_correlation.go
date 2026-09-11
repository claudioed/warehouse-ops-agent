// Package policy: labor-utilization/idleness correlation overlay (E3.5,
// ADR 0008).
//
// This file adds a small, pure, side-effect-free correlation rule that
// reads a queue-depth reading (already gathered by FlowBalanceAdvisory as
// wes-work-planning's BacklogDepth) alongside labor-performance's observed
// task-type utilization, and derives one of three named advisory outcomes.
// It is deliberately ADDITIVE to the existing wes/wfm/fe-anchored Decide()
// rule above: it never changes RecommendedAction, ProposedHeads, or the
// existing Rationale text, and a nil result here (missing/null utilization
// data, or a queue-depth/idle-share combination that matches none of the
// three named shapes) leaves the deterministic Decision produced by
// Decide() completely unchanged, per ADR-0004's fallback discipline.
package policy

import "fmt"

// UtilizationCorrelationKind names which of the two "problem" outcomes
// fired, or that the healthy-utilization case corroborated an existing
// staffing-gap recommendation. There is deliberately no "none" member:
// the absence of a correlation is represented by a nil *UtilizationCorrelation,
// never by a zero-value/sentinel kind.
type UtilizationCorrelationKind string

const (
	// UtilizationCorrelationClaimFlowProblem fires when the queue is deep
	// AND measured utilization is low: work is available but associates
	// are measurably idle. This points at a claim/flow problem (stuck
	// tasks, lease churn) in fulfillment-execution, never at a staffing
	// recommendation -- assigning more heads would not fix a claim
	// problem.
	UtilizationCorrelationClaimFlowProblem UtilizationCorrelationKind = "claim_flow_problem"

	// UtilizationCorrelationStarvation fires when the queue is shallow
	// AND measured utilization is low: associates are idle for lack of
	// available work, not because of a claim/flow problem. The advisory
	// text recommends WES release pacing / upstream attention; this
	// agent has zero write capability (v1) and never calls a WES action
	// tool -- the non-integration with WES release pacing is deliberate
	// (see ADR 0008's Alternatives).
	UtilizationCorrelationStarvation UtilizationCorrelationKind = "starvation"

	// UtilizationCorrelationStaffingGapConfirmed fires when the queue is
	// deep AND measured utilization is healthy: the existing
	// staffing-gap recommendation (if Decide() already reached one) is
	// corroborated by observed utilization, not contradicted by it.
	UtilizationCorrelationStaffingGapConfirmed UtilizationCorrelationKind = "staffing_gap_confirmed"
)

const (
	// UtilizationQueueDepthHighThreshold is the BacklogDepth above which
	// a path's queue is considered "deep" for this correlation. Chosen
	// to match the same order of magnitude already used as a "worth
	// flagging" backlog depth elsewhere in this package's daily-brief
	// correlation fixtures (see ADR 0008's threshold rationale) --
	// documented here rather than derived from a live percentile, since
	// this repo has no telemetry-backed baseline for "normal" backlog
	// depth per path today.
	UtilizationQueueDepthHighThreshold = 50

	// UtilizationLowPctThreshold is the utilizationPct below which
	// observed utilization is considered "low" for this correlation.
	// Below 60% busy time, more than two-fifths of the window was idle
	// gap -- enough to be a genuine flow signal rather than ordinary
	// between-task noise (ADR 0008).
	UtilizationLowPctThreshold = 60.0

	// UtilizationHighIdleShareThreshold is the idleSeconds /
	// (taskSeconds + idleSeconds) share above which idle time is
	// considered "high" for this correlation. Set higher than the
	// inverse of UtilizationLowPctThreshold (40%) would strictly imply,
	// so BOTH the tool's own derived utilizationPct AND this
	// independently-computed idle share must agree before a "problem"
	// outcome fires -- a single noisy metric never triggers an advisory
	// alone (ADR 0008).
	UtilizationHighIdleShareThreshold = 0.40
)

// UtilizationCorrelation is one path's E3.5 advisory: which outcome fired
// and the human-readable rationale that produced it. Always accompanied by
// its Kind -- there is no "detected but unexplained" state.
type UtilizationCorrelation struct {
	Kind      UtilizationCorrelationKind
	Rationale string
}

// CorrelateUtilization derives the E3.5 advisory from a queue-depth
// reading and labor-performance's observed utilization for the same task
// type. It returns nil (no correlation to surface) when:
//   - util is nil (labor-performance unreachable, or no task type could be
//     resolved for the path), or
//   - util.UtilizationPct is nil (nothing was observed in the window --
//     NEVER treated as 0% utilization), or
//   - the queue-depth/idle-share combination does not clearly match one of
//     the three named shapes (e.g. a shallow queue with healthy
//     utilization is ordinary operating noise, not an exception).
//
// This function is pure and side-effect-free: no I/O, no upstream calls.
func CorrelateUtilization(queueDepth int, util *UtilizationSignal) *UtilizationCorrelation {
	if util == nil || util.UtilizationPct == nil {
		return nil
	}

	queueHigh := queueDepth > UtilizationQueueDepthHighThreshold
	lowUtil := *util.UtilizationPct < UtilizationLowPctThreshold
	highIdle := idleShareOf(util) > UtilizationHighIdleShareThreshold

	if !lowUtil {
		if queueHigh {
			return &UtilizationCorrelation{
				Kind: UtilizationCorrelationStaffingGapConfirmed,
				Rationale: fmt.Sprintf(
					"staffing gap confirmed by observed utilization: task type %s is running at %.1f%% utilization (idle share %.0f%%) over the last %ds, consistent with a genuine staffing gap rather than a flow/claim problem.",
					util.TaskType, *util.UtilizationPct, idleShareOf(util)*100, util.WindowSeconds,
				),
			}
		}
		// Shallow queue, healthy utilization: ordinary operating noise,
		// nothing to flag.
		return nil
	}

	if !highIdle {
		// utilizationPct alone reads low, but the independently-computed
		// idle share does not corroborate it strongly enough -- require
		// both metrics to agree before naming an outcome.
		return nil
	}

	if queueHigh {
		return &UtilizationCorrelation{
			Kind: UtilizationCorrelationClaimFlowProblem,
			Rationale: fmt.Sprintf(
				"queue depth %d is high but labor-performance measures only %.1f%% utilization (idle share %.0f%%) for task type %s over the last %ds -- work is available yet associates are measurably idle. This looks like a claim/flow problem (stuck tasks, lease churn), not a staffing gap; check fulfillment-execution's diagnose_stuck_tasks for this task type before assigning more labor.",
				queueDepth, *util.UtilizationPct, idleShareOf(util)*100, util.TaskType, util.WindowSeconds,
			),
		}
	}

	return &UtilizationCorrelation{
		Kind: UtilizationCorrelationStarvation,
		Rationale: fmt.Sprintf(
			"queue depth %d is low and labor-performance measures only %.1f%% utilization (idle share %.0f%%) for task type %s over the last %ds -- associates are idle for lack of available work, not a claim problem. Recommend WES release pacing / upstream attention for this path; this advisory does not trigger a WES action.",
			queueDepth, *util.UtilizationPct, idleShareOf(util)*100, util.TaskType, util.WindowSeconds,
		),
	}
}

// idleShareOf computes idleSeconds / (taskSeconds + idleSeconds), the
// share of measured time this task type's associates spent idle. Returns
// 0 when there is nothing to divide by, rather than dividing by zero --
// callers must still gate on UtilizationPct being non-nil before trusting
// this as a meaningful signal (see CorrelateUtilization).
func idleShareOf(u *UtilizationSignal) float64 {
	total := u.TaskSeconds + u.IdleSeconds
	if total <= 0 {
		return 0
	}
	return float64(u.IdleSeconds) / float64(total)
}
