# Architecture — mwanachama-backend-assetmanager

Companion to [`1. requirements/requirements.md`](../1.%20requirements/requirements.md).
This is the shape agreed in the 2026-09-03 research session and — as of
`todo_done.md`'s A1-A8 — built and tested. One shape decision changed
between the research session and the implementation; see "Core types"
below and A3 in `../3. implementation/todo_done.md` for why.

## Core types

Only `Location`'s `child_of` is a real graph edge — it's the only
reference this schema ever walks multi-hop (`ListDescendantLocations`'s
recursive tree query). `Asset`, `Movement`, and `Hold`'s cross-references
are plain denormalized string properties instead: the same choice
`mwanachama-backend-taskmanager` made for `Task.WorkflowRunID`, and simpler
than an edge for a reference that's always looked up by exact-match filter,
never traversed. (The initial scaffold before A6/A7 drew every reference as
an edge — `located_at`, `of_asset`, `from_location`, `to_location`,
`reserves`, `at_location` — collapsed to properties during implementation.)

```
Agency (tenant) — from entitygraph, not redefined here
  └─ Location ──child_of──► Location              (the one real graph edge — recursive tree)
  └─ Asset (location_id: string)                  (denormalized, updated by Movement)
  └─ Movement (asset_id, from_location_id, to_location_id: string)   (append-only ledger entry)
             (performed_by: string — a bare external actor id, not an edge; see below)
  └─ Hold (asset_id, location_id: string)
       (placed_by: string — a bare external actor id, not an edge; see below)
```

### Asset

One row per trackable thing *or* per fungible type, distinguished by
`tracking_mode`:

- `tracking_mode = "serialized"` — one Asset row is one physical unit
  (a specific cow, a specific laptop). It carries its own `serial_tag`
  (unique per Agency) and its balance is always 0 or 1 at any Location.
- `tracking_mode = "fungible"` — one Asset row is a *type*
  (e.g. "50kg rice bag", "T-shirt, size M" — the `merchandise-item`/
  `merchandise-variant` shape). Its on-hand quantity at a Location is
  derived by folding Movement entries, never stored directly.

Domain-specific attributes (an animal's breed, a laptop's warranty
expiry, a SKU's barcode) live in `attributes_json` — see requirements.md's
open question on flexible-attribute mechanics. Attributes that need to be
filtered/indexed on get promoted to real typed properties later; this is
a v1 simplification, not a permanent ceiling.

### Location

Recursive tree via `parent_id` (nullable — root locations have none).
`kind` (string, open-ended: "warehouse", "aisle", "bin", "room", "shelf",
"pasture"...) is descriptive only — nothing in the schema enforces what
kind of location can nest under what, mirroring how `Task`/`Project` don't
constrain nesting depth either. Aggregate quantity queries ("how much rice
is in this warehouse total") walk the subtree with a recursive CTE, the
same approach the taskmanager port notes prescribe for `WorkflowRun`
closures.

### Movement

Append-only. Never updated, never deleted — a correction is a new Movement
that reverses a prior one (mirrors `merchandise-entry`'s fold-to-zero
ledger and `rpc_resolve_merchandise_dispute`'s "reverse posts the mirror,
original untouched" rule). Current balance at any (Asset, Location) pair
is the fold of every committed Movement affecting it — never a stored
running total, so it can never drift from its own history.

A Movement's `kind` distinguishes at minimum: `arrived` (from outside the
system), `transferred` (Location → Location), `departed` (left the
system), `adjusted` (a counted correction, e.g. a stock-count variance),
`reversed` (the mirror of a prior entry). This list is provisional —
expected to grow the same way `merchandise-entry`'s did (`handed_out`,
`handout_reversed`, `write_off`...) as real flows get built.

### Hold

The v1 stateful piece, closest in spirit to `WorkflowRun`:

```
reserved ──► committed   (posts a Movement for the fulfilled quantity)
         ──► released    (manual release, or automatic on expiry)
         ──► partially_committed ──► committed | released
                                     (remainder auto-released)
```

- `expires_at` — a hold not committed by this time is released
  automatically. Needs a watchdog process, the same role
  `workflow_run_watchdog.go` plays for stuck `WorkflowRun`s — polled
  expiry, not a database-native TTL, since the DataManager is a plain
  Postgres table, not something with expiring rows built in.
- Partial fulfillment — committing a Hold for less than its reserved
  quantity is legal; the difference is released in the same transaction,
  not left dangling. A Hold never silently over-commits: the commit
  amount is checked against the remaining reserved quantity at commit
  time.

Holds only make sense against fungible Assets in the first instance — a
serialized Asset (one specific unit) is either reserved or not, no partial
math applies. Whether serialized Assets get holds at all is left to the
v1 MVP scope question in requirements.md.

## What this deliberately does not decide yet

- The Actor/auth model (who `performed_by`/`placed_by` actually points at,
  and how the caller authenticates) — requirements.md's open question.
- Whether this ships its own HTTP API or is imported as a package the way
  taskmanager is.
- Which of the above ships in v1 vs. is stubbed — this document describes
  the target shape, not the first cut.

## Precedent this design leans on

- `mwanachama-backend-taskmanager/schema.go`, `models.go`,
  `workflow_run.go`, `workflow_run_watchdog.go` — file layout, string-enum
  state machines with `CanTransitionTo`, and the watchdog pattern for a
  timed-out stateful process.
- `mwanachama` (party) `documentation/2. design/datamodel/merchandise-entry.md`,
  `handout.md`, `merchandise-dispute.md` — the ledger-folds-to-balance
  design and the "reverse posts a mirror, never edits" rule this schema is
  built to be compatible with.
