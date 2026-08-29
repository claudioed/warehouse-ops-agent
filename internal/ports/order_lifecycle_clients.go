package ports

import (
	"context"
	"errors"
)

// ErrNotFound is returned by any OrderLifecycleClients client when the
// upstream service responds 404 for the requested id/reference. The
// order-lifecycle use case treats this as "this stage hasn't happened
// yet" (a normal, expected state), never as a hard failure of the whole
// lifecycle lookup.
var ErrNotFound = errors.New("resource not found")

// OrderLifecycleClients groups the four bounded-context REST clients the
// console-bff fans out to for GET /console/orders/{id}/lifecycle. These
// are plain REST calls against each service's own public HTTP API — never
// a Go import of another service's packages, and never a database read
// (governance charter: no cross-service DB reads, ever). This is
// deliberately a SEPARATE port shape from WesWorkPlanningClient et al
// above: those wrap each context's curated MCP tools for LLM-facing use
// cases; these wrap each context's plain REST endpoints for a
// browser-facing read model. Mixing the two would blur "MCP surface" and
// "BFF surface" into one interface that serves neither well.
type (
	// OrderManagementClient reads order-management's own order aggregate
	// via GET /orders/{id}.
	OrderManagementClient interface {
		GetOrder(ctx context.Context, orderId string) (*OrderDTO, error)
	}

	// InventoryReservationsClient reads inventory-storage's reservations
	// via GET /reservations?demandRef=<order+line reference>.
	InventoryReservationsClient interface {
		GetReservationsByDemandRef(ctx context.Context, demandRef string) ([]ReservationDTO, error)
	}

	// WorkUnitsClient reads wes-work-planning's work units via
	// GET /work-units?reference=<order-line reference>.
	WorkUnitsClient interface {
		GetWorkUnitsByReference(ctx context.Context, reference string) ([]WorkUnitDTO, error)
	}

	// TasksByOrderClient reads fulfillment-execution's PICK/PACK/SLAM
	// tasks via GET /tasks?orderRef=<order id>.
	TasksByOrderClient interface {
		GetTasksByOrderRef(ctx context.Context, orderRef string) ([]TaskDTO, error)
	}
)

// OrderDTO mirrors order-management's orderResponse wire shape exactly
// (see internal/adapters/inbound/http/dto.go in that repo).
type OrderDTO struct {
	ID                   string         `json:"id"`
	Status               string         `json:"status"`
	AllowPartialShipment bool           `json:"allowPartialShipment"`
	PromiseDate          *string        `json:"promiseDate,omitempty"`
	Lines                []OrderLineDTO `json:"lines"`
}

type OrderLineDTO struct {
	LineNo        int     `json:"lineNo"`
	SKU           string  `json:"sku"`
	Quantity      int     `json:"quantity"`
	PathID        string  `json:"pathId"`
	GiftWrap      bool    `json:"giftWrap"`
	Status        string  `json:"status"`
	ReservationID *string `json:"reservationId,omitempty"`
}

// ReservationDTO mirrors inventory-storage's reservationResponse wire
// shape (only the fields the lifecycle view needs).
type ReservationDTO struct {
	ID        string `json:"id"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	DemandRef string `json:"demandRef"`
	Status    string `json:"status"`
}

// WorkUnitDTO mirrors wes-work-planning's workUnitResponseDTO wire shape.
type WorkUnitDTO struct {
	Id          string  `json:"id"`
	PathId      string  `json:"pathId"`
	Reference   string  `json:"reference"`
	State       string  `json:"state"`
	GiftWrap    bool    `json:"giftWrap"`
	SKU         string  `json:"sku,omitempty"`
	ReleasedAt  *string `json:"releasedAt,omitempty"`
	CompletedAt *string `json:"completedAt,omitempty"`
}

// TaskDTO mirrors fulfillment-execution's taskResponse wire shape (only
// the fields the lifecycle view needs).
type TaskDTO struct {
	Id             string  `json:"id"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	OrderRef       string  `json:"orderRef"`
	Fragile        bool    `json:"fragile"`
	GiftWrap       bool    `json:"giftWrap"`
	LeaseStationId *string `json:"leaseStationId,omitempty"`
}
