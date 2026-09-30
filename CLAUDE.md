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

## The rule that matters most: this module names no domain

**A word that means something in one domain and nothing in another does not
belong in this module.** "Pallet" fails it — a household has belongings.
"Asset" passes it.

It was written for warehouses and must run a household inventory or a
livestock register without a line changing. `domain_agnostic_test.go`
enforces it over four things: identifiers in `models/`, `routes/` and the
root package; every **stored enum value**; the blueprint's roles, fields and
declared values; and the route table's **action ids and paths**. The last
two exist because a stored value and a published address both outlive a
rename, which is the worst version of this failure.

Unlike catalog, nothing here had to be renamed to pass — Asset, Location,
Movement, Hold, Category, SerialTag and every stored value already read the
same to a warehouse, a household and a livestock keeper.

## Objects are declared, not written

**Converted to the declared-domain standard 2026-09-30 (A16–A21)**, the
shape `mwanachama-backend-catalog` sets and
`developer/documentation/2. design/architecture-spec-driven-modules.md`
decided. Before it, storage was `gormstore/`'s five row structs and
`AutoMigrate`; now the tables come from JSON.

- **`assetmanager.blueprint.json`** — the module's five objects, declared
  once. Embedded and reached through `Blueprint()`, `LoadSpec(path)` and
  `ParseSpec(raw)`. Load a domain spec through those, never through
  `spec.Load`, or its roled objects arrive with no fields.
- **`spec/examples/warehouse.assetmanager.json`** and
  **`household.assetmanager.json`** — the same module under two domains,
  chosen as the two furthest apart in `requirements.md`'s problem statement.
  `TestTwoDomainsCoexist` migrates both into one database. Neither is
  provisioned anywhere, so a shipped example is never also production config
  (catalog's CAT6).
- **A domain names objects; it does not re-declare them.** Its spec supplies
  `instance`, the name and table each role lands in, its own indexes and a
  **default** on a declared field — nothing else. `household` sets a default
  on `kind` and declares `warranty`, an object of its own with no role,
  which is how a domain adds a field the module does not have.
- `mwanachama-backend-shared`'s `spec` is the loader and migrator,
  `specstore` the codec, `dispatch` the route builder.

**A column is found by field name, never by json tag**, and **every declared
column is written on every write** — a map missing a key means "leave it
alone" to an update, so omitting empty values would make clearing a field
impossible. **The spec and the types are checked against each other in
`NewAssetManager`**, so a declared column with no field fails at
construction rather than dropping a value on every write.

**A table is `<instance>_hashOf(<mount>)_hashOf(<module>_<object>)`** — only
the instance stays readable. Assert on `spec.RawNameFor`, never on a
physical name literal, and exclude `%_spec_table_names` from anything that
counts tables. See
[declared-domains.md](../mwanachama-backend-shared/documentation/2.%20design/declared-domains.md).

**`asset.serial_tag`'s uniqueness needed the engine widened.** The index is
`unique (serial_tag) where deleted = false and serial_tag <> ''`, and
`spec.Index` could express only the first condition. Every fungible asset
has an empty tag by definition, so an index without the second collides the
second untagged row with the first. `Index.Where` landed in shared as
**S33**; `cmd/ddl` prints the result.

## Rules are declared too, where they can be

`validate.go` reads `required`, an enum's `values` and `matches` off the
spec; `patterns.go` is the registry (`rfc3339`), and a spec naming a pattern
nobody supplies is an error rather than a rule that quietly never runs.
`Check(spec, role, value)` is the same validation without a database.

`check` skips the fields the **store** decides — `id`, `code`, the
timestamps, a hold's `status` — so a caller is held only to what a caller
supplies. That set is `storeOwned`.

**What stays in Go is what a spec cannot say**, each with the type it is
about: the four postable movement shapes, a serialized asset needing a tag,
a positive quantity, an expiry in the future, and
`models.HoldStatus.CanTransitionTo`.

`TestVocabularyMatchesTheBlueprint` holds `models/`'s constants and the
blueprint's declared `values` to each other in **both** directions.

## Routes are declared too

`assetmanager.operations.json` holds all 22 addresses and maps every
sentinel to a status. `AnonymousActions` is the gate boundary and is
**empty** — this module has no auth model, so every address arrives gated
and a mount names what it wants to open. Full record:
[documentation/2. design/routes.md](documentation/2.%20design/routes.md).

## What is superseded

- **`gormstore/`** — **deleted 2026-09-30 (A20)**. Five row structs, their
  `*ToRow`/`*FromRow` converters, `AutoMigrate`-based `Migrate`,
  `DefaultTableNames` and `syncSerialTagUniqueIndex`, replaced by the
  blueprint and `spec.Migrate`. Root `tables.go` and its
  `TableNames`/`DefaultTableNames`/`Migrate` re-export went with it.
  `NewAssetManager(db, *spec.Spec)` is the constructor now.
- **`routes/asset.go`, `hold.go`, `location.go`, `movement.go`** — the
  hand-written handlers, **deleted 2026-09-30 (A17)**. The four builders
  `LocationRoutes`/`AssetRoutes`/`MovementRoutes`/`HoldRoutes` went too: the
  gate boundary they encoded is `AnonymousActions` now, so a mount names
  actions rather than this module's Go identifiers.
- **`models/`'s per-field doc comments** — gone (A18). They were a
  hand-rewritten copy of what the blueprint's `description` now owns, and
  `models/` went from 270 lines to 91. What remains is the types, the
  constants Go compares against, and `CanTransitionTo`.
- **The `models/`+`gormstore/` split and `NewXManager(db, tables)`** that
  this file's Conventions section pointed at `mwanachama-backend-actor` for.
  That pointer recorded the org convention of 2026-09-04 and is superseded
  by the standard; actor is on the same migration path.

`NextCode` did **not** go with `gormstore/` — it moved to `code.go`
unchanged. A counter is not something a spec can declare, and **A15 is an
open bug filed against exactly that function**, so fixing it inside a
refactor is what would have hidden it.

## Behaviours to preserve

- **A caller-supplied id is never honoured** on any create. The server mints
  it (A12), and the two PATCH addresses bind the id from the path, applied
  after the body is decoded.
- **`Asset.TrackingMode` is immutable after creation.** Changing it would
  silently invalidate every prior Movement's balance semantics.
- **A balance is never stored**, always folded from the ledger. A reversal
  is a new entry mirroring an old one, with origin and destination swapped
  so the pair cancels; the original is never edited or deleted.
- **A reservation is committed at most once.** Whatever is not taken is
  released in the same call — there is no `partially_committed` →
  `committed` path.
- **`ListLocations{RootOnly}` filters on `parent_location_id = ''`**, not
  `IS NULL`. The column is a plain string now; the domain type always
  spelled a root as `""` and the `*string` was only a row-boundary dance.
- **A15, A13 and A14 still reproduce.** The conversion was checked against
  them at every step rather than trusted: A13 35/40, A14 196/200 and
  199/200. If one of those pinning tests goes green, investigate — do not
  update the test.

**Storage moved from entitygraph to GORM, 2026-09-04** (A10), which is the
step that made the above possible; the declared-domain move is the step
after. That migration also **dropped multi-tenancy**, superseding
requirements.md decision #4, and collapsed `Location`'s `child_of` edge into
a plain `parent_location_id` column. `spec`'s `<instance>_…` naming is
per-deployment mount scoping, not per-request tenancy, so it does not reopen
#4 — no `AssetManager` method takes a tenant-scoping argument.

**HTTP surface added 2026-09-04** (A11), superseding decision #10 ("no
HTTP/gRPC layer of its own") at the user's explicit request. The bare-package
import path is unchanged and still works — `routes/` is additive.

**Verification default**: `go test ./...` (sqlite via `glebarez/sqlite`).
The Postgres behaviours SQLite cannot stand in for live in
`postgres_integration_test.go` (`//go:build integration`, gated on
`POSTGRES_URL`, run by `make test-pg`), which is **not** part of
`go test ./...`. The build tag was added 2026-09-30 (A21) — before that
there was no build tag in the repo at all, so `make test-pg` selected
nothing and ran exactly what `make test` ran. See
[[feedback_use_memory_backend_for_tests]].

## Open questions

None open — requirements.md's decision table is fully closed (including
the #1/#4 supersessions the 2026-09-04 GORM migration recorded above).

## Conventions

- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
- `mwanachama-backend-catalog` is the reference for this repo's shape, not
  `mwanachama-backend-actor` any more. Read its `CLAUDE.md` and `doc.go`
  before adding persistence code.
- No Go file over 300 lines; split by responsibility.
- There are no hand-written route builders left to name: an address is an
  entry in `assetmanager.operations.json`, and adding one is an edit to that
  file rather than a new Go function.
- This module has **no auth model**: a route still needs a capability gate
  wrapped around it by whatever mounts it.

## Code comments

Write code with no comments. Not one-liners above a function, not section
banners, not doc comments on exported symbols, not "why" notes next to a
tricky line. A name, a type, or a smaller function carries it instead.

Anything that genuinely needs explaining goes in this repo's `documentation/`
folder, under the phase it belongs to (`1. requirements`, `2. design`,
`3. implementation`, `4. qa`) — never inline.

**Why:** inline prose drifts out of sync with the code, duplicates what
`documentation/` already owns, and buries the explanation where nobody
looking for it will search.

**How to apply:**

- New code ships without comments. If a line seems to need one, rename or
  split until it doesn't.
- Touching code that already has comments: strip the ones in the code you are
  changing. Do not sweep untouched files unless asked.
- If the reasoning matters, add or update the matching `documentation/` page
  in the same change and leave nothing behind in the source.
- Machine-read directives are not comments and stay: build tags, `//go:embed`,
  `//go:generate`, linter pragmas (`//nolint`, `// eslint-disable-next-line`,
  `// ignore:`), license headers, codegen "do not edit" banners, and generated
  files as a whole.
- Commit messages, PR descriptions, and test names carry the narration that
  used to go in comments.

This rule is repeated verbatim in every mwanachama repo's `CLAUDE.md` so that
it reaches sessions that do not load this machine's user-level config —
scheduled cloud routines, other machines, and other agent harnesses.
