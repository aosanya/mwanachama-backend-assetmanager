# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project: mwanachama-backend-assetmanager

General-purpose asset/inventory-tracking backend, GORM-backed — module path
`github.com/aosanya/mwanachama-backend-assetmanager`. Sibling of
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager) in
spirit (same no-HTTP-layer, imported-as-a-package shape), different domain:
instead of Task/Agent/Project/WorkflowRun, this is
Asset/Location/Movement/Hold.

**Standalone product.** Not part of the Mwanachama party platform's
tier/chapter/RLS model — no DSN board, no `documentation/2. design/todo.md`
row filing. It exists to serve warehouses, supermarkets, household
inventories and livestock tracking, and is designed (see
[documentation/2. design/architecture.md](documentation/2.%20design/architecture.md))
so it can *later* absorb the party's existing merchandise domain
(`merchandise-item`/`merchandise-entry`/`handout`/`merchandise-dispute` in
the `mwanachama` repo's documentation set) without a schema rewrite — that
migration is not in scope yet.

**Storage moved from entitygraph to GORM, 2026-09-04** — the same day and
for the same reason as `mwanachama-backend-actor`'s migration (the org
moved away from entitygraph's generic entity/relationship graph engine in
favor of fixed Go structs mapped to plain relational tables), and this
repo mirrored that migration's shape deliberately: `AssetManager`'s method
set is unchanged; only `NewAssetManager`'s constructor and everything
behind it changed. See `documentation/3. implementation/todo_done.md`'s A10
for the full record and `doc.go` for the file layout. Highlights:

- **Domain types and GORM plumbing each live in their own subpackage**,
  same split as actor: `models/` (`Location`, `Asset`, `Movement`, `Hold`,
  one file per type) and `gormstore/` (row structs, `*ToRow`/`*FromRow`
  converters, `Migrate`), one file per entity. No re-export shim in the
  root package — `AssetManager`'s methods spell these types as
  `models.Location` etc. directly, same call actor made for the same
  reason (the type set is small and stable; callers pay a small coupling
  cost instead of an alias layer). Not yet imported by any caller, so this
  interface-shape break costs nothing today.
- **Dropped multi-tenancy**, superseding requirements.md decision #4:
  entitygraph scoped every vertex by `AgencyID` for free; actor's GORM
  migration dropped that scoping outright rather than re-plumbing it as a
  column, and this repo made the identical call for consistency — no
  `AssetManager` method takes a tenant-scoping argument. Revisit if a real
  multi-tenant caller shows up; re-adding it means a column plus a `WHERE`
  clause on every query, not a redesign.
- **`Location`'s `child_of` graph edge and its `parent_location_id`
  denormalized property collapsed into one plain `parent_location_id`
  column** (nullable — `gormstore.StringToNullable`/`nullableToString`
  mirror actor's `GroupRow.ParentID` pattern exactly). A real relational
  table needs only the one; `RelLabelChildOf`/`RelLabelHasChildLocation`
  and the old `schema.go` are gone. `ListDescendantLocations` now walks
  `parent_location_id` with a Go BFS (an in-memory child index, `visited`
  guard against a corrupted cycle) rather than the old edge-walking BFS —
  requirements.md decision #6 named a recursive CTE as the eventual shape,
  and this repo could now write one against the real column, but per
  actor's identical judgment call on `MoveGroup`'s cycle check, that's out
  of scope for a storage-engine swap. Revisit if a deployment's location
  count ever makes the Go walk a real cost.
- **`Asset.SerialTag` uniqueness gained a real DB-level guard**:
  `gormstore.Migrate`'s `syncSerialTagUniqueIndex` adds a partial unique
  index (`WHERE deleted = false AND serial_tag <> ''`) behind
  `CreateAsset`'s existing pre-check query — the entitygraph version only
  had the pre-check, which two concurrent creates could both pass before
  either committed.
- **`Asset.AttributesJSON` stays a plain JSON-encoded string column**, not
  promoted to GORM JSONB the way actor's `Attributes` was — requirements.md
  decision #12 explicitly left that promotion for later, not a blocker on
  today's shape. Don't conflate "GORM makes JSONB available" with "this is
  now the time to promote it."
- **Soft-delete parity**: `LocationRow`/`AssetRow` gained an explicit
  `Deleted bool` column with `WHERE deleted = false` on every Get/List,
  reproducing entitygraph's implicit soft-delete filtering now that this
  repo owns the table directly. `MovementRow`/`HoldRow` have no such
  column — Movement is append-only and Hold has no delete operation, so
  neither needed one before and neither needs one now.

Files: `doc.go` (package doc + layout), `tables.go` (`TableNames`/
`DefaultTableNames`/`Migrate` wrappers), `errors.go` (sentinel errors,
unchanged by the storage swap), `asset_manager.go` (the `AssetManager`
interface + `assetManager` struct + constructor), `location.go`,
`asset.go`, `movement.go`, `hold.go`, `hold_watchdog.go`. `models/` and
`gormstore/` as described above. `routes/` (added 2026-09-04, see below)
holds this repo's HTTP surface: `doc.go`, `wire.go`, `routes.go`, and one
file per aggregate (`location.go`/`asset.go`/`movement.go`/`hold.go`).

Tests: `testdb_test.go` (`newTestManager` — in-memory sqlite via
`glebarez/sqlite`, migrated through `Migrate` the same way a real
deployment would, mirroring actor's `testdb_test.go`), `location_test.go`/
`asset_test.go`/`movement_test.go`/`hold_test.go` against that manager, and
`postgres_integration_test.go` (opens `POSTGRES_URL` through
`gorm.io/driver/postgres`, skipped when unset).

**HTTP surface added 2026-09-04**, superseding decision #10 ("no
HTTP/gRPC layer of its own") — the user asked for it explicitly right
after A10's storage swap, mirroring `mwanachama-backend-actor/routes`'s
shape and conventions: `routes/` package, `Route`/`Route.Pattern`,
`LocationRoutes`/`AssetRoutes`/`MovementRoutes`/`HoldRoutes` + a `Routes`
aggregator, decode-call-encode handlers with no caller-identity gate of
their own (a mounting process wraps the returned `http.HandlerFunc` for
auth). No `ResourceNames` override, unlike actor — actor's exists only
because Group is renamed "chapters" by its gateway, and this repo has no
equivalent org-configurable-noun precedent, so routes address fixed nouns
("locations"/"assets"/"movements"/"holds") directly. The bare-package
import path (`NewAssetManager` with no HTTP dependency) is unchanged and
still works — `routes/` is additive, not a replacement. Full record:
`documentation/3. implementation/todo_done.md`'s A11.

The AssetManager Go API itself is still importable without any HTTP layer
at all, the same shape `mwanachama-backend-taskmanager` has. Actor identity
is decided too (#11): `performed_by`/`placed_by` are required,
caller-supplied strings, validated non-empty but never interpreted — no
modelled Actor type, no auth of any kind anywhere in this repo, including
`routes/` — a route built from this package still needs an auth/capability
gate wrapped around it by whatever mounts it.

**Verification default**: `go test ./...` (sqlite-backed) is the expected
way to verify a change here, not the Postgres integration suite against a
shared container. `postgres_integration_test.go` exists for real
Postgres-wiring coverage sqlite can't fully stand in for, but per
`todo_done.md`'s A8 note, don't reach for a shared `mwanachama-test-pg`-style
container to run it without being asked — a parallel session got
corrected for exactly that ad hoc pattern the same day this repo was
built. See [[feedback_use_memory_backend_for_tests]].

## Open questions

None open — requirements.md's decision table is fully closed (including
the #1/#4 supersessions the 2026-09-04 GORM migration recorded above).

## Conventions

- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
- Before adding new persistence code, read
  `mwanachama-backend-actor`'s `gormstore/` package and `user.go`'s
  `UserManager` interface as the reference shape this repo's GORM layer
  was ported from — same `models/`+`gormstore/` split, same
  `NewXManager(db, tables)` constructor shape.
