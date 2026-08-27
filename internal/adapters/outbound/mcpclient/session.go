// Package mcpclient is the outbound MCP-client adapter: one thin,
// schema-typed client per upstream bounded context's Open Host Service. Each
// client in this package implements one of the internal/ports interfaces by
// calling the EXISTING published MCP tools each context already exposes at
// internal/adapters/inbound/mcp/ in its own repo (Streamable HTTP,
// mcp-go-inbound-adapter skill/template). These clients depend on published
// tool schemas (name + JSON input/output shape) only — never on any
// context's Go code; see the sibling repos' tools.go/mapping.go files, which
// are the contract this package binds to.
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config is the connection info for one upstream context's MCP server:
// its Streamable-HTTP endpoint and the static bearer key to present. The key
// is read-scope only for every T1 client — none of them call a write tool.
type Config struct {
	// Name identifies the upstream context in client implementation names
	// and error messages (e.g. "wes-work-planning").
	Name string
	// Endpoint is the Streamable-HTTP MCP endpoint, e.g.
	// "http://wes-work-planning-mcp:8090".
	Endpoint string
	// BearerKey is the static read-scope key for this server (ADR-0008: no
	// IdP). Never logged.
	BearerKey string
	// Timeout bounds a single tool call. Defaults to 10s when zero.
	Timeout time.Duration
}

// bearerRoundTripper attaches the static bearer key to every outbound
// request, so the SDK's StreamableClientTransport needs no knowledge of
// authentication.
type bearerRoundTripper struct {
	key  string
	next http.RoundTripper
}

func (rt *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.key != "" {
		req.Header.Set("Authorization", "Bearer "+rt.key)
	}
	next := rt.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
}

// Session is a thin wrapper over one connected MCP client session to a
// single upstream server, shared by every typed client in this package. Each
// typed client (WesWorkPlanning, FulfillmentExecution, ...) embeds a Session
// and adds one method per tool it calls.
type Session struct {
	cfg Config
}

// New builds a Session for the given upstream server config. It does not
// connect eagerly: connection happens lazily on the first tool call and the
// resulting client session is reused for subsequent calls.
func New(cfg Config) *Session {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Session{cfg: cfg}
}

// callTool opens a fresh MCP client session, calls the named tool with the
// given arguments, and unmarshals its structured content into out. A fresh
// session per call keeps this adapter stateless and simple; the SDK's
// Streamable HTTP client is a lightweight logical connection, not a costly
// TCP handshake, so this trades a small per-call overhead for never having to
// reason about session/reconnect lifecycle in a decision-support agent that
// calls each tool infrequently (interval sampling, not a hot path).
func (s *Session) callTool(ctx context.Context, tool string, args any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "warehouse-ops-agent", Version: "0.1.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint: s.cfg.Endpoint,
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{key: s.cfg.BearerKey},
		},
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("%s: connect: %w", s.cfg.Name, err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s: call %s: %w", s.cfg.Name, tool, err)
	}
	if result.IsError {
		return fmt.Errorf("%s: tool %s reported an error: %s", s.cfg.Name, tool, contentText(result))
	}
	if out == nil {
		return nil
	}

	// Prefer StructuredContent (the typed path every published tool in this
	// fleet returns per the mcp-go-inbound-adapter template); fall back to
	// re-marshaling it into out's concrete type.
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("%s: marshal structured content from %s: %w", s.cfg.Name, tool, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: decode structured content from %s: %w", s.cfg.Name, tool, err)
	}
	return nil
}

// contentText renders a tool result's unstructured content for an error
// message, best-effort.
func contentText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return "(no content)"
}
