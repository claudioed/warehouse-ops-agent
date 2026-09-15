package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/telemetry"
)

func TestPrometheusReader_InstantQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("want path /api/v1/query, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{"metric": {"destination_service_name": "order-management"}, "value": [1700000000, "42.5"]}
				]
			}
		}`))
	}))
	defer srv.Close()

	reader := telemetry.NewPrometheusReader(srv.URL, 5*time.Second)
	samples, err := reader.InstantQuery(context.Background(), `up`)
	if err != nil {
		t.Fatalf("InstantQuery: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	if samples[0].Value != 42.5 {
		t.Errorf("want value 42.5, got %v", samples[0].Value)
	}
	if samples[0].Labels["destination_service_name"] != "order-management" {
		t.Errorf("want label destination_service_name=order-management, got %v", samples[0].Labels)
	}
	if samples[0].Timestamp.Unix() != 1700000000 {
		t.Errorf("want timestamp 1700000000, got %v", samples[0].Timestamp.Unix())
	}
}

func TestPrometheusReader_RangeQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("want path /api/v1/query_range, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "success",
			"data": {
				"resultType": "matrix",
				"result": [
					{"metric": {"le": "100"}, "values": [[1700000000, "1"], [1700000060, "2"]]}
				]
			}
		}`))
	}))
	defer srv.Close()

	reader := telemetry.NewPrometheusReader(srv.URL, 5*time.Second)
	samples, err := reader.RangeQuery(context.Background(), `up`, time.Unix(1700000000, 0), time.Unix(1700000060, 0), time.Minute)
	if err != nil {
		t.Fatalf("RangeQuery: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("want 2 samples, got %d", len(samples))
	}
}

func TestPrometheusReader_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "error", "error": "bad query"}`))
	}))
	defer srv.Close()

	reader := telemetry.NewPrometheusReader(srv.URL, 5*time.Second)
	_, err := reader.InstantQuery(context.Background(), `garbage(((`)
	if err == nil {
		t.Fatal("want an error on a prometheus-reported error status, got nil")
	}
}

func TestPrometheusReader_UnreachableServer(t *testing.T) {
	reader := telemetry.NewPrometheusReader("http://127.0.0.1:1", 100*time.Millisecond)
	_, err := reader.InstantQuery(context.Background(), `up`)
	if err == nil {
		t.Fatal("want an error against an unreachable server, got nil")
	}
}
