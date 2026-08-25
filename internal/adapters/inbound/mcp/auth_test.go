package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScopeAllows(t *testing.T) {
	tests := []struct {
		name     string
		granted  Scope
		required Scope
		want     bool
	}{
		{"read-write satisfies read", ScopeReadWrite, ScopeRead, true},
		{"read-write satisfies read-write", ScopeReadWrite, ScopeReadWrite, true},
		{"read satisfies read", ScopeRead, ScopeRead, true},
		{"read does not satisfy read-write", ScopeRead, ScopeReadWrite, false},
		{"empty scope satisfies nothing", Scope(""), ScopeRead, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeAllows(tc.granted, tc.required); got != tc.want {
				t.Errorf("scopeAllows(%q, %q) = %v, want %v", tc.granted, tc.required, got, tc.want)
			}
		})
	}
}

func TestStaticKeyAuth_Authenticate(t *testing.T) {
	auth := NewStaticKeyAuth(map[string]Scope{
		"good-key": ScopeRead,
		"":         ScopeReadWrite, // must be filtered out; a blank key must never authorize.
	})

	tests := []struct {
		name      string
		header    string
		wantScope Scope
		wantOK    bool
	}{
		{"valid bearer key", "Bearer good-key", ScopeRead, true},
		{"unknown key rejected", "Bearer wrong-key", "", false},
		{"missing header rejected", "", "", false},
		{"malformed header (no Bearer prefix) rejected", "good-key", "", false},
		{"empty token after Bearer rejected", "Bearer ", "", false},
		{"case-insensitive Bearer prefix accepted", "bearer good-key", ScopeRead, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			scope, ok := auth.Authenticate(req)
			if ok != tc.wantOK || scope != tc.wantScope {
				t.Errorf("Authenticate() = (%q, %v), want (%q, %v)", scope, ok, tc.wantScope, tc.wantOK)
			}
		})
	}
}

func TestScopeFromContext_Empty(t *testing.T) {
	if got := scopeFromContext(context.Background()); got != "" {
		t.Errorf("scopeFromContext(background) = %q, want empty for a context with no scope set", got)
	}
}

func TestUnauthorizedErr(t *testing.T) {
	err := unauthorizedErr("get_daily_brief", ScopeRead)
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	const want = `tool "get_daily_brief" requires read scope`
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestInvalidSeverityErr(t *testing.T) {
	err := invalidSeverityErr("apocalyptic")
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
}
