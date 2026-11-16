# Hub copy diff

`mwanachama-backend-assetmanager` is the custodian of the assetmanager module. The hub (`projects/hub-steward/code/wakala-api/modules/assetmanager`) holds a copy made on 2026-11-04 and changed it afterwards. Every change made in that copy exists here, so the two differ by the import path only (hub issue A24, the assetmanager part of ISS-105).

## Command

```
scripts/hub-copy-diff.sh [path-to-hub-modules-assetmanager]
```

The default path is `../projects/hub-steward/code/wakala-api/modules/assetmanager`, relative to this repository's parent folder. The exit code is 1 when the trees differ, 0 when they do not. The script and `scripts/hubdiff` are the ones of `mwanachama-backend-forms`, which generalised the one of `mwanachama-backend-git` (G78) to take the module path from `go.mod`.

## What it does

1. Copies the `*.go` and `*.json` files of both trees into two temporary folders, leaving out `cmd/`, `documentation/`, `scripts/` and `.git/`.
2. Rewrites the hub's imports: `wakalaserver/modules/shared/` becomes `github.com/aosanya/mwanachama-backend-shared/` and `wakalaserver/modules/assetmanager` becomes `github.com/aosanya/mwanachama-backend-assetmanager`.
3. Runs every Go file on both sides through `scripts/hubdiff`, which parses it, drops all comments and prints it with `gofmt`, so comments and layout do not show as differences.
4. Runs `diff -r backend hub`.

## Result of 2026-11-16

Every file the hub has exists here and is identical after the rewrite. Nothing in the output is `diff` or `Only in hub`. What remains is `Only in backend` lines:

| Difference | Reason |
|---|---|
| Only here: `asset_test.go`, `blueprint_guard_test.go`, `blueprint_test.go`, `conflict_test.go`, `domain_agnostic_test.go`, `examples_test.go`, `hold_slot_test.go`, `hold_test.go`, `location_test.go`, `movement_test.go`, `postgres_integration_test.go`, `testdb_test.go`, `mcp/resolve_test.go` | The hub copy of 2026-11-04 was made without most test files. This repository kept them. They test code that is identical in both. |
| Only here: `routes/` tests `a12_caller_supplied_id_test.go`, `a13_hold_commit_release_race_test.go`, `a14_update_delete_race_test.go`, `asset_test.go`, `hold_test.go`, `location_test.go`, `movement_test.go`, `routes_test.go`, `testdb_test.go` | Same reason. After the caller-signing change they mount a session caller (`signedBy` in `routes/testdb_test.go`), and `TestReverseMovement_MissingPerformedBy` mounts none. |
| Only here: `routes/list_page_test.go` | The red test written for hub 067f672b. The hub commit tested paging in `wakala-api/internal/api/http/misuse_assetlistpage_test.go`, outside the module. |
| Only here: `spec/` | The example declaration `spec/examples/warehouse.assetmanager.json`, which the tests in this repository load. The hub never had it. |

## The hub commits carried over

One commit here per hub commit, each naming the hub hash: f2f44c31, c10da6e4, da677484, 067f672b, ea7a0444, 6863707a.

Files a commit touched lost their comments (the repository rule is no comments in code).

Two tests need a Postgres and skip without one: `hold_postgres_race_test.go` (hub c10da6e4) needs `DATABASE_URL` pointing at localhost, and the Postgres leg of `movement_reverse_race_test.go` (hub da677484) needs `ASSETMANAGER_REVERSE_PG`. The SQLite leg of the reversal test runs everywhere and was red before the change.
