package logs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/logs"
)

func TestLokiReader_QueryErrorLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			t.Errorf("want path /loki/api/v1/query_range, got %s", r.URL.Path)
		}
		q := r.URL.Query().Get("query")
		if q == "" {
			t.Error("want a non-empty query param")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "success",
			"data": {
				"result": [
					{
						"stream": {"app": "order-management", "namespace": "warehouse-systems"},
						"values": [["1700000000000000000", "ERROR: something broke"]]
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	reader := logs.NewLokiReader(srv.URL, 5*time.Second)
	entries, err := reader.QueryErrorLines(context.Background(), "warehouse-systems", 600, 50)
	if err != nil {
		t.Fatalf("QueryErrorLines: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Labels["app"] != "order-management" {
		t.Errorf("want label app=order-management, got %v", entries[0].Labels)
	}
	if entries[0].Line != "ERROR: something broke" {
		t.Errorf("want line 'ERROR: something broke', got %q", entries[0].Line)
	}
}

func TestLokiReader_EmptyResultIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "success", "data": {"result": []}}`))
	}))
	defer srv.Close()

	reader := logs.NewLokiReader(srv.URL, 5*time.Second)
	entries, err := reader.QueryErrorLines(context.Background(), "warehouse-systems", 600, 50)
	if err != nil {
		t.Fatalf("want no error on an empty result, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

func TestLokiReader_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status": "error"}`))
	}))
	defer srv.Close()

	reader := logs.NewLokiReader(srv.URL, 5*time.Second)
	_, err := reader.QueryErrorLines(context.Background(), "warehouse-systems", 600, 50)
	if err == nil {
		t.Fatal("want an error on a non-200 response, got nil")
	}
}

func TestLokiReader_UnreachableServer(t *testing.T) {
	reader := logs.NewLokiReader("http://127.0.0.1:1", 100*time.Millisecond)
	_, err := reader.QueryErrorLines(context.Background(), "warehouse-systems", 600, 50)
	if err == nil {
		t.Fatal("want an error against an unreachable server, got nil")
	}
}
