---
id: getting-started
title: Getting started
sidebar_label: Getting started
description: Run warehouse-ops-agent locally, wired against the five sibling MCP servers, and hit its daily brief.
---

# Getting started

`warehouse-ops-agent` is a single Go binary. It has no database of its own
— it holds no persisted state — and needs no Postgres. What it does need
is a Streamable HTTP endpoint and a static bearer read-key for each of the
five upstream contexts' MCP servers (see
[Configuration](https://github.com/claudioed/warehouse-ops-agent#configuration)
in the repo README for the full environment-variable table).

## Run it against the full fleet

The simplest way to exercise this agent end-to-end is via the shared
`e2e-tests` harness, which brings up all five bounded contexts, their MCP
servers, and this agent together:

```bash
cd ~/warehouse-systems/e2e-tests
bash scripts/02-up-infra.sh      # Kafka + 5 Postgres instances
bash scripts/01-build.sh         # builds all binaries, including this agent
bash scripts/03-up-services.sh   # starts every service + every MCP server
                                  # + this agent, pointed at them
```

## Run it standalone

```bash
export WES_WORK_PLANNING_MCP_ENDPOINT=http://localhost:8091/mcp
export WES_WORK_PLANNING_MCP_READ_KEY=***
export FULFILLMENT_EXECUTION_MCP_ENDPOINT=http://localhost:8092/mcp
export FULFILLMENT_EXECUTION_MCP_READ_KEY=***
export INVENTORY_STORAGE_MCP_ENDPOINT=http://localhost:8093/mcp
export INVENTORY_STORAGE_MCP_READ_KEY=***
export WORKFORCE_MANAGEMENT_MCP_ENDPOINT=http://localhost:8094/mcp
export WORKFORCE_MANAGEMENT_MCP_READ_KEY=***
export FACILITY_LAYOUT_MCP_ENDPOINT=http://localhost:8095/mcp
export FACILITY_LAYOUT_MCP_READ_KEY=***
export MCP_READ_KEY=***          # this agent's OWN inbound MCP read key
export AGENT_ADDR=:8096

go run ./cmd/agent
```

## Hit the daily brief

```bash
curl -s http://localhost:8096/daily-brief | jq .
```

A path with two or more correlated signals (backlog over its alarm
threshold, understaffed, or stuck tasks) shows up under
`openExceptions`, each entry carrying its `evidence` trail.

## Quality gate

```
make check       # fast pre-commit bundle: fmt-check vet build lint test
make check-all   # + arch-test (pre-push gate)
```

`lefthook install` once to activate the pre-commit/pre-push git hooks.
