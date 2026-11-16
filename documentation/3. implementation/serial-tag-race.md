# Serial tag race

`CreateAsset` checks a serialized asset's tag with a count and then inserts in a second statement. Two creates sent at the same moment both pass the count, so the check cannot be the only guard. The unique index on `serial_tag` (live rows with a non-empty tag) is the guard that decides.

The loser of that race gets a database unique violation from the insert: `UNIQUE constraint failed ... serial_tag (2067)` on SQLite, SQLSTATE 23505 on Postgres. Before MUN684 it was wrapped as a plain error and callers saw `internal error`.

## Answer

`CreateAsset` and `UpdateAsset` map a unique violation to `ErrAssetSerialTagExists`, the refusal the in-a-row check already gives, only when the violated constraint is the `serial_tag` index. The route table answers it with the status `assetmanager.operations.json` declares for that sentinel (400 today), the same for the loser and for a duplicate sent later. Moving a tag onto another asset with `UpdateAsset` hit the same index and is mapped the same way.

The asset table has a second unique constraint, on `code`. A violation of it (the code counter went backwards after a restore or an import) is not a serial tag problem and is not answered as one: it keeps its own error, wrapped as `CreateAsset: ...`, which the route table answers as an internal error. It points at a data problem, so it stays visible.

The violated constraint comes from `gormutil.ClassifyError`, which hands it to its conflict callback. `violatesSerialTagIndex` in `conflict.go` takes it from there and `namesSerialTagIndex` compares it with the two names the asset table's serial tag index has:

| Database | What the driver names | Compared with |
|---|---|---|
| SQLite | `<asset table>.serial_tag`, followed by ` (2067)` which is cut off | asset table + `.serial_tag` |
| Postgres | the index name (`PgError.ConstraintName`) | `IndexFor` of the asset object with the index `serial_tag`, which the spec guarantees is at most 63 bytes, so the database does not cut it |

Nothing imports pgx and nothing in shared changed. The open owner question whether `ErrAssetSerialTagExists` answers 400 or 409 is not decided here.

## Tests

`asset_serial_race_test.go` sends eight creates of one tag at once over two managers on one SQLite file, six rounds, and over a single-connection in-memory store, and requires one created asset and every other answer to be `ErrAssetSerialTagExists`. A second test moves one asset's tag onto another's.

`asset_unique_violation_test.go` sets the asset code counter back to 1 and creates a fungible and then a serialized asset: both fail on the `code` index and neither is answered `ErrAssetSerialTagExists`. It also pins that a second create of one tag still is, and checks `namesSerialTagIndex` on the SQLite and Postgres names of the serial tag index, the code constraint and the id. The Postgres variant runs through `reverseRaceDatabases`, so it runs only when `ASSETMANAGER_REVERSE_PG` is set and is skipped otherwise.

The hub copy under `wakala-api/modules/assetmanager` needs the same change until ISS-105 removes it.
