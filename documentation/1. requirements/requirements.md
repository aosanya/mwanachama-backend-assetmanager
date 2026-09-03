# Requirements — mwanachama-backend-assetmanager

## Problem

Track physical stuff — what exists, where it is, who holds it, and what
moved — across domains with nothing else in common: warehouse pallets,
supermarket SKUs, household belongings, livestock. One backend, reused across
all of them, the same way `mwanachama-backend-taskmanager` is one task/project
engine reused across every AI-agent workflow rather than one per project type.

**Eventual target**: this backend should be able to absorb the Mwanachama
party's existing merchandise domain (campaign t-shirts/flags — count on
arrival, hand out at a rally, dispute a count, write off a loss) without a
redesign. That domain is *not* in scope for v1 and this repo carries no
tier/chapter/RLS concept — but the ledger and hold shapes below are chosen
because that domain already proved they work for this exact problem
(`merchandise-entry.md`, `handout.md`, `merchandise-dispute.md` in the
`mwanachama` documentation set).

## Scope decisions (session of 2026-09-03)

| # | Question | Decision |
| - | -------- | -------- |
| 1 | Product boundary | Standalone repo/product. No tier/RLS coupling, no DSN board. Reuses `mwanachama-backend-shared`'s `entitygraph.DataManager` engine and `mwanachama-backend-taskmanager`'s file/schema conventions. |
| 2 | Schema shape | Fixed core types (Asset, Location, Movement, Hold) + a per-asset-type flexible attribute bag, rather than one rigid schema per domain or no schema at all. |
| 3 | Movement model | Append-only ledger of Movement entries that fold to a balance (mirrors `merchandise-entry`), not a mutable "current location" pointer. Chosen specifically so the eventual merchandise migration doesn't require a data-model rewrite. |
| 4 | Tenancy | Multi-tenant. Free: `entitygraph` already scopes every vertex by `AgencyID` — "owner" (a household, a store, a warehouse operator) maps directly onto Agency, no new plumbing needed. |
| 5 | Asset identity | Both serialized (unique tag/serial, its own row and movement history — one specific cow, one specific laptop) and fungible (type + quantity, like `merchandise-item`/`merchandise-variant` today) tracking modes, selected per Asset via `tracking_mode`. |
| 6 | Location model | Recursive tree (`parent_id`), resolved by recursive CTE — the same shape `mwanachama-backend-taskmanager`'s port notes call out for `WorkflowRun` closures, and the same shape the party's chapter tree already uses. Unifies "warehouse > aisle 3 > bin 12", "kitchen > pantry shelf" and (eventually) "chapter > sub-chapter" under one primitive. |
| 7 | Reservations | In scope for v1. A Hold reserves a quantity/asset before it actually moves (checkout, order picking) — not just "record what moved". |
| 8 | Hold semantics | Holds auto-expire (`expires_at`, needs a watchdog — same shape as `workflow_run_watchdog.go`) **and** support partial fulfillment (commit ≤ held quantity; the remainder is released automatically). |
| 9 | v1 deliverable | A design doc (this + `2. design/architecture.md`) plus a scaffolded repo — go.mod, conventions, and the initial `schema.go`/`models.go` shape. No business logic yet. |
| 10 | API surface | No HTTP/gRPC layer of its own — imported directly as a Go package by whatever consumes it, the same shape `mwanachama-backend-taskmanager` has (no `cmd/server`, no service boundary; a caller constructs a `DataManager` and passes it to `NewAssetManager`). Decided 2026-09-03, after A1-A8 landed. Leaves open *which* caller imports it first and whether more than one eventually does (warehouse ops tooling, a household app, the party's merchandise flows) — that's a wiring decision for whoever imports it, not something this repo needs to pre-decide. |
| 11 | Actor identity | No modelled Actor vertex, and no auth of any kind in this package — `performed_by` (Movement) / `placed_by` (Hold) are **required** plain caller-supplied strings, validated non-empty but never interpreted. Falls straight out of decision #10: with no HTTP boundary, there's nowhere in this package for auth to live — whatever authenticates the caller already owns actor identity and is expected to hand this package an id it trusts. Required (not optional) because an un-attributed ledger entry or hold is a real gap merchandise's own `issued_by not null` invariant exists to close. Decided 2026-09-03, implemented in the same pass (`validateMovementShape`, `CreateHold`, `ReverseMovement`) — not deferred to "next session" the way #1-#9 were. |
| 12 | Flexible-attribute mechanics | Confirmed as already implemented, not deferred: a JSON-encoded string property (`Asset.AttributesJSON`), because `entitygraph`'s `schema.PropertyType` has no object/map type (string/integer/float/number/date/datetime/boolean/uuid/option/select/multiselect/array only). Frequently-queried attributes can be promoted to first-class typed properties per domain later — that's an additive migration, not a blocker on today's shape. |

## Open questions

None — decisions #11-#12 above closed out the two carried over from the
prior session (actor/auth model, flexible-attribute mechanics).
