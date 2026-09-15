package ports

import "context"

// Reasoner is the driven port behind which a model-backed planner lives
// (ADR 0004). The application layer assembles a Brief from the same signals
// the deterministic policy already gathered, and the Reasoner returns a
// Plan -- a proposal in the policy package's own action vocabulary, never a
// free-form instruction. The policy layer remains the arbiter of whether
// that Plan is used (policy.Arbitrate).
//
// A Reasoner may consult the read tools listed in Brief.Tools through the
// ToolInvoker it was constructed with; it has no other actuator.
type Reasoner interface {
	Reason(ctx context.Context, brief Brief) (Plan, error)
}

// Brief is everything the Reasoner is allowed to know: the question, the
// facts already gathered (each with its source so provenance survives), and
// the catalogue of read tools it may call for more.
type Brief struct {
	// UseCase names the calling use case, e.g. "flow_balance_advisory".
	UseCase string
	// Question is the operator-level question in plain words.
	Question string
	// Facts are the already-gathered signals, JSON-encodable, keyed by
	// source (e.g. "wes-work-planning.get_rebalance_recommendation").
	Facts map[string]any
	// AllowedActions is the closed vocabulary the Plan may use.
	AllowedActions []string
	// Tools the Reasoner may invoke for more facts. Empty means none.
	Tools []ToolSpec
}

// ToolSpec describes one MCP read tool exposed to the model, 1:1 with the
// upstream context's own tool definition.
type ToolSpec struct {
	// Upstream is the mcpclient session name, e.g. "wes-work-planning".
	Upstream string
	Name     string
	// Description is the upstream tool's own description, verbatim.
	Description string
	// InputSchema is the upstream tool's JSON schema, verbatim.
	InputSchema map[string]any
}

// ToolInvoker executes one read tool on one upstream. The Anthropic
// adapter is handed an implementation backed by the existing mcpclient
// sessions; it never receives an HTTP client.
type ToolInvoker interface {
	Invoke(ctx context.Context, upstream, tool string, args map[string]any) (string, error)
}

// Plan is the Reasoner's proposal. Every field is validated by the policy
// package before it can influence a Decision.
type Plan struct {
	RecommendedAction string
	ProposedHeads     int
	Rationale         string
	// ToolCalls is the audit trail of what the model actually invoked.
	ToolCalls []ToolCall
	// Model identifies the backend that produced the plan, for logs/metrics.
	Model string
}

// ToolCall is one audited invocation the Reasoner made while planning.
type ToolCall struct {
	Upstream string
	Tool     string
	Args     map[string]any
	Outcome  string // "ok" or the error string
}
