---
id: index
title: Architecture Decision Records
sidebar_label: About ADRs
description: Why these records exist, the template they follow, and how to propose a new one.
---

# Architecture Decision Records

An **Architecture Decision Record** captures one architecturally
significant decision: what was decided, what was going on at the time
that made the decision necessary, and what the team now has to live with
as a result. These follow the same convention every warehouse-systems
repository uses: **Michael Nygard's** lightweight format, one markdown
file per decision, numbered `0001-`, `0002-`, and so on, immutable once
accepted.

## The records

| # | Title | Status |
|---|---|---|
| [0001](./0001-warehouse-ops-agent-placement.md) | warehouse-ops-agent placement as a new, thin read-side repo | Accepted |

## Proposing a new one

1. Copy the most recent record and renumber it. Numbers are never reused,
   even if a record is later withdrawn.
2. Write the **Context** first, and write it neutrally.
3. Write **Consequences** honestly, including the ones you dislike.
4. Open it as `Proposed`. Flip it to `Accepted` when it is agreed, or
   leave it `Proposed` and supersede it later if the discussion goes
   elsewhere.
5. Add it to the table above and to `sidebars.ts`.

See [facility-layout's ADR index](https://claudioed.github.io/facility-layout/docs/adr)
for the fuller template rationale this convention is copied from.
