package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// scopeKey is the context key under which the authenticated scope is
// carried from the auth middleware into tool handlers.
type scopeKey struct{}

// scopeFromContext returns the scope stored by the auth middleware, or the
// empty scope if none is present (which scopeAllows treats as
// unauthorized).
func scopeFromContext(ctx context.Context) Scope {
	if s, ok := ctx.Value(scopeKey{}).(Scope); ok {
		return s
	}
	return ""
}

// NewServer builds warehouse-ops-agent's own MCP server: get_daily_brief
// and list_open_exceptions, both read-only. Handlers read the
// authenticated scope from their context (placed there by Handler's
// middleware).
func NewServer(deps Deps) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "warehouse-ops-agent-mcp", Version: "1.0.0"},
		&mcp.ServerOptions{
			Instructions: "Read-only access to warehouse-ops-agent's synthesized daily operational brief: per-site/per-path backlog, staffing, and stuck-task facts, plus correlated open exceptions ranked by severity. This agent never writes to any bounded context.",
		},
	)

	deps.registerTools(server, scopeFromContext)

	return server
}

// Handler returns the Streamable HTTP handler for the MCP server, wrapped
// in the auth middleware. Every request must carry a valid bearer key; the
// scope it grants is placed in the request context for handlers to
// enforce per-tool.
//
// This is the single seam described in ADR-0008: replacing StaticKeyAuth
// with an OAuth 2.1 resource-server Authenticator changes only what is
// passed here, not any handler.
func Handler(server *mcp.Server, auth Authenticator) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, ok := auth.Authenticate(r)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="warehouse-ops-agent-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), scopeKey{}, scope)
		streamable.ServeHTTP(w, r.WithContext(ctx))
	})
}
