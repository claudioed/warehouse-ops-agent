// Package policy: travel-factor advisory correlation (ADR 0009).
//
// This file adds a small, pure, side-effect-free correlation rule that
// takes a real measured (or graph-estimated) travel distance between two
// facility-layout location codes — gathered by a NEW, separate use case
// (ExplainTravelFactor), never invented or resolved by this package — and
// classifies it against a documented distance threshold into a
// human-readable advisory. It never changes FlowBalanceAdvisory's
// existing Decision.RecommendedAction/ProposedHeads/Rationale: this is a
// deliberately separate, additive correlation, mirroring
// CorrelateUtilization's own precedent (ADR 0008) of keeping a new
// diagnostic overlay in its own file with its own named outcome type,
// rather than folding it into Decide().
package policy

import "fmt"

// TravelFactorOutcomeKind names which of the two named outcomes fired.
// There is deliberately no "none" member: the absence of a correlation is
// represented by a nil *TravelFactorCorrelation, never a zero-value/
// sentinel kind — the same discipline UtilizationCorrelationKind already
// established.
type TravelFactorOutcomeKind string

const (
	// TravelFactorOutcomeSignificant fires when the estimated/measured
	// travel distance between the two supplied locations exceeds
	// TravelFactorDistanceThresholdMetres: travel time is a plausible,
	// evidence-backed explanation for a slow path, worth surfacing
	// alongside (never instead of) FlowBalanceAdvisory's own
	// stuck-task/staffing/backlog evidence.
	TravelFactorOutcomeSignificant TravelFactorOutcomeKind = "travel_significant"

	// TravelFactorOutcomeNegligible fires when the distance is at or
	// below the threshold: travel time is unlikely to be a material
	// contributor to whatever slowness prompted the check, so the
	// caller should keep looking at the other flow-balance signals
	// rather than treating travel as the explanation.
	TravelFactorOutcomeNegligible TravelFactorOutcomeKind = "travel_negligible"
)

// TravelFactorDistanceThresholdMetres is the estimated/measured route
// length above which travel time is considered a materially significant
// contributor to a slow path. Documented here, not derived from a live
// percentile — this repo has no telemetry-backed per-path baseline for
// "normal" travel distance today, the same posture ADR 0008 already
// accepted for its own thresholds. 60 metres is roughly 15-20 aisle-widths
// of pure walking at a typical warehouse pace (~1.2 m/s, unloaded) —
// enough to plausibly account for tens of seconds of a task's measured
// duration, not just ordinary in-aisle repositioning.
const TravelFactorDistanceThresholdMetres = 60.0

// TravelDistanceReading is the domain-owned mirror of the reading
// gathered from facility-layout's estimate_travel_distance tool. Source
// identifies which tool call produced it, for the correlation's
// rationale; From/To are the two location codes supplied by the caller —
// this package never resolves, infers, or validates them itself (see
// ExplainTravelFactor, the application-layer use case that gathers this
// reading).
type TravelDistanceReading struct {
	Source    string
	From      string
	To        string
	MetresM   float64
	Estimated bool
}

// TravelFactorCorrelation is the ExplainTravelFactor advisory: which
// outcome fired and the human-readable rationale that produced it.
// Always accompanied by its Kind — there is no "detected but unexplained"
// state, mirroring UtilizationCorrelation's own shape.
type TravelFactorCorrelation struct {
	Kind      TravelFactorOutcomeKind
	Rationale string
}

// CorrelateTravelFactor classifies a facility-layout travel-distance
// reading against TravelFactorDistanceThresholdMetres. It returns nil
// only when reading is nil (the upstream call failed, or the use case was
// never given both location codes) — unlike CorrelateUtilization, there
// is no "nothing observed" case to guard for here: a resolved distance
// reading always classifies into exactly one of the two named outcomes,
// since a route length is never itself a nullable business fact the way
// a utilization percentage is.
//
// This function is pure and side-effect-free: no I/O, no upstream calls.
func CorrelateTravelFactor(reading *TravelDistanceReading) *TravelFactorCorrelation {
	if reading == nil {
		return nil
	}

	estimatedNote := ""
	if reading.Estimated {
		estimatedNote = " (estimated from the travel graph, not a measured route)"
	}

	if reading.MetresM > TravelFactorDistanceThresholdMetres {
		return &TravelFactorCorrelation{
			Kind: TravelFactorOutcomeSignificant,
			Rationale: fmt.Sprintf(
				"facility-layout estimates %.1fm of travel from %s to %s%s, above the %.0fm threshold — travel time is a plausible, evidence-backed contributor to this path's slowness, worth weighing alongside any staffing/claim-flow evidence rather than treating this task type's engineered standard as travel-free.",
				reading.MetresM, reading.From, reading.To, estimatedNote, TravelFactorDistanceThresholdMetres,
			),
		}
	}

	return &TravelFactorCorrelation{
		Kind: TravelFactorOutcomeNegligible,
		Rationale: fmt.Sprintf(
			"facility-layout estimates only %.1fm of travel from %s to %s%s, at or below the %.0fm threshold — travel time is unlikely to materially explain this path's slowness; look to staffing or claim-flow evidence instead.",
			reading.MetresM, reading.From, reading.To, estimatedNote, TravelFactorDistanceThresholdMetres,
		),
	}
}
