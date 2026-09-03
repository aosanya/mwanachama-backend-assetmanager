# mwanachama-backend-assetmanager — documentation

## Layout

Four folders, in SDLC order, and everything lives under one of them.

| Folder | What's inside |
|--------|---------------|
| [1. requirements/](1.%20requirements/) | Problem, vision and scope decisions for a general-purpose asset/inventory backend. |
| [2. design/](2.%20design/) | The Asset/Location/Movement/Hold graph schema and how it maps onto `mwanachama-backend-shared`'s entity-graph store. |
| [3. implementation/](3.%20implementation/) | The work: `todo.md` (open board), `todo_done.md` (completed rows + board context). |
| [4. qa/](4.%20qa/) | Test coverage and results — see `todo_done.md`'s A8 for what's covered; no separate QA doc yet. |

## What this repo is

A standalone, general-purpose asset-tracking backend — warehouses,
supermarkets, household inventories, livestock — modelled on
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager)'s
architecture and built on
[mwanachama-backend-shared](../mwanachama-backend-shared)'s Postgres
entity-graph store. Not part of the Mwanachama party platform's tier/RLS
model; no DSN board. Designed so it can eventually absorb the party's
existing merchandise domain without a schema rewrite — see
[1. requirements/requirements.md](1.%20requirements/requirements.md) for why.

Currently: core Location/Asset/Movement/Hold business logic implemented and
tested (unit tests + Postgres integration tests, see `todo_done.md`'s A8).
No HTTP/gRPC layer of its own by design (decision #10) — imported directly
as a Go package, same as `mwanachama-backend-taskmanager` — and not yet
imported by any specific caller. The actor/auth model is the remaining
open question in `1. requirements/requirements.md`.
