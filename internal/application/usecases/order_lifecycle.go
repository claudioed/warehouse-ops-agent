// OrderLifecycle (the console-bff read model) fans out to four bounded
// contexts to answer "what happened to order X" as a single response,
// mirroring FlowBalanceAdvisory's partial-tolerant orchestration style: a
// slow or down upstream degrades that ONE stage to nil rather than
// failing the whole lookup, so the Order Lifecycle screen can render
// whatever did come back.
package usecases

import (
	"context"
	"errors"
	"log/slog"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// OrderLifecycle is the console-bff's cross-service order-lookup use case.
// It has no domain-layer correlation logic of its own (unlike
// FlowBalanceAdvisory's policy.Decide) — each stage is reported as-is from
// its owning context, since the console's job is to narrate the order's
// journey, not to derive a new judgment about it.
type OrderLifecycle struct {
	OrderManagement *ports.OrderManagementClient
	Inventory       ports.InventoryReservationsClient
	WorkUnits       ports.WorkUnitsClient
	Tasks           ports.TasksByOrderClient

	// Logger receives a structured warning for each upstream call that
	// failed, mirroring FlowBalanceAdvisory's observability convention.
	// Defaults to slog.Default() when nil.
	Logger *slog.Logger
}

// OrderLifecycleResult is the assembled cross-service view. Every field
// past OrderId is a pointer/slice that is nil/empty when that stage
// hasn't happened yet OR its owning service didn't respond — the two
// cases are deliberately not distinguished here (see toTimelineSteps in
// the console frontend for how the UI narrates "not reached yet, or this
// service didn't respond" either way).
type OrderLifecycleResult struct {
	OrderId         string
	OrderManagement *ports.OrderDTO
	Inventory       []ports.ReservationDTO
	Planning        []ports.WorkUnitDTO
	Fulfillment     []ports.TaskDTO
}

// Execute fans out to all four contexts sequentially (v1: simple, a
// handful of sub-second local calls -- parallelizing was deferred, see
// the PR description). The join keys differ per hop, confirmed by
// reading the actual producer code rather than assumed:
//   - inventory-storage's reservation DemandRef IS the plain order id
//     (order-management/internal/application/usecases/allocation.go
//     passes o.ID() verbatim).
//   - wes-work-planning's WorkUnit.Reference is also the plain order id
//     (wes-work-planning/internal/adapters/inbound/kafka/consumer.go's
//     handleOrderManagementEvent sets Reference: data.OrderId).
//   - fulfillment-execution's Task.OrderRef is NOT the plain order id --
//     it's wes's composite WorkUnitId ("<orderId>-line-<lineNo>"), which
//     is what actually flows through the WorkReleased Kafka payload's
//     work_unit_id field into shared.OrderRef(env.Data.WorkUnitId) (see
//     fulfillment-execution/internal/adapters/inbound/kafka/consumer.go).
//     So tasks must be looked up per WorkUnit id returned by the
//     work-units call above, one call per line, not by the plain order
//     id directly.
func (uc *OrderLifecycle) Execute(ctx context.Context, orderId string) (OrderLifecycleResult, error) {
	logger := uc.Logger
	if logger == nil {
		logger = slog.Default()
	}

	result := OrderLifecycleResult{OrderId: orderId}

	order, err := uc.gatherOrder(ctx, orderId)
	if err != nil {
		// order-management not finding the order at all IS a hard
		// failure -- there is no lifecycle to report without it, unlike
		// every downstream stage which is optional.
		return OrderLifecycleResult{}, err
	}
	result.OrderManagement = order
	if order == nil {
		logger.Warn("order_lifecycle: order-management unavailable", "orderId", orderId)
	}

	reservations, err := uc.gatherReservations(ctx, orderId)
	if err != nil {
		logger.Warn("order_lifecycle: inventory-storage unavailable", "orderId", orderId, "error", err)
	}
	result.Inventory = reservations

	workUnits, err := uc.gatherWorkUnits(ctx, orderId)
	if err != nil {
		logger.Warn("order_lifecycle: wes-work-planning unavailable", "orderId", orderId, "error", err)
	}
	result.Planning = workUnits

	// Tasks are keyed by each WorkUnit's own id, not the plain order id
	// -- see the doc comment above. Without any work units (not released
	// yet, or wes unavailable), there is nothing to look up and
	// Fulfillment stays empty rather than guessing at a fallback key.
	var tasks []ports.TaskDTO
	for _, wu := range workUnits {
		found, err := uc.gatherTasks(ctx, wu.Id)
		if err != nil {
			logger.Warn("order_lifecycle: fulfillment-execution unavailable", "orderId", orderId, "workUnitId", wu.Id, "error", err)
			continue
		}
		tasks = append(tasks, found...)
	}
	result.Fulfillment = tasks

	return result, nil
}

func (uc *OrderLifecycle) gatherOrder(ctx context.Context, orderId string) (*ports.OrderDTO, error) {
	if uc.OrderManagement == nil {
		return nil, nil
	}
	order, err := (*uc.OrderManagement).GetOrder(ctx, orderId)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, nil
	}
	return order, nil
}

func (uc *OrderLifecycle) gatherReservations(ctx context.Context, demandRef string) ([]ports.ReservationDTO, error) {
	if uc.Inventory == nil {
		return nil, nil
	}
	return uc.Inventory.GetReservationsByDemandRef(ctx, demandRef)
}

func (uc *OrderLifecycle) gatherWorkUnits(ctx context.Context, reference string) ([]ports.WorkUnitDTO, error) {
	if uc.WorkUnits == nil {
		return nil, nil
	}
	return uc.WorkUnits.GetWorkUnitsByReference(ctx, reference)
}

func (uc *OrderLifecycle) gatherTasks(ctx context.Context, orderRef string) ([]ports.TaskDTO, error) {
	if uc.Tasks == nil {
		return nil, nil
	}
	return uc.Tasks.GetTasksByOrderRef(ctx, orderRef)
}
