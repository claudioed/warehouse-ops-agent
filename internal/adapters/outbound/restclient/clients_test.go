package restclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

func TestOrderManagement_GetOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/ord-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ports.OrderDTO{ID: "ord-1", Status: "Received"})
	}))
	defer srv.Close()

	client := NewOrderManagement(srv.URL, 0)
	order, err := client.GetOrder(context.Background(), "ord-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.ID != "ord-1" || order.Status != "Received" {
		t.Fatalf("unexpected order: %+v", order)
	}
}

func TestOrderManagement_GetOrder_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewOrderManagement(srv.URL, 0)
	_, err := client.GetOrder(context.Background(), "missing")
	if err != ports.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOrderManagement_GetOrder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewOrderManagement(srv.URL, 0)
	if _, err := client.GetOrder(context.Background(), "ord-1"); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestInventoryReservations_GetReservationsByDemandRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("demandRef"); got != "ord-1" {
			t.Fatalf("expected demandRef=ord-1, got %q", got)
		}
		_ = json.NewEncoder(w).Encode([]ports.ReservationDTO{{SKU: "SKU-1", Quantity: 2, Status: "ACTIVE"}})
	}))
	defer srv.Close()

	client := NewInventoryReservations(srv.URL, 0)
	reservations, err := client.GetReservationsByDemandRef(context.Background(), "ord-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reservations) != 1 || reservations[0].SKU != "SKU-1" {
		t.Fatalf("unexpected reservations: %+v", reservations)
	}
}

func TestWorkUnits_GetWorkUnitsByReference(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("reference"); got != "ord-1" {
			t.Fatalf("expected reference=ord-1, got %q", got)
		}
		_ = json.NewEncoder(w).Encode([]ports.WorkUnitDTO{{Id: "ord-1-line-1", PathId: "pick-zone-a", State: "Released"}})
	}))
	defer srv.Close()

	client := NewWorkUnits(srv.URL, 0)
	units, err := client.GetWorkUnitsByReference(context.Background(), "ord-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(units) != 1 || units[0].Id != "ord-1-line-1" {
		t.Fatalf("unexpected work units: %+v", units)
	}
}

func TestTasksByOrder_GetTasksByOrderRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("orderRef"); got != "ord-1-line-1" {
			t.Fatalf("expected orderRef=ord-1-line-1, got %q", got)
		}
		_ = json.NewEncoder(w).Encode([]ports.TaskDTO{{Id: "task-1", Type: "PICK", Status: "COMPLETED"}})
	}))
	defer srv.Close()

	client := NewTasksByOrder(srv.URL, 0)
	tasks, err := client.GetTasksByOrderRef(context.Background(), "ord-1-line-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Id != "task-1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestHTTPGetJSON_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := NewOrderManagement(srv.URL, 0)
	if _, err := client.GetOrder(context.Background(), "ord-1"); err == nil {
		t.Fatal("expected a decode error")
	}
}
