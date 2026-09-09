package ports

import "context"

// ArbitrationMetrics records, per use case, how the deterministic policy
// and the model-backed Reasoner were combined (ADR 0004). The shadow-mode
// rollout gate is the agreement ratio this feeds.
//
// agree is nil when no valid plan was available (off mode, transport
// error, invalid plan) -- callers must not coerce it to false.
type ArbitrationMetrics interface {
	RecordArbitration(ctx context.Context, useCase, mode, source string, agree *bool)
}
