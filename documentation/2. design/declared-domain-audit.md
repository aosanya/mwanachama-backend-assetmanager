# Declared-domain audit — mwanachama-backend-assetmanager

Audited 2026-09-30 against the standard `mwanachama-backend-catalog` sets,
per `developer/documentation/2. design/architecture-spec-driven-modules.md`
(decided 2026-09-23). The engine is `mwanachama-backend-shared`'s `spec`,
`specstore`, `dispatch` and `httpwire`.

**Verdict: convert.** No exclusion applies, and this repo has the most
favourable precondition of any candidate — nothing mounts it, so there are
no live tables to adopt. Two things must land before Step B, both named
below.

## Gate table

| # | Gate | Verdict | Evidence |
|---|------|---------|----------|
| 1 | Blueprint | ❌ | `ls *.blueprint.json` → no matches |
| 2 | Blueprint reached from Go | ❌ | no `blueprint.go` |
| 3 | Domain specs | ❌ | no `*.json` at root, no `spec/` directory at all |
| 4 | No row structs | ❌ | `gormstore/` carries `LocationRow`, `AssetRow`, `MovementRow`, `HoldRow`, `CodeSequenceRow` with `*ToRow`/`*FromRow` converters; root `tables.go` re-exports `TableNames`/`DefaultTableNames`/`Migrate`; five `AutoMigrate` calls in `gormstore/tables.go:38-53` |
| 5 | Store | ❌ | no `store.go`; storage is per-entity hand-written converters |
| 6 | Constructor cross-check | ❌ | `NewAssetManager(db *gorm.DB, t TableNames)` (`asset_manager.go:172`) takes a table-name struct, not a spec. Nothing holds columns and fields to each other |
| 7 | Declared rules | ❌ | no `validate.go`, no `patterns.go`. Validation is hand-written per manager file (`validateMovementShape`, `CreateHold`'s expiry check, `CreateAsset`'s tracking-mode/serial-tag rules) |
| 8 | Thin models | ⚠️ | `grep 'func.*Validate\|func Is' models/` is clean — no `Validate` methods, no `IsX` membership maps. Better than catalog was pre-CAT3. But every field carries a doc comment restating what a blueprint `description` would own (`models/asset.go` is 68 lines for 10 fields). `HoldStatus.CanTransitionTo` is the only method and is a rule a spec cannot state, so it stays |
| 9 | Operations | ❌ | no `*.operations.json`, no `operations.go` |
| 10 | Routes are an adapter | ❌ | `routes/routes.go` is an 84-line hand-built ladder over 22 addresses; 541 lines of decode-call-encode handlers across `asset.go`/`hold.go`/`location.go`/`movement.go`; 29 `httpwire.ReadJSON`/`WriteJSON` call sites |
| 11 | Gate is data | ❌ | no `AnonymousActions`. The gate boundary is four Go builder functions — `LocationRoutes`/`AssetRoutes`/`MovementRoutes`/`HoldRoutes` |
| 12 | MCP is the same table | n/a | no `mcp/` package, no `routes/tools.go` |
| 13 | Provisioning | ❌ | no `provision.go`, no `cmd/`. `Migrate(db, t)` wraps `gormstore.Migrate`'s `AutoMigrate` |
| 14 | Guard tests | ❌ 0/6 | none of the six exist. `postgres_integration_test.go` is present but **carries no build tag** — see "Two preconditions" below |
| 15 | Depends on shared | ✅ | `go.mod` requires `mwanachama-backend-shared v0.0.0` with `replace => ../mwanachama-backend-shared`. `httpwire` and `gormtest` are already imported |
| 16 | Makefile | ✅ | `build`, `test`, `test-pg`, `vet`, `clean` all present |
| 17 | Documentation | ⚠️ | four-phase layout present (`4. qa` empty); board rows A13/A14/A15 open in `3. implementation/todo.md`; no `2. design/routes.md` |
| 18 | House rules | ⚠️ | no Go file over 300 lines (largest is `location_test.go` at 290) ✅; comments throughout ❌ — the no-comments rule postdates most of this code, and `CLAUDE.md` says not to sweep untouched files |

Two clean passes, three partial, twelve fails, one not applicable.
`go test ./...` is green today (the baseline the conversion must hold).

## Why conversion is warranted

None of the five exclusions applies:

- **Not the engine.** This is a module, not `mwanachama-backend-shared`.
- **Has a store of its own.** `gormstore/`, five row structs.
- **Not entitygraph-backed.** Storage moved to GORM on 2026-09-04 (A10).
  The prerequisite step is already done; the declared-domain move is
  precisely the step after.
- **No live table set.** `grep -rn assetmanager` across
  `mwanachama-backend-api-gateway` and `mwanachama-wakala-api` finds only
  prose. `cmd/server/modules.go:16` says so outright: "assetmanager has no
  import into this gateway". Nothing has ever provisioned an
  `<instance>_locations` table, so there are **no rows to adopt** and Step H
  needs no legacy-adoption path. This is the cleanest starting position of
  any candidate repo.
- **Not deliberately shaped otherwise.** `requirements.md` closes every
  decision; none of them chooses hand-written row structs on the merits.
  Decision #1 and #4 both record that this repo mirrors whatever
  `mwanachama-backend-actor` did for consistency, which is an argument for
  moving with the org, not against it.

It also **already passes the rule that matters most.** Unlike
`mwanachama-backend-insights`, no identifier here carries a domain word:
Asset, Location, Movement, Hold, Category, SerialTag, TrackingMode and Code
all mean the same thing to a warehouse, a supermarket, a household and a
livestock keeper. The stored enum values — `serialized`/`fungible`,
`arrived`/`transferred`/`departed`/`adjusted`/`reversed`,
`reserved`/`committed`/`released`/`partially_committed` — are neutral too.
`domain_agnostic_test.go` should go green on the current names, which makes
Step A a transcription rather than a renaming argument.

## Two preconditions

### 1. `spec.Index` cannot express this module's serial-tag index

`gormstore/tables.go`'s `syncSerialTagUniqueIndex` creates:

```sql
CREATE UNIQUE INDEX ... ON <assets> (serial_tag)
  WHERE deleted = false AND serial_tag <> ''
```

`spec.Index` (shared, `spec/spec.go:131`) carries `Unique` and `NotDeleted`
and nothing else, and `createIndexes` (`spec/migrate.go`) renders
`NotDeleted` as exactly `where deleted = false`. The `AND serial_tag <> ''`
half is inexpressible.

This is not cosmetic. Every fungible Asset has `SerialTag == ""` by
definition, so a unique index over `serial_tag WHERE deleted = false` would
collide the second fungible asset ever created with the first. A naive
transcription of this index breaks `CreateAsset` for half the module's
tracking modes.

It is also not this module's problem to solve locally — a partial index over
a declared field is exactly what the engine should own. Per the
standardize-into-shared rule, the fix goes into `spec.Index` (a declared
predicate over declared fields, the way `NotDeleted` already is) and this
repo is the only one switched onto it in the same pass. **This lands before
Step B.**

### 2. `make test-pg` currently selects nothing

There is not one `//go:build` line in the repo. `postgres_integration_test.go`
is an ordinary test file gated at runtime on `POSTGRES_URL`, so
`go test ./...` compiles and runs it, and `make test-pg`'s
`go test -tags=integration ./...` selects no additional file — the two
targets run an identical set. Gate 14's sixth test wants
`//go:build integration`, and adding it is part of Step I rather than a
separate fix.

## What the conversion has to carry across

Named here so the refactor cannot quietly drop them.

- **`NextCode` has no declared equivalent.** `specstore` has no sequence
  concept, so `gormstore/codesequence.go`'s counter is module Go that
  survives Step G — it belongs in the "rules a spec cannot state" bucket
  beside `CanTransitionTo`. **A15 is an open 🐞 against exactly this
  function**, and the behaviour-is-held rule says it stays broken through the
  conversion: fixing a filed race inside a refactor is what would hide it.
  `code_sequence` becomes a declared object like any other
  (`entity_type` primary, `next_number` int).
- **A13 and A14's pinning tests assert the current broken behaviour** and
  drive through `routes.Routes(am)` behind `httptest`. They must be re-pointed
  at the dispatcher-built table with their assertions unchanged. Both races
  live in `hold.go`/`asset.go`/`location.go`'s read-then-write sequences, not
  in the storage codec, so `specstore` will not accidentally close them — but
  if either test goes green, that is a signal to investigate, not to update
  the test.
- **`Location.ParentLocationID` is `*string` at the row boundary**, `""` in
  the domain type. `specstore`'s nullable handling replaces
  `StringToNullable`/`nullableToString`; `nullable_test.go` in shared is the
  reference for what it does with an absent value.
- **`Asset.AttributesJSON` should become a declared `json` field**, which is
  `jsonb` on Postgres and text on SQLite. That is requirements decision #12's
  deferred promotion arriving for free, and it is the one place the
  conversion improves on today's shape rather than reproducing it.
- **Every sentinel in `errors.go` needs a status in `operations.json`'s
  `errors` map.** There are fourteen. An unmapped one is redacted to a 500
  with `"internal error"` — catalog's CAT7. Transcribe the deleted handlers'
  status tables exactly and file anything that looks wrong as a board row.
- **The gateway SQL mirror (Step H) is deferred, not skipped.** The gateway
  keeps its active migrations under
  `internal/store/postgres/migrations/` and has a `cmd/migrate`, but nothing
  there references this module. The mirror is owed whenever something first
  mounts it.

## Conflicts with this repo's own record

Surfaced rather than resolved, per the skill's rule.

- **`CLAUDE.md`'s Conventions section** says to read
  `mwanachama-backend-actor`'s `gormstore/` before adding persistence code,
  and names "same `models/`+`gormstore/` split, same `NewXManager(db, tables)`
  constructor shape" as the reference. The standard deletes both. This reads
  as a pointer at the then-current org convention rather than a prohibition —
  and actor itself is on the same migration path — so it is treated as
  superseded by Step J rather than as a blocker. Say so if that reading is
  wrong.
- **Decision #4 dropped multi-tenancy.** The spec engine's
  `<instance>_<hashOf(mount)>_<hashOf(module_object)>` naming is per-deployment
  mount scoping, not per-request tenancy, and no `AssetManager` method gains
  an argument. `DefaultTableNames(instance)` already carries an instance
  prefix, so the shape maps over without reopening #4.

## Candidate duplicates — restated against this repo

The skill's candidate table lists `routes/wire.go`'s `writeJSON`/`writeErr`
and a local `Route` struct as still present here. **That is stale.** This
repo has no `wire.go`; `routes/` imports `httpwire` directly and
`routes/routes.go:15` is `type Route = httpwire.Route`, a plain alias. The
deletion that candidate asks for has already happened here —
`mwanachama-backend-insights` is the remaining carrier.

The builder-ladder candidate does apply: `LocationRoutes`/`AssetRoutes`/
`MovementRoutes`/`HoldRoutes`/`Routes` is a fifth variant of the drifted
ladder, with none of the `…With` forms catalog and taskmanager grew.
