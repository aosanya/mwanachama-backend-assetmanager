# mwanachama-backend-assetmanager

A general-purpose asset/inventory-tracking backend — warehouses,
supermarkets, household inventories, livestock — built the same way
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager) is: on
[mwanachama-backend-shared](../mwanachama-backend-shared)'s Postgres
entity-graph store, no gRPC, no sub-service shape.

Standalone product — not part of the Mwanachama party platform's tier/RLS
model. Designed so it can eventually absorb the party's existing
merchandise domain (campaign inventory: item/variant/handout/dispute)
without a schema rewrite.

Status: core business logic implemented — Location/Asset CRUD, the
append-only Movement ledger, and the Hold reservation lifecycle (create,
commit with partial fulfillment, release, expiry watchdog query), all
backed by `entitygraph.DataManager`. Tested against both an in-memory fake
and real Postgres (40 tests, `go test -race ./...` clean). See
[documentation/](documentation/) for the requirements/architecture write-up
and [documentation/3. implementation/todo_done.md](documentation/3.%20implementation/todo_done.md)
for what shipped. Not yet wired into any HTTP/gRPC layer, and the
actor/auth model is still an open question — see requirements.md.
