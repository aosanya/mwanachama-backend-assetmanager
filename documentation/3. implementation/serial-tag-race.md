# Serial tag race

`CreateAsset` checks a serialized asset's tag with a count and then inserts in a second statement. Two creates sent at the same moment both pass the count, so the check cannot be the only guard. The unique index on `serial_tag` (live rows with a non-empty tag) is the guard that decides.

The loser of that race gets a database unique violation from the insert: `UNIQUE constraint failed ... serial_tag (2067)` on SQLite, SQLSTATE 23505 on Postgres. Before MUN684 it was wrapped as a plain error and callers saw `internal error`.

## Answer

`CreateAsset` and `UpdateAsset` map a unique violation to `ErrAssetSerialTagExists`, the refusal the in-a-row check already gives. The route table answers it with the status `assetmanager.operations.json` declares for that sentinel (400 today), the same for the loser and for a duplicate sent later. Moving a tag onto another asset with `UpdateAsset` hit the same index and is mapped the same way.

The mapping uses `gormutil.IsUniqueViolation`, which reads both the SQLite text and the Postgres code, as `ReverseMovement` does.

## Tests

`asset_serial_race_test.go` sends eight creates of one tag at once over two managers on one SQLite file, six rounds, and over a single-connection in-memory store, and requires one created asset and every other answer to be `ErrAssetSerialTagExists`. A second test moves one asset's tag onto another's. The Postgres variant runs through `reverseRaceDatabases`, so it runs only when `ASSETMANAGER_REVERSE_PG` is set and is skipped otherwise.

The hub copy under `wakala-api/modules/assetmanager` needs the same change until ISS-105 removes it.
