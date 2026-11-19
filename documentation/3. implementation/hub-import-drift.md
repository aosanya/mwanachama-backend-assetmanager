# Hub import drift

What stands between `mwanachama-backend-assetmanager` and the hub's in-tree copy (`projects/hub-steward/code/wakala-api/modules/assetmanager`) before the hub imports this module and deletes the copy. The method is `scripts/hub-copy-diff.sh` as [hub-copy-diff.md](hub-copy-diff.md) describes it: both trees, hub imports rewritten to this module's path, comments and layout removed, `diff -r`.

Run on 2026-11-19 against the working tree of the hub's `dev` branch. The module folder has no uncommitted change there; its last commit is `3a6759b3` (asset update refuses a save made on an old version). This repository was clean on `master` at `a8996b7`.

## Result of the run

The script exits 1. Besides 26 `Only in backend` lines, it reports:

| File | Side | Kind |
|---|---|---|
| `asset_version_test.go` | only in the hub | test |
| `routes/asset_version_test.go` | only in the hub | test |
| `asset.go` | differs | behaviour, both directions |
| `asset_manager.go` | differs | interface |
| `models/asset.go` | differs | model |
| `assetmanager.operations.json` | differs | error status table |
| `routes/routes.go` | differs | error mapping |
| `conflict.go` | differs | sibling only |

Every other file of the hub copy is identical to this repository's, apart from the import path. This repository carries every hub commit before `3a6759b3` (see [hub-copy-diff.md](hub-copy-diff.md), and `a8996b7` for the balance check).

## Only in the hub

| Item | What it is | Disposition |
|---|---|---|
| `asset_version_test.go` | Five tests of a version-checked asset update: the current version saves, no version saves as before, an old version is refused naming the changed field and writes nothing, the first changed field is named, a missing asset still says not found | Move with the code. The import path is rewritten. |
| `routes/asset_version_test.go` | Tool schema declares an optional `version`; the tool saves on the current version and on none; an old version is refused naming the field; the route answers 409 on an old version | Move with the code. The import path is rewritten. |
| `models.AssetUpdate` | The update payload: an asset's fields plus an optional `version` | Move. |
| `UpdateAsset` takes `models.AssetUpdate`, and `AssetManager.UpdateAsset` has that signature | The save is refused with `versioncheck.StaleError` when the sent `version` is not the stored `updated_at` to the second; the refusal names the first of name, category, serial tag and attributes that differs. An empty `version` saves as before. | Merge by hand into `asset.go`, next to this repository's serial tag handling, which the hub copy lacks. |
| `staleAssetUpdate` | The check above, built on `shared/versioncheck` | Move. |
| `ErrStale` in `assetmanager.operations.json` | The `ErrStale` refusal answers 409 | Move. |
| `ErrStale` in `routes/routes.go` | Maps the sentinel name to `dispatch.ByType(&versioncheck.StaleError{}, nil)` | Merge by hand into the sentinel map. |

`shared/versioncheck` exists in `mwanachama-backend-shared` and this module already resolves shared through a local `replace`, so no new dependency is needed.

## Only in this repository

| Item | Disposition |
|---|---|
| `violatesSerialTagIndex`, `namesSerialTagIndex` and the `serial_tag` constant in `conflict.go`, and their calls in `CreateAsset` and `UpdateAsset` (`ErrAssetSerialTagExists` on a serial tag race; [serial-tag-race.md](serial-tag-race.md)) | Keep. The hub copy goes when it imports this module, so the hub gains the behaviour. |
| Test files this repository carries and the hub copy never had (the 26 `Only in backend` lines: root tests, `routes/` tests, `mcp/resolve_test.go`, `spec/`) | Keep. |
| `cmd/`, `scripts/`, `documentation/`, `README.md`, `CLAUDE.md`, `Makefile`, `LICENSE` | Keep. Repository files, outside the module. |

## Differing beyond the import path

| Operation | This repository | Hub | Disposition |
|---|---|---|---|
| `UpdateAsset` | Takes `models.Asset`; no stale-version refusal; refuses a serial tag race with `ErrAssetSerialTagExists` | Takes `models.AssetUpdate`; refuses a stale version with 409; no serial tag race refusal | Not a conflict. The two changes touch different branches of the same function, so the union holds both. |

No operation has two conflicting behaviours, so there is no owner question.

## Hub wiring that is not part of the module

Not moved. It stays in the hub and changes only when the hub imports this module (todo 4 of the hub issue, import path rewrite).

| Hub file | What it does |
|---|---|
| `internal/api/http/assetmanager_routes.go` | Mounts the module's routes under `/instances/{instanceID}/assetmanager`, one handler per route shape, with the caller from the account context and the role permission check |
| `internal/api/http/mcp_assets_permissions.go` | Registers the module's tools on the hub's MCP server and guards each by its action |
| `internal/api/http/mcp.go`, `mcp_tool_actions.go` | Resolver for the instance's manager; the action list of the tool filter |
| `internal/api/http/change_permissions.go` | Admission check (`assetmanager.location.update`) and the change-needs-its-code gate |
| `internal/api/http/membership.go` | `assetManagerFor`, the manager lookup by instance |
| `internal/agencies/assetmanager_spec.go`, `assetmanager.wakala.json` | The hub's declaration of the module's objects, with the instance and mount set per agency |
| `internal/agencies/registry.go`, `caches.go` | Builds, provisions and pools one manager per instance |
| `modules/agency/models/modules_catalog.json`, `permissions_catalog.json` | The module in the catalogue and its permissions |
| `internal/accounts/router.go`, `modules/catalog/doc.go` | A reference to the module in routing and in documentation |
| `modules/shared/gormutil/gormutil.go` | A mention of the module; shared code, not the module's |

## Counts the union may move

`ErrStale` adds one sentinel to the route table's map and one status to the operations file. If a test of this repository pins the number of sentinels or error statuses it changes by one, recorded in the report of the merge.

## Merge result of 2026-11-19

Every "Only in the hub" row above is merged here. A new run of `scripts/hub-copy-diff.sh` reports only the two files that carry this repository's serial tag handling (`asset.go`, `conflict.go`) and no `Only in hub` line.

- Merged by hand: `asset.go`, `asset_manager.go`, `models/asset.go`, `routes/routes.go`, `assetmanager.operations.json`. The two test files were taken over with their import paths rewritten.
- `UpdateAsset` now takes `models.AssetUpdate`. The existing tests of this repository that passed a `models.Asset` build an `AssetUpdate` from it at the call (`asset_test.go`, `conflict_test.go`, `asset_serial_race_test.go`, `routes/a14_update_delete_race_test.go`). Their assertions are unchanged.
- `ErrStale` is declared in `shared/versioncheck`, not in this module's `errors.go`, so the count of 14 sentinels in [routes.md](../2.%20design/routes.md) is unchanged and the route table's map holds one more entry. No test pinned either number.
- `go test ./...` passes on in-memory SQLite, also with `-race`.
