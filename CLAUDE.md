# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project: mwanachama-backend-assetmanager

General-purpose asset/inventory-tracking backend for
[mwanachama-backend-shared](../mwanachama-backend-shared)'s `entitygraph`
engine — module path `github.com/aosanya/mwanachama-backend-assetmanager`.
Sibling of [mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager),
same file layout and conventions, different domain: instead of
Task/Agent/Project/WorkflowRun, this is Asset/Location/Movement/Hold.

**Standalone product.** Not part of the Mwanachama party platform's
tier/chapter/RLS model — no DSN board, no `documentation/2. design/todo.md`
row filing. It exists to serve warehouses, supermarkets, household
inventories and livestock tracking, and is designed (see
[documentation/2. design/architecture.md](documentation/2.%20design/architecture.md))
so it can *later* absorb the party's existing merchandise domain
(`merchandise-item`/`merchandise-entry`/`handout`/`merchandise-dispute` in
the `mwanachama` repo's documentation set) without a schema rewrite — that
migration is not in scope yet.

## Current state

Core business logic implemented and tested — see
`documentation/3. implementation/todo_done.md` (A1-A8) for the full build
narrative, including two real deviations from the original scaffold worth
knowing before touching this code:

- **Asset/Movement/Hold's cross-references are plain denormalized string
  properties, not graph edges** (`location_id`, `asset_id`,
  `from_location_id`, `to_location_id`) — same choice taskmanager made for
  `Task.WorkflowRunID`. The **only** real graph edge in this schema is
  `Location`'s `child_of`, because it's the only reference ever walked
  multi-hop (`ListDescendantLocations`).
- `child_of` needs a **declared, symmetric inverse relationship**
  (`RelLabelHasChildLocation`) on `Location` itself —
  `entitygraph.ValidateSchema` requires it, and this was only caught by the
  Postgres integration tests, not the fake-backed unit tests (the fake
  never calls `ValidateSchema`). If you add another self-referencing edge
  with an `Inverse`, declare both directions or `SchemaManager.Publish`
  will reject the schema on first real use — don't trust `go test ./...`
  alone to catch this class of bug.

Files: `schema.go` (`DefaultAssetSchema`), `models.go` (domain types +
`HoldStatus.CanTransitionTo`), `errors.go`, `converters.go`,
`asset_manager.go` (the `AssetManager` interface + constructor),
`location.go`, `asset.go`, `movement.go`, `hold.go`, `hold_watchdog.go`.
Tests: `fake_test.go` (in-memory `DataManager`, single-hop `TraverseGraph`
only — deeper tree/traversal correctness is Postgres-only, see
`postgres_integration_test.go`).

No HTTP/gRPC layer by design (decision #10, 2026-09-03) — this stays a
bare Go package, imported directly by whatever consumes it, the same shape
`mwanachama-backend-taskmanager` has. Not yet imported by any specific
caller. The actor/auth model is still open — `performed_by`/`placed_by`
are bare strings, not edges to a modelled Actor type, on purpose.

## Open questions

Tracked in `documentation/1. requirements/requirements.md`'s "Open
questions" section — actor/auth model, API surface (own HTTP layer vs.
imported as a package), v1 MVP cut line, and how the flexible-attribute
bag is actually represented given `entitygraph.schema` has no object/map
property type. Resolve these before writing `schema.go`/`models.go` for
real, not while writing them.

## Conventions

- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
- Before adding real persistence code, read
  `mwanachama-backend-shared/entitygraph`'s `DataManager` interface and
  `mwanachama-backend-taskmanager/task.go`'s `TaskManager` interface as the
  reference shape for what a domain-specific manager interface looks like
  on top of it.
