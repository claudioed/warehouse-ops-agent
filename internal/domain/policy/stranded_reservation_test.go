package policy

import "testing"

func TestEvaluate_RejectsUnknownTaskType(t *testing.T) {
	_, err := Evaluate(Inputs{
		TaskType:            TaskType("UNKNOWN"),
		StuckTasksAvailable: true,
	})
	if err == nil {
		t.Fatal("expected an error for an unknown task type, got nil")
	}
}

func TestEvaluate_DegradesWhenStuckTasksUnavailable(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: false,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Detected {
		t.Fatal("expected Detected=false when stuck-task signal is unavailable")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q", got.Action, ActionHold)
	}
	if len(got.Evidence) != 1 || got.Evidence[0].Tool != "fulfillment-execution.diagnose_stuck_tasks" {
		t.Fatalf("expected a single evidence entry citing diagnose_stuck_tasks, got %+v", got.Evidence)
	}
}

func TestEvaluate_HoldsWhenNoExpiredLeasesForTaskType(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePack, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 0},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Detected {
		t.Fatal("expected Detected=false when no expired leases match the task type")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q", got.Action, ActionHold)
	}
}

func TestEvaluate_DegradesWhenAvailabilityUnavailable(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePick, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       nil,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Detected {
		t.Fatal("expected Detected=false when availability signal is unavailable")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q", got.Action, ActionHold)
	}
}

func TestEvaluate_HoldsWhenUsableStockAboveThreshold(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePick, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 50},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Detected {
		t.Fatal("expected Detected=false when usable stock is comfortably above threshold")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q", got.Action, ActionHold)
	}
}

func TestEvaluate_HoldsWhenReservationIdMissingOnCorrelatedShortfall(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePick, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 0},
		ReservationId:      "",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !got.Detected {
		t.Fatal("expected Detected=true: expired leases correlate with a confirmed shortfall")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q (no candidate reservation must never yield a write recommendation)", got.Action, ActionHold)
	}
	if got.BlastRadius != nil {
		t.Fatal("expected nil BlastRadius when no reservation candidate was supplied")
	}
}

func TestEvaluate_DetectsButHoldsWhenBlastRadiusUnavailable(t *testing.T) {
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePick, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 0},
		ReservationId:      "R-1",
		Bin:                nil,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !got.Detected {
		t.Fatal("expected Detected=true: expired leases correlate with a confirmed shortfall")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q (blast radius unavailable must never yield a write recommendation)", got.Action, ActionHold)
	}
	if got.BlastRadius != nil {
		t.Fatal("expected nil BlastRadius when it could not be read")
	}
}

func TestEvaluate_RecommendsRevokeWithBlastRadiusWhenFullyCorrelated(t *testing.T) {
	bin := &BlastRadius{
		SKU:           "SKU-1",
		BinId:         "BIN-A1",
		ReservationId: "R-1",
		QuantityFreed: 12,
		BinLines: []BinLine{
			{StockUnitId: "SU-1", SKU: "SKU-1", Reserved: 12, Usable: 0, State: "RESERVED"},
		},
	}

	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePick, LeaseStationId: "ST-1", Reason: "lease already expired"},
			{TaskId: "t-2", Type: TaskTypePack, LeaseStationId: "ST-2", Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 2},
		ReservationId:      "R-1",
		Bin:                bin,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !got.Detected {
		t.Fatal("expected Detected=true")
	}
	if got.Action != ActionRevokeReservation {
		t.Fatalf("Action = %q, want %q", got.Action, ActionRevokeReservation)
	}
	if got.ReservationId != "R-1" {
		t.Fatalf("ReservationId = %q, want %q", got.ReservationId, "R-1")
	}
	if got.BlastRadius == nil || got.BlastRadius.BinId != "BIN-A1" || got.BlastRadius.QuantityFreed != 12 {
		t.Fatalf("BlastRadius = %+v, want bin BIN-A1 freeing 12 units", got.BlastRadius)
	}
	if len(got.Evidence) != 3 {
		t.Fatalf("Evidence = %+v, want 3 entries (stuck tasks, availability, bin occupancy)", got.Evidence)
	}
	if got.Rationale == "" {
		t.Fatal("expected a non-empty rationale")
	}
}

func TestEvaluate_IgnoresUnrelatedTaskTypesInCorrelation(t *testing.T) {
	// Expired PACK leases must never trigger a PICK-scoped recommendation.
	got, err := Evaluate(Inputs{
		TaskType:            TaskTypePick,
		StuckTasksAvailable: true,
		StuckTasks: []StuckTaskSignal{
			{TaskId: "t-1", Type: TaskTypePack, Reason: "lease already expired"},
			{TaskId: "t-2", Type: TaskTypeSlam, Reason: "lease already expired"},
		},
		MinUsableThreshold: 5,
		Availability:       &AvailabilitySignal{SKU: "SKU-1", Usable: 0},
		ReservationId:      "R-1",
		Bin:                &BlastRadius{SKU: "SKU-1", BinId: "BIN-A1", QuantityFreed: 5},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Detected {
		t.Fatal("expected Detected=false: no PICK leases are expired")
	}
	if got.Action != ActionHold {
		t.Fatalf("Action = %q, want %q", got.Action, ActionHold)
	}
}
