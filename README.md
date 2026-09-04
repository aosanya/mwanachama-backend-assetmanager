# mwanachama-backend-assetmanager

A general-purpose asset/inventory-tracking backend — warehouses,
supermarkets, household inventories, livestock — no gRPC, no sub-service
shape.

Standalone product — not part of the Mwanachama party platform's tier/RLS
model. Designed so it can eventually absorb the party's existing
merchandise domain (campaign inventory: item/variant/handout/dispute)
without a schema rewrite.

Storage is GORM directly (moved off `mwanachama-backend-shared/entitygraph`,
2026-09-04, mirroring `mwanachama-backend-actor`'s same-day migration — see
that repo's CLAUDE.md and this repo's `doc.go`/CLAUDE.md for the decision
record): `models/` holds the domain types, `gormstore/` holds the row
structs and migration, one file per entity in each.

Status: core business logic implemented — Location/Asset CRUD, the
append-only Movement ledger, and the Hold reservation lifecycle (create,
commit with partial fulfillment, release, expiry watchdog query). Tested
against both an in-memory sqlite `AssetManager` and real Postgres
(`go test ./...` clean). See [documentation/](documentation/) for the
requirements/architecture write-up and
[documentation/3. implementation/todo_done.md](documentation/3.%20implementation/todo_done.md)
for what shipped. Importable as a bare Go package with no HTTP dependency
(the original design), and — since 2026-09-04 — also ships an optional
`routes/` HTTP surface (`routes.Routes(am)`, mirroring
`mwanachama-backend-actor/routes`) for a caller that wants one; not yet
imported by any specific caller either way. Every actor on a Movement or
Hold is required and caller-supplied, with no auth model of its own
anywhere in this repo — a route from `routes/` still needs an
auth/capability gate wrapped around it by whatever mounts it. See
requirements.md's decision table for the full record.
