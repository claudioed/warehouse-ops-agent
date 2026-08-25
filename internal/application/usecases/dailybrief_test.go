package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestDailyBrief_Execute_HealthyPath_NoExceptions(t *testing.T) {
	targets := []usecases.PathTarget{
		{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK", BuildingId: "wh1", ShiftId: "shift-1"},
	}
	facility := &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "Fulfilment Centre One"}}}}
	wes := &fakeWes{backlog: map[string]ports.BacklogTelemetry{
		"pick-zone-a": {PathId: "pick-zone-a", BacklogDepth: 4, WIP: 2, OverAlarmThreshold: false},
	}}
	fe := &fakeFe{
		queue: map[string]ports.QueueStatus{"PICK": {ProcessPath: "PICK", Depth: 3}},
		stuck: ports.StuckTasksResult{Count: 0},
	}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{
		"pick-zone-a": {PathId: "pick-zone-a", PlannedHeads: 3, ActiveHeads: 3, Understaffed: false},
	}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}
	brief := uc.Execute(context.Background())

	if len(brief.Sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(brief.Sites))
	}
	if got := brief.Sites[0].SiteName; got != "Fulfilment Centre One" {
		t.Errorf("site name = %q, want resolved from facility-layout list_sites", got)
	}
	if len(brief.Sites[0].Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(brief.Sites[0].Paths))
	}
	if len(brief.OpenExceptions) != 0 {
		t.Errorf("expected no open exceptions for a healthy path, got %d", len(brief.OpenExceptions))
	}
}

func TestDailyBrief_Execute_ImbalancedPath_ProducesRankedException(t *testing.T) {
	targets := []usecases.PathTarget{
		{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK", BuildingId: "wh1", ShiftId: "shift-1"},
	}
	facility := &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "Fulfilment Centre One"}}}}
	wes := &fakeWes{backlog: map[string]ports.BacklogTelemetry{
		"pick-zone-a": {PathId: "pick-zone-a", BacklogDepth: 50, WIP: 10, OverAlarmThreshold: true},
	}}
	fe := &fakeFe{
		queue: map[string]ports.QueueStatus{"PICK": {ProcessPath: "PICK", Depth: 40}},
		stuck: ports.StuckTasksResult{Count: 2, Tasks: []ports.StuckTask{
			{TaskId: "t1", Type: "PICK", Reason: "lease expired"},
			{TaskId: "t2", Type: "PICK", Reason: "lease expired"},
		}},
	}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{
		"pick-zone-a": {PathId: "pick-zone-a", PlannedHeads: 5, ActiveHeads: 2, Understaffed: true},
	}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}
	brief := uc.Execute(context.Background())

	if len(brief.OpenExceptions) != 1 {
		t.Fatalf("expected exactly 1 open exception (3 correlated signals), got %d: %+v", len(brief.OpenExceptions), brief.OpenExceptions)
	}
	exc := brief.OpenExceptions[0]
	if exc.Severity != policy.SeverityCritical {
		t.Errorf("severity = %q, want critical (3 signals)", exc.Severity)
	}
	if exc.PathId != "pick-zone-a" || exc.SiteCode != "WH1" {
		t.Errorf("exception identity = (%q,%q), want (WH1,pick-zone-a)", exc.SiteCode, exc.PathId)
	}
	if len(exc.Evidence) == 0 {
		t.Errorf("expected a non-empty evidence trail on the exception")
	}
}

func TestDailyBrief_Execute_PartialUpstreamFailure_DegradesGracefully(t *testing.T) {
	targets := []usecases.PathTarget{
		{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK", BuildingId: "wh1", ShiftId: "shift-1"},
	}
	facility := &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "Fulfilment Centre One"}}}}
	// wes is down for this path.
	wes := &fakeWes{err: map[string]error{"pick-zone-a": errors.New("connect: connection refused")}}
	fe := &fakeFe{
		queue: map[string]ports.QueueStatus{"PICK": {ProcessPath: "PICK", Depth: 3}},
		stuck: ports.StuckTasksResult{Count: 0},
	}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{
		"pick-zone-a": {PathId: "pick-zone-a", PlannedHeads: 3, ActiveHeads: 3, Understaffed: false},
	}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}

	// Execute must not panic despite the upstream failure.
	brief := uc.Execute(context.Background())

	if len(brief.Sites) != 1 || len(brief.Sites[0].Paths) != 1 {
		t.Fatalf("expected a partial brief to still be produced, got %+v", brief)
	}
	pb := brief.Sites[0].Paths[0]
	if pb.Backlog != nil {
		t.Errorf("expected nil backlog fact for an unavailable source, got %+v", pb.Backlog)
	}
	if len(pb.Unavailable) != 1 {
		t.Fatalf("expected exactly 1 unavailable-source note, got %d: %v", len(pb.Unavailable), pb.Unavailable)
	}
	if pb.Queue == nil || pb.Staffing == nil {
		t.Errorf("expected the OTHER sources' facts to still be present: queue=%+v staffing=%+v", pb.Queue, pb.Staffing)
	}
}

func TestDailyBrief_Execute_FacilityUnavailable_ReturnsEmptyNotPanic(t *testing.T) {
	targets := []usecases.PathTarget{
		{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"},
	}
	facility := &fakeFacility{err: errors.New("connect: connection refused")}
	wes := &fakeWes{backlog: map[string]ports.BacklogTelemetry{"pick-zone-a": {PathId: "pick-zone-a"}}}
	fe := &fakeFe{queue: map[string]ports.QueueStatus{"PICK": {}}, stuck: ports.StuckTasksResult{}}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{"pick-zone-a": {}}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}
	brief := uc.Execute(context.Background())

	// Site name resolution failed, but the path is still synthesized under
	// its configured SiteCode with an empty name rather than dropped.
	if len(brief.Sites) != 1 {
		t.Fatalf("expected 1 site (grouped by configured SiteCode) even with facility-layout down, got %d", len(brief.Sites))
	}
	if brief.Sites[0].SiteName != "" {
		t.Errorf("expected empty site name when list_sites failed, got %q", brief.Sites[0].SiteName)
	}
}

func TestDailyBrief_Execute_MultiplePaths_GroupedBySite(t *testing.T) {
	targets := []usecases.PathTarget{
		{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"},
		{SiteCode: "WH1", PathId: "pack-station-1", ProcessPath: "PACK"},
		{SiteCode: "WH2", PathId: "pick-zone-b", ProcessPath: "PICK"},
	}
	facility := &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{
		{Code: "WH1", Name: "One"}, {Code: "WH2", Name: "Two"},
	}}}
	wes := &fakeWes{backlog: map[string]ports.BacklogTelemetry{}}
	fe := &fakeFe{queue: map[string]ports.QueueStatus{}, stuck: ports.StuckTasksResult{}}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}
	brief := uc.Execute(context.Background())

	if len(brief.Sites) != 2 {
		t.Fatalf("expected 2 distinct sites, got %d", len(brief.Sites))
	}
	var wh1 *policy.SiteBrief
	for i := range brief.Sites {
		if brief.Sites[i].SiteCode == "WH1" {
			wh1 = &brief.Sites[i]
		}
	}
	if wh1 == nil {
		t.Fatalf("WH1 site not found")
	}
	if len(wh1.Paths) != 2 {
		t.Errorf("expected 2 paths grouped under WH1, got %d", len(wh1.Paths))
	}
}

func TestDailyBrief_Execute_NoTargets_EmptyBrief(t *testing.T) {
	uc := &usecases.DailyBrief{
		Facility: &fakeFacility{},
		Wes:      &fakeWes{},
		Fe:       &fakeFe{},
		Wfm:      &fakeWfm{},
		Now:      fixedClock(time.Unix(0, 0)),
	}
	brief := uc.Execute(context.Background())

	if len(brief.Sites) != 0 || len(brief.OpenExceptions) != 0 {
		t.Fatalf("expected an empty brief with no configured targets, got %+v", brief)
	}
	if brief.GeneratedAt.IsZero() {
		t.Errorf("expected GeneratedAt to be stamped even for an empty brief")
	}
}

func TestDailyBrief_Execute_NilClient_TreatedAsUnavailable(t *testing.T) {
	targets := []usecases.PathTarget{{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"}}
	uc := &usecases.DailyBrief{
		// Facility and Wfm deliberately nil: not every deployment wires
		// every upstream client (e.g. a phased rollout).
		Wes:     &fakeWes{backlog: map[string]ports.BacklogTelemetry{"pick-zone-a": {PathId: "pick-zone-a"}}},
		Fe:      &fakeFe{queue: map[string]ports.QueueStatus{"PICK": {}}, stuck: ports.StuckTasksResult{}},
		Targets: targets,
		Now:     fixedClock(time.Unix(0, 0)),
	}

	brief := uc.Execute(context.Background())
	if len(brief.Sites) != 1 || len(brief.Sites[0].Paths) != 1 {
		t.Fatalf("expected a degraded but non-panicking brief, got %+v", brief)
	}
	pb := brief.Sites[0].Paths[0]
	if pb.Staffing != nil {
		t.Errorf("expected nil staffing fact when Wfm client is nil, got %+v", pb.Staffing)
	}
	found := false
	for _, u := range pb.Unavailable {
		if u == "workforce-management: client not configured" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an explicit 'client not configured' note for the nil Wfm client, got %v", pb.Unavailable)
	}
}

func TestDailyBrief_Execute_StuckTaskCount_FilteredByProcessPath(t *testing.T) {
	// diagnose_stuck_tasks is queue-wide; the use case must filter its
	// flat task list down to the tasks matching THIS path's process-path
	// queue name before counting.
	targets := []usecases.PathTarget{{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"}}
	facility := &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "One"}}}}
	wes := &fakeWes{backlog: map[string]ports.BacklogTelemetry{"pick-zone-a": {}}}
	fe := &fakeFe{
		queue: map[string]ports.QueueStatus{"PICK": {}},
		stuck: ports.StuckTasksResult{Count: 3, Tasks: []ports.StuckTask{
			{TaskId: "t1", Type: "PICK"},
			{TaskId: "t2", Type: "PACK"}, // different queue: must not count against pick-zone-a.
			{TaskId: "t3", Type: "PICK"},
		}},
	}
	wfm := &fakeWfm{gaps: map[string]ports.StaffingGap{"pick-zone-a": {}}}

	uc := &usecases.DailyBrief{Facility: facility, Wes: wes, Fe: fe, Wfm: wfm, Targets: targets, Now: fixedClock(time.Unix(0, 0))}
	brief := uc.Execute(context.Background())

	pb := brief.Sites[0].Paths[0]
	if pb.Stuck == nil || pb.Stuck.Count != 2 {
		t.Fatalf("expected stuck count filtered to PICK-type tasks only (2), got %+v", pb.Stuck)
	}
}
