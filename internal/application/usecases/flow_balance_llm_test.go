package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type fakeReasoner struct {
	plan  ports.Plan
	err   error
	brief ports.Brief
	calls int
}

func (f *fakeReasoner) Reason(_ context.Context, b ports.Brief) (ports.Plan, error) {
	f.calls++
	f.brief = b
	return f.plan, f.err
}

type fakeMetrics struct {
	useCase, mode, source string
	agree                 *bool
	calls                 int
}

func (m *fakeMetrics) RecordArbitration(_ context.Context, useCase, mode, source string, agree *bool) {
	m.calls++
	m.useCase, m.mode, m.source, m.agree = useCase, mode, source, agree
}

// healthy wires the three upstream fakes so the deterministic path yields
// assign_labor with 4 heads (same fixture as TestFlowBalanceAdvisory_Execute).
func healthy() (*fbFakeWes, *fbFakeWFM, *fbFakeFE) {
	return &fbFakeWes{recommendation: ports.RebalanceRecommendation{PathId: "pick", Action: "ReassignLabor", BacklogDepth: 120, WIP: 40}},
		&fbFakeWFM{gap: ports.StaffingGap{PathId: "pick", PlannedHeads: 10, ActiveHeads: 6, Understaffed: true}},
		&fbFakeFE{result: ports.StuckTasksResult{Count: 0}}
}

func TestFlowBalanceAdvisory_LLMArbitration(t *testing.T) {
	ctx := context.Background()

	t.Run("mode off never calls the reasoner", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{plan: ports.Plan{RecommendedAction: "hold", Rationale: "x"}}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r, LLMMode: policy.LLMOff}
		got, _ := uc.Execute(ctx, "b", "s", "pick")
		if r.calls != 0 || got.RecommendedAction != policy.ActionAssignLabor {
			t.Fatalf("calls=%d action=%s", r.calls, got.RecommendedAction)
		}
	})

	t.Run("zero-value mode with a reasoner wired is still off", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r}
		_, _ = uc.Execute(ctx, "b", "s", "pick")
		if r.calls != 0 {
			t.Fatal("zero LLMMode must behave as off")
		}
	})

	t.Run("shadow: reasoner consulted, deterministic returned, disagreement recorded", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{plan: ports.Plan{RecommendedAction: "hold", Rationale: "model says hold", Model: "m1"}}
		m := &fakeMetrics{}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r, LLMMode: policy.LLMShadow, Metrics: m,
			ReasonerTools: []ports.ToolSpec{{Upstream: "workforce-management", Name: "get_staffing_gap"}}}
		got, err := uc.Execute(ctx, "b", "s", "pick")
		if err != nil {
			t.Fatal(err)
		}
		if got.RecommendedAction != policy.ActionAssignLabor || got.ProposedHeads != 4 || got.Rationale == "model says hold" {
			t.Fatalf("shadow must return the deterministic decision, got %+v", got)
		}
		if r.calls != 1 {
			t.Fatalf("reasoner calls = %d", r.calls)
		}
		if m.calls != 1 || m.useCase != "flow_balance_advisory" || m.mode != "shadow" || m.source != "deterministic" || m.agree == nil || *m.agree {
			t.Fatalf("metrics %+v", m)
		}
		// The brief carries the vocabulary, the facts by source, and the tools.
		b := r.brief
		if b.UseCase != "flow_balance_advisory" || len(b.AllowedActions) != 3 || len(b.Tools) != 1 {
			t.Fatalf("brief %+v", b)
		}
		if _, ok := b.Facts["wes-work-planning.get_rebalance_recommendation"]; !ok {
			t.Fatalf("facts must be keyed by source: %v", b.Facts)
		}
	})

	t.Run("shadow: absent signals are stated as unavailable in the brief", func(t *testing.T) {
		wes, _, fe := healthy()
		r := &fakeReasoner{plan: ports.Plan{RecommendedAction: "hold", Rationale: "x"}}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: &fbFakeWFM{err: errors.New("down")}, FE: fe, Reasoner: r, LLMMode: policy.LLMShadow}
		_, _ = uc.Execute(ctx, "b", "s", "pick")
		if r.brief.Facts["workforce-management.get_staffing_gap"] != "unavailable" {
			t.Fatalf("facts %v", r.brief.Facts)
		}
	})

	t.Run("on: valid plan replaces action/heads/rationale, keeps evidence", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{plan: ports.Plan{RecommendedAction: "assign_labor", ProposedHeads: 2, Rationale: "two is enough"}}
		m := &fakeMetrics{}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r, LLMMode: policy.LLMOn, Metrics: m}
		got, _ := uc.Execute(ctx, "b", "s", "pick")
		if got.RecommendedAction != policy.ActionAssignLabor || got.ProposedHeads != 2 || got.Rationale != "two is enough" || len(got.Evidence) != 3 {
			t.Fatalf("got %+v", got)
		}
		if m.source != "llm" || m.agree == nil || !*m.agree {
			t.Fatalf("metrics %+v", m)
		}
	})

	t.Run("on: reasoner error falls back to deterministic", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{err: errors.New("anthropic: status 529")}
		m := &fakeMetrics{}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r, LLMMode: policy.LLMOn, Metrics: m}
		got, err := uc.Execute(ctx, "b", "s", "pick")
		if err != nil || got.RecommendedAction != policy.ActionAssignLabor || got.ProposedHeads != 4 {
			t.Fatalf("fallback must be the deterministic decision, got %+v err=%v", got, err)
		}
		if m.source != "fallback" || m.agree != nil {
			t.Fatalf("metrics %+v", m)
		}
	})

	t.Run("on: out-of-vocabulary plan falls back", func(t *testing.T) {
		wes, wfm, fe := healthy()
		r := &fakeReasoner{plan: ports.Plan{RecommendedAction: "shut_down_the_warehouse", Rationale: "x"}}
		m := &fakeMetrics{}
		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe, Reasoner: r, LLMMode: policy.LLMOn, Metrics: m}
		got, _ := uc.Execute(ctx, "b", "s", "pick")
		if got.RecommendedAction != policy.ActionAssignLabor || m.source != "fallback" {
			t.Fatalf("got %+v metrics %+v", got, m)
		}
	})
}
