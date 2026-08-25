package policy

import (
	"testing"
)

func TestSynthesizePathBrief_NoSignals_NoException(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}
	backlog := &BacklogFact{BacklogDepth: 5, WIP: 2, OverAlarmThreshold: false}
	staffing := &StaffingFact{PlannedHeads: 3, ActiveHeads: 3, Understaffed: false}
	stuck := &StuckTasksFact{Count: 0}

	brief := SynthesizePathBrief(target, backlog, staffing, &QueueFact{Depth: 4}, stuck, nil)

	if len(brief.Exceptions) != 0 {
		t.Fatalf("expected no exceptions, got %d: %+v", len(brief.Exceptions), brief.Exceptions)
	}
}

func TestSynthesizePathBrief_OneSignal_NoException(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}
	backlog := &BacklogFact{BacklogDepth: 50, WIP: 2, OverAlarmThreshold: true}
	staffing := &StaffingFact{PlannedHeads: 3, ActiveHeads: 3, Understaffed: false}
	stuck := &StuckTasksFact{Count: 0}

	brief := SynthesizePathBrief(target, backlog, staffing, nil, stuck, nil)

	if len(brief.Exceptions) != 0 {
		t.Fatalf("a single signal must not trip an exception, got %d: %+v", len(brief.Exceptions), brief.Exceptions)
	}
}

func TestSynthesizePathBrief_TwoSignals_WarningException(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}
	backlog := &BacklogFact{BacklogDepth: 50, WIP: 10, OverAlarmThreshold: true}
	staffing := &StaffingFact{PlannedHeads: 5, ActiveHeads: 2, Understaffed: true}
	stuck := &StuckTasksFact{Count: 0}

	brief := SynthesizePathBrief(target, backlog, staffing, nil, stuck, nil)

	if len(brief.Exceptions) != 1 {
		t.Fatalf("expected exactly 1 exception, got %d", len(brief.Exceptions))
	}
	exc := brief.Exceptions[0]
	if exc.Kind != ExceptionFlowBalanceRisk {
		t.Errorf("kind = %q, want %q", exc.Kind, ExceptionFlowBalanceRisk)
	}
	if exc.Severity != SeverityWarning {
		t.Errorf("severity = %q, want %q (2 signals)", exc.Severity, SeverityWarning)
	}
	if exc.SiteCode != "WH1" || exc.PathId != "pick-zone-a" {
		t.Errorf("exception identity = (%q,%q), want (WH1,pick-zone-a)", exc.SiteCode, exc.PathId)
	}
	if len(exc.Evidence) != 2 {
		t.Errorf("evidence trail length = %d, want 2 (one per firing signal)", len(exc.Evidence))
	}
}

func TestSynthesizePathBrief_ThreeSignals_CriticalException(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}
	backlog := &BacklogFact{BacklogDepth: 80, WIP: 10, OverAlarmThreshold: true}
	staffing := &StaffingFact{PlannedHeads: 5, ActiveHeads: 1, Understaffed: true}
	stuck := &StuckTasksFact{Count: 3}

	brief := SynthesizePathBrief(target, backlog, staffing, nil, stuck, nil)

	if len(brief.Exceptions) != 1 {
		t.Fatalf("expected exactly 1 exception, got %d", len(brief.Exceptions))
	}
	if got := brief.Exceptions[0].Severity; got != SeverityCritical {
		t.Errorf("severity = %q, want %q (3 signals)", got, SeverityCritical)
	}
	if len(brief.Exceptions[0].Evidence) != 3 {
		t.Errorf("evidence trail length = %d, want 3", len(brief.Exceptions[0].Evidence))
	}
}

func TestSynthesizePathBrief_NilFacts_NoPanic(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}

	brief := SynthesizePathBrief(target, nil, nil, nil, nil, []string{"wes-work-planning: unavailable", "workforce-management: unavailable"})

	if len(brief.Exceptions) != 0 {
		t.Fatalf("expected no exceptions when every fact is nil, got %d", len(brief.Exceptions))
	}
	if len(brief.Unavailable) != 2 {
		t.Fatalf("expected 2 unavailable sources recorded, got %d", len(brief.Unavailable))
	}
}

func TestSynthesizePathBrief_QueueFactAlonePreserved(t *testing.T) {
	target := PathTarget{SiteCode: "WH1", PathId: "pick-zone-a"}
	queue := &QueueFact{Depth: 12}

	brief := SynthesizePathBrief(target, nil, nil, queue, nil, nil)

	if brief.Queue == nil || brief.Queue.Depth != 12 {
		t.Fatalf("expected queue fact to be preserved on the brief, got %+v", brief.Queue)
	}
}
