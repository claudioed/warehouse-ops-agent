package mcp

import "fmt"

// unauthorizedErr and invalidSeverityErr centralise the two error shapes
// tool handlers in this package return, so their wording stays consistent
// with the five sibling contexts' MCP adapters.

func unauthorizedErr(toolName string, required Scope) error {
	return fmt.Errorf("tool %q requires %s scope", toolName, required)
}

func invalidSeverityErr(got string) error {
	return fmt.Errorf("invalid severity %q: must be info, warning, or critical", got)
}
