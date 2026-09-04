# mwanachama-backend-assetmanager (Go)

Open tasks only — 🚀 In Progress · 📋 Not Started · ⏸️ Blocked.
Everything else (completed rows, board context) is in [todo_done.md](todo_done.md).

Nothing open — see [todo_done.md](todo_done.md) for A1-A11. Decision #10
("no HTTP/gRPC layer of its own") was superseded 2026-09-04 by A11 (the
`routes/` package) at the user's explicit request; #11 actor identity and
#12 flexible-attribute mechanics stand as decided — see
[../1. requirements/requirements.md](../1.%20requirements/requirements.md)'s
decision table. Next work starts from a wiring decision (who mounts
`routes.Routes` first, and what auth gate it wraps the handlers in), not
from an open design question.
