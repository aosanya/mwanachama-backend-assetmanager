# mwanachama-backend-assetmanager (Go)

Open tasks only — 🚀 In Progress · 📋 Not Started · ⏸️ Blocked.
Everything else (completed rows, board context) is in [todo_done.md](todo_done.md).

| Task | Title | Status | Notes |
|------|-------|--------|-------|
| A22 | 📋 **Promote `Asset.AttributesJSON` to a declared `json` column.** Requirements decision #12 left this deliberately deferred, and the 2026-09-30 declared-domain conversion (A16) kept it as `text` for the same reason rather than doing it as a side effect of a storage change — this repo's own `CLAUDE.md` warns against conflating "the engine makes jsonb available" with "this is the time to promote it". What it would buy: `jsonb` on Postgres, so a domain could index and filter *inside* the attribute bag by path the way catalog's `Entry.Doc` does, which is the whole point of the bag. What it costs, and why it is its own task rather than a one-line spec edit: **nothing validates the field today**, so an arbitrary non-JSON string is storable, and `jsonb` would start refusing those writes — any existing caller sending junk breaks at the write rather than at review. The move is therefore: declare `matches: "json_object"` first (the pattern registry is already there in `patterns.go`), let that refuse bad values at the module boundary with `ErrInvalidAsset`, and only then change the declared type to `json`. `specstore`'s codec already stores a Go `string` into a json column as-is and reads it back unchanged, so `models.Asset.AttributesJSON` does not change type and no caller signature moves. | 📋 | — |
| A23 | 📋 **No SQL mirror for this module in the gateway's active `migrations/`.** Every GORM-backed domain is supposed to have a hand-maintained SQL mirror under `mwanachama-backend-api-gateway/internal/store/postgres/migrations/` so `cmd/migrate up` alone provisions a fresh database. This module has none — and correctly so today, because **nothing mounts it**: `cmd/server/modules.go:16` says "assetmanager has no import into this gateway", and `mwanachama-wakala-api`'s `go.mod` does not require it either. Writing a mirror now would mean adding the module to the gateway, which is a wiring decision nobody has made. Recorded so the debt is tracked rather than assumed: **whoever first mounts this module owes the mirror in the same change.** `cmd/ddl <spec.json> postgres` prints exactly the statements it needs, so the mirror is a copy of reviewed output rather than hand-written SQL. | 📋 | — |

Decision #10
("no HTTP/gRPC layer of its own") was superseded 2026-09-04 by A11 (the
`routes/` package) at the user's explicit request; #11 actor identity and
#12 flexible-attribute mechanics stand as decided — see
[../1. requirements/requirements.md](../1.%20requirements/requirements.md)'s
decision table. Next work starts from a wiring decision (who mounts
`routes.Routes` first, and what auth gate it wraps the handlers in), not
from an open design question.
