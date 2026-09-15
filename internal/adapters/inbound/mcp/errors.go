package mcp

import "fmt"

// invalidSeverityErr centralises the error shape for an out-of-vocabulary
// severity filter, so its wording stays consistent with the five sibling
// contexts' MCP adapters.

func invalidSeverityErr(got string) error {
	return fmt.Errorf("invalid severity %q: must be info, warning, or critical", got)
}
