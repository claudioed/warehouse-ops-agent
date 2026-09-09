// Package auth authenticates REST requests with standards-compliant OIDC
// discovery and JWT verification. It deliberately has no bypass or
// permissive mode: a configured server accepts only issuer-signed tokens for
// its configured audience.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	ScopeRead  = "warehouse-ops-agent.read"
	ScopeWrite = "warehouse-ops-agent.write"
)

// Middleware verifies bearer access tokens and enforces the operation scope.
type Middleware struct{ verifier *oidc.IDTokenVerifier }

// New discovers issuer metadata and its JWKS. Discovery errors are returned
// to the composition root so a misconfigured service cannot start open.
func New(ctx context.Context, issuerURL, clientID string) (*Middleware, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}
	return &Middleware{verifier: provider.Verifier(&oidc.Config{ClientID: clientID})}, nil
}

// Handler protects a REST route group. GET and HEAD require read scope; all
// other methods require write scope. A write scope also satisfies read.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			unauthorized(w, "missing bearer token")
			return
		}
		idToken, err := m.verifier.Verify(r.Context(), token)
		if err != nil {
			unauthorized(w, "invalid bearer token")
			return
		}
		var claims struct {
			Scope  string   `json:"scope"`
			Scopes []string `json:"scp"`
		}
		if err := idToken.Claims(&claims); err != nil {
			unauthorized(w, "invalid bearer token claims")
			return
		}
		required := ScopeWrite
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			required = ScopeRead
		}
		if !allows(claims.Scope, claims.Scopes, required) {
			forbidden(w, required)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnValue := len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != ""
	if !returnValue {
		return "", false
	}
	return parts[1], true
}

func allows(scope string, scopes []string, required string) bool {
	all := append(strings.Fields(scope), scopes...)
	for _, actual := range all {
		if actual == required || (required == ScopeRead && actual == ScopeWrite) {
			return true
		}
	}
	return false
}

func unauthorized(w http.ResponseWriter, detail string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="warehouse-ops-agent", error="invalid_token"`)
	problem(w, http.StatusUnauthorized, "urn:warehouse-ops-agent:auth:invalid-token", "Unauthorized", detail)
}
func forbidden(w http.ResponseWriter, required string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="warehouse-ops-agent", error="insufficient_scope", scope="`+required+`"`)
	problem(w, http.StatusForbidden, "urn:warehouse-ops-agent:auth:insufficient-scope", "Forbidden", "token lacks required scope")
}
func problem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": typ, "title": title, "status": status, "detail": detail})
}
