package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// fakeOrderManagement, fakeInventoryReservations, fakeWorkUnits,
// fakeTasksByOrder are table-driven, in-memory fakes of the console-bff's
// four REST-client ports -- no live server, mirroring the existing
// fakeWes/fakeFe/fakeWfm convention in fakes_test.go.

type fakeOrderManagement struct {
	order *ports.OrderDTO
	err   error
}

func (f *fakeOrderManagement) GetOrder(ctx context.Context, orderId string) (*ports.OrderDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.order, nil
}

type fakeInventoryReservations struct {
	byDemandRef map[string][]ports.ReservationDTO
	err         error
}

func (f *fakeInventoryReservations) GetReservationsByDemandRef(ctx context.Context, demandRef string) ([]ports.ReservationDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byDemandRef[demandRef], nil
}

type fakeWorkUnits struct {
	byReference map[string][]ports.WorkUnitDTO
	err         error
}

func (f *fakeWorkUnits) GetWorkUnitsByReference(ctx context.Context, reference string) ([]ports.WorkUnitDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byReference[reference], nil
}

type fakeTasksByOrder struct {
	byOrderRef map[string][]ports.TaskDTO
	err        map[string]error
	calls      []string
}

func (f *fakeTasksByOrder) GetTasksByOrderRef(ctx context.Context, orderRef string) ([]ports.TaskDTO, error) {
	f.calls = append(f.calls, orderRef)
	if err, ok := f.err[orderRef]; ok {
		return nil, err
	}
	return f.byOrderRef[orderRef], nil
}

var (
	_ ports.OrderManagementClient       = (*fakeOrderManagement)(nil)
	_ ports.InventoryReservationsClient = (*fakeInventoryReservations)(nil)
	_ ports.WorkUnitsClient             = (*fakeWorkUnits)(nil)
	_ ports.TasksByOrderClient          = (*fakeTasksByOrder)(nil)
)

func TestOrderLifecycle_Execute_FullHappyPath(t *testing.T) {
	order := &ports.OrderDTO{
		ID:     "ord-1",
		Status: "Released",
		Lines:  []ports.OrderLineDTO{{LineNo: 1, SKU: "SKU-1", Quantity: 2, Status: "Released"}},
	}
	var omClient ports.OrderManagementClient = &fakeOrderManagement{order: order}
	inv := &fakeInventoryReservations{byDemandRef: map[string][]ports.ReservationDTO{
		"ord-1": {{SKU: "SKU-1", Quantity: 2, Status: "CONFIRMED"}},
	}}
	wu := &fakeWorkUnits{byReference: map[string][]ports.WorkUnitDTO{
		"ord-1": {{Id: "ord-1-line-1", PathId: "pick-zone-a", State: "Released"}},
	}}
	tasks := &fakeTasksByOrder{byOrderRef: map[string][]ports.TaskDTO{
		"ord-1-line-1": {{Id: "task-1", Type: "PICK", Status: "COMPLETED"}},
	}}

	uc := &usecases.OrderLifecycle{OrderManagement: &omClient, Inventory: inv, WorkUnits: wu, Tasks: tasks}
	result, err := uc.Execute(context.Background(), "ord-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OrderManagement == nil || result.OrderManagement.ID != "ord-1" {
		t.Fatalf("expected order to be populated, got %+v", result.OrderManagement)
	}
	if len(result.Inventory) != 1 || result.Inventory[0].SKU != "SKU-1" {
		t.Fatalf("unexpected inventory: %+v", result.Inventory)
	}
	if len(result.Planning) != 1 || result.Planning[0].Id != "ord-1-line-1" {
		t.Fatalf("unexpected planning: %+v", result.Planning)
	}
	if len(result.Fulfillment) != 1 || result.Fulfillment[0].Id != "task-1" {
		t.Fatalf("unexpected fulfillment: %+v", result.Fulfillment)
	}
	// Tasks must be looked up by the WorkUnit's own id, not the plain
	// order id -- this is the crux of the whole join-key correction; a
	// regression here would silently break the Order Lifecycle screen's
	// fulfillment stage on every real order.
	if len(tasks.calls) != 1 || tasks.calls[0] != "ord-1-line-1" {
		t.Fatalf("expected GetTasksByOrderRef to be called with the work unit id, got %+v", tasks.calls)
	}
}

func TestOrderLifecycle_Execute_OrderNotFound_IsHardFailure(t *testing.T) {
	var omClient ports.OrderManagementClient = &fakeOrderManagement{err: ports.ErrNotFound}
	uc := &usecases.OrderLifecycle{OrderManagement: &omClient}

	_, err := uc.Execute(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expected ErrNotFound to propagate, got %v", err)
	}
}

func TestOrderLifecycle_Execute_PartialUpstreamFailure_DegradesNotFails(t *testing.T) {
	order := &ports.OrderDTO{ID: "ord-2", Status: "Received"}
	var omClient ports.OrderManagementClient = &fakeOrderManagement{order: order}
	inv := &fakeInventoryReservations{err: errors.New("inventory-storage unreachable")}
	wu := &fakeWorkUnits{err: errors.New("wes-work-planning unreachable")}

	uc := &usecases.OrderLifecycle{OrderManagement: &omClient, Inventory: inv, WorkUnits: wu}
	result, err := uc.Execute(context.Background(), "ord-2")
	if err != nil {
		t.Fatalf("a downstream failure must degrade, not error out: %v", err)
	}
	if result.OrderManagement == nil {
		t.Fatal("order-management result should still be present")
	}
	if result.Inventory != nil {
		t.Fatalf("expected nil inventory on upstream failure, got %+v", result.Inventory)
	}
	if result.Planning != nil {
		t.Fatalf("expected nil planning on upstream failure, got %+v", result.Planning)
	}
	if result.Fulfillment != nil {
		t.Fatalf("expected no fulfillment lookups without any work units, got %+v", result.Fulfillment)
	}
}

func TestOrderLifecycle_Execute_NoWorkUnitsYet_SkipsTaskLookup(t *testing.T) {
	order := &ports.OrderDTO{ID: "ord-3", Status: "Received"}
	var omClient ports.OrderManagementClient = &fakeOrderManagement{order: order}
	wu := &fakeWorkUnits{byReference: map[string][]ports.WorkUnitDTO{}}
	tasks := &fakeTasksByOrder{}

	uc := &usecases.OrderLifecycle{OrderManagement: &omClient, WorkUnits: wu, Tasks: tasks}
	result, err := uc.Execute(context.Background(), "ord-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks.calls) != 0 {
		t.Fatalf("expected no task lookups when there are no work units yet, got %+v", tasks.calls)
	}
	if result.Fulfillment != nil {
		t.Fatalf("expected nil fulfillment, got %+v", result.Fulfillment)
	}
}

func TestOrderLifecycle_Execute_NilClients_DoNotPanic(t *testing.T) {
	order := &ports.OrderDTO{ID: "ord-4", Status: "Received"}
	var omClient ports.OrderManagementClient = &fakeOrderManagement{order: order}
	uc := &usecases.OrderLifecycle{OrderManagement: &omClient}

	result, err := uc.Execute(context.Background(), "ord-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Inventory != nil || result.Planning != nil || result.Fulfillment != nil {
		t.Fatalf("expected all downstream stages nil when their clients are unwired, got %+v", result)
	}
}
