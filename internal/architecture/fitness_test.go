// Package architecture contains fitness tests that encode
// warehouse-ops-agent's hexagonal dependency rule as executable checks (the
// Go equivalent of ArchUnit), using github.com/arch-go/arch-go.
package architecture

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	archgo "github.com/arch-go/arch-go/api"
	"github.com/arch-go/arch-go/api/configuration"
)

// TestMCPAdapterDependencyRule encodes this agent's own governance charter
// (docs/mcp/governance-charter.md, ADR-0008 fleet-wide): the MCP inbound
// adapter is additive, never load-bearing for the composition root. It may
// depend only on the application layer (never outbound adapters, never
// cmd), and — the direction that actually matters for keeping it additive
// — nothing else in this codebase may depend on it. If some other package
// started importing internal/adapters/inbound/mcp, this agent's own MCP
// surface would have silently become a dependency other code relies on
// rather than a pure inbound entrypoint cmd/agent wires up and nothing
// else touches.
func TestMCPAdapterDependencyRule(t *testing.T) {
	moduleInfo := configuration.Load(modulePath)

	t.Run("mcp adapter depends only on application", func(t *testing.T) {
		result := archgo.CheckArchitecture(moduleInfo, configuration.Config{
			DependenciesRules: []*configuration.DependenciesRule{
				{
					Package: "**.inbound.mcp.**",
					ShouldOnlyDependsOn: &configuration.Dependencies{
						Internal: []string{
							"**.inbound.mcp.**",
							"**.application.**",
							"**.domain.**",
							"**.ports.**",
						},
					},
				},
			},
		})

		assertArchGoPasses(t, result)
	})

	t.Run("nothing else depends on the mcp adapter", func(t *testing.T) {
		result := archgo.CheckArchitecture(moduleInfo, configuration.Config{
			DependenciesRules: []*configuration.DependenciesRule{
				{
					Package: "**.internal.**",
					ShouldNotDependsOn: &configuration.Dependencies{
						Internal: []string{"**.inbound.mcp.**"},
					},
				},
			},
		})

		assertArchGoPasses(t, result)
	})
}

// assertArchGoPasses is a shared helper for the two-return-shape arch-go
// result, kept local to this file to avoid touching the existing
// assertPasses/describeViolations helpers in architecture_test.go.
func assertArchGoPasses(t *testing.T, result *archgo.Result) {
	t.Helper()

	if result.Pass {
		return
	}

	if result.DependenciesRuleResult != nil {
		for _, r := range result.DependenciesRuleResult.Results {
			if r.Passes {
				continue
			}
			for _, v := range r.Verifications {
				if v.Passes {
					continue
				}
				for _, d := range v.Details {
					t.Errorf("%s: %s", v.Package, d)
				}
			}
		}
	}

	t.FailNow()
}

// ---------------------------------------------------------------------
// Source-scanning fitness test below, same technique as
// internal/architecture/zerowrite's own static scan.
// ---------------------------------------------------------------------

// goFilesUnder returns every non-test .go file under root (relative to the
// module root).
func goFilesUnder(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

// authPatternRE matches the literal shapes a reintroduced bearer/JWT/API-key
// auth middleware would contain in source. Deliberately narrow (case
// sensitive on "Bearer " and known JWT library import paths) so it doesn't
// false-positive on comment-only mentions of the fleet's auth revert — this
// test scans non-comment, non-test Go source only.
var authPatternRE = regexp.MustCompile(`"Bearer |golang-jwt/jwt|dgrijalva/jwt-go|lestrrat-go/jwx`)

// TestNoAuthMiddlewareReintroduced encodes ADR-0006 (superseding ADR-0005):
// this agent's inbound REST/MCP surfaces are deliberately unauthenticated,
// and its outbound MCP-client calls to the five upstreams carry no bearer
// key, as part of the fleet-wide static-bearer-auth revert on 2026-09-11.
// AGENTS.md explicitly says not to resurrect OIDC_ISSUER_URL/MCP_READ_KEY
// env vars or similar from old code/docs. An agent "helpfully" re-adding a
// bearer/JWT middleware to internal/adapters/inbound should fail CI, not
// ship silently.
func TestNoAuthMiddlewareReintroduced(t *testing.T) {
	for _, path := range goFilesUnder(t, "../adapters/inbound") {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		lineNo := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if authPatternRE.MatchString(line) {
				t.Errorf("%s:%d: matches a reintroduced-auth pattern: %q — ADR-0006 (superseding ADR-0005) deliberately removed all auth fleet-wide 2026-09-11 (unauthenticated pending a fresh auth-model decision); if this is intentional, update this test alongside the ADR documenting the new decision", path, lineNo, trimmed)
			}
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan %s: %v", path, err)
		}
	}
}
