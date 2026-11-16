# Hub copy diff

`mwanachama-backend-assetmanager` is the custodian of the assetmanager module. The hub (`projects/hub-steward/code/wakala-api/modules/assetmanager`) holds a copy made on 2026-11-04 and changed it afterwards. Every change made in that copy exists here. The copy has none of the changes made here afterwards (MUN684 and A25), so the two differ in `asset.go` and `conflict.go`, besides the files only this repository has (hub issue A24, the assetmanager part of ISS-105).

## Command

```
scripts/hub-copy-diff.sh [path-to-hub-modules-assetmanager]
```

The default path is `../projects/hub-steward/code/wakala-api/modules/assetmanager`, relative to this repository's parent folder. The exit code is 0 when every line of the `diff -r` output starts with `Only in backend`, and 1 when any line starts with `diff `, `Binary files ` or `Only in hub`. The script prints the whole report either way. The script and `scripts/hubdiff` are the ones of `mwanachama-backend-forms`, which generalised the one of `mwanachama-backend-git` (G78) to take the module path from `go.mod`.

## What it does

1. Copies the `*.go` and `*.json` files of both trees into two temporary folders, leaving out `cmd/`, `documentation/`, `scripts/` and `.git/`.
2. Rewrites the hub's imports: `wakalaserver/modules/shared/` becomes `github.com/aosanya/mwanachama-backend-shared/` and `wakalaserver/modules/assetmanager` becomes `github.com/aosanya/mwanachama-backend-assetmanager`.
3. Runs every Go file on both sides through `scripts/hubdiff`, which parses it, drops all comments and prints it with `gofmt`, so comments and layout do not show as differences.
4. Runs `diff -r backend hub`, prints the report, and sets the exit code by the rule above. A changed `.go` or `.json` file, a file only the hub has, and a binary difference exit 1; files only this repository has (its tests and `spec/`) do not.

## Result of 2026-11-16

Run at d0ef4d9 against the working tree of the hub's `dev` branch, where the module folder has no uncommitted change and its last commit is 6863707a. The exit code is 1. Every file the hub has exists here. Two of them differ, and nothing in the output is `Only in hub`. The rest is `Only in backend` lines.

### The two diffs: `asset.go` and `conflict.go`

Both differ on purpose and are not an error to fix here. They are the changes made in this repository after the hub copy was last changed (6863707a), in two commits:

| Commit here | Change | Files |
|---|---|---|
| 8ce7a49 (MUN684) | A serialized asset created or renamed onto a serial tag that another request took at the same moment is refused as `ErrAssetSerialTagExists`, not as an internal error | `asset.go` |
| d0ef4d9 (A25) | That refusal is given only when the violated constraint is the `serial_tag` index, so a collision on the `code` index is not blamed on the serial tag | `asset.go`, `conflict.go` |

The page `serial-tag-race.md` holds the reasoning. The hunks, as the script prints them (`<` is this repository):

```
diff asset.go hub/asset.go
57,59d56
< 		if m.violatesSerialTagIndex(err) {
< 			return models.Asset{}, ErrAssetSerialTagExists
< 		}
89,91d85
< 		if m.violatesSerialTagIndex(written.Error) {
< 			return models.Asset{}, ErrAssetSerialTagExists
< 		}
diff conflict.go hub/conflict.go
6,7d5
< 	"github.com/aosanya/mwanachama-backend-shared/gormutil"
< 	"github.com/aosanya/mwanachama-backend-shared/spec"
11,12d8
< const serialTagIndex = "serial_tag"
< 
35,57d30
< }
< func (m *assetManager) violatesSerialTagIndex(err error) bool {
< 	taken := false
< 	gormutil.ClassifyError(err, func(string) error {
< 		return err
< 	}, func(violated string) error {
< 		taken = m.namesSerialTagIndex(violated)
< 		return err
< 	})
< 	return taken
< }
< func (m *assetManager) namesSerialTagIndex(violated string) bool {
< 	if cut := strings.Index(violated, " ("); cut >= 0 {
< 		violated = violated[:cut]
< 	}
< 	violated = strings.TrimSpace(violated)
< 	if violated == "" {
< 		return false
< 	}
< 	asset := m.store.Object(roleAsset)
< 	postgresName := m.store.Spec().IndexFor(asset, spec.Index{Name: serialTagIndex})
< 	sqliteName := m.store.Table(roleAsset) + "." + serialTagIndex
< 	return violated == postgresName || violated == sqliteName
```

The hub copy is left as it is: ISS-105 deletes it and makes the hub import this module, so a change there would be thrown away. Until then the script exits 1 and this section is the explanation. Whether the hub copy takes the change in the meantime is the owner's call (hub issue A27).

The report at d0ef4d9 is these two `diff` blocks and 26 `Only in backend` lines.

### The `Only in backend` lines

All are test files the hub copy never carried or that were written here:

| Difference | Reason |
|---|---|
| Only here: `asset_serial_race_test.go`, `asset_test.go`, `asset_unique_violation_test.go`, `blueprint_guard_test.go`, `blueprint_test.go`, `conflict_test.go`, `domain_agnostic_test.go`, `examples_test.go`, `hold_slot_test.go`, `hold_test.go`, `location_test.go`, `movement_test.go`, `postgres_integration_test.go`, `testdb_test.go`, `mcp/resolve_test.go` | The hub copy of 2026-11-04 was made without most test files. This repository kept them. They test code that is identical in both. |
| Only here: `routes/` tests `a12_caller_supplied_id_test.go`, `a13_hold_commit_release_race_test.go`, `a14_update_delete_race_test.go`, `asset_test.go`, `hold_test.go`, `location_test.go`, `movement_test.go`, `routes_test.go`, `testdb_test.go` | Same reason. After the caller-signing change they mount a session caller (`signedBy` in `routes/testdb_test.go`), and `TestReverseMovement_MissingPerformedBy` mounts none. |
| Only here: `routes/list_page_test.go` | The red test written for hub 067f672b. The hub commit tested paging in `wakala-api/internal/api/http/misuse_assetlistpage_test.go`, outside the module. |
| Only here: `spec/` | The example declaration `spec/examples/warehouse.assetmanager.json`, which the tests in this repository load. The hub never had it. |

## The hub commits carried over

One commit here per hub commit, each naming the hub hash: f2f44c31, c10da6e4, da677484, 067f672b, ea7a0444, 6863707a.

Files a commit touched lost their comments (the repository rule is no comments in code).

Two tests need a Postgres and skip without one: `hold_postgres_race_test.go` (hub c10da6e4) needs `DATABASE_URL` pointing at localhost, and the Postgres leg of `movement_reverse_race_test.go` (hub da677484) needs `ASSETMANAGER_REVERSE_PG`. The SQLite leg of the reversal test runs everywhere and was red before the change.
