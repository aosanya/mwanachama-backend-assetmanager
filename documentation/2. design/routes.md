# Routes

The 22 addresses this module answers are declared in
`assetmanager.operations.json` and built by
`mwanachama-backend-shared/dispatch`. There are no hand-written handlers:
adding an address is an edit to that file, not a new Go function.

Converted 2026-09-30 (A17). The twelve-or-so decode-call-encode handlers
across `routes/asset.go`, `hold.go`, `location.go` and `movement.go` — 541
lines — are gone, and the route tests passed against the declared table,
which was the acceptance.

## The table

| Method | Path | Action |
| ------ | ---- | ------ |
| POST | `/locations` | `assetmanager.location.create` |
| GET | `/locations` | `assetmanager.location.list` |
| GET | `/locations/{locationID}` | `assetmanager.location.read` |
| PATCH | `/locations/{locationID}` | `assetmanager.location.update` |
| DELETE | `/locations/{locationID}` | `assetmanager.location.delete` |
| GET | `/locations/{locationID}/descendants` | `assetmanager.location.descendants` |
| POST | `/assets` | `assetmanager.asset.create` |
| GET | `/assets` | `assetmanager.asset.list` |
| GET | `/assets/{assetID}` | `assetmanager.asset.read` |
| PATCH | `/assets/{assetID}` | `assetmanager.asset.update` |
| DELETE | `/assets/{assetID}` | `assetmanager.asset.delete` |
| GET | `/assets/{assetID}/balance` | `assetmanager.asset.balance` |
| POST | `/movements` | `assetmanager.movement.post` |
| GET | `/movements` | `assetmanager.movement.list` |
| GET | `/movements/{movementID}` | `assetmanager.movement.read` |
| POST | `/movements/{movementID}/reverse` | `assetmanager.movement.reverse` |
| POST | `/holds` | `assetmanager.hold.create` |
| GET | `/holds` | `assetmanager.hold.list` |
| GET | `/holds/expired` | `assetmanager.hold.expired` |
| GET | `/holds/{holdID}` | `assetmanager.hold.read` |
| POST | `/holds/{holdID}/commit` | `assetmanager.hold.commit` |
| POST | `/holds/{holdID}/release` | `assetmanager.hold.release` |

`TestTheDeclaredTableAnswersExactlyTheAddressesItAlwaysDid` holds this list
and the built table to each other, so an address cannot move without the
change being deliberate.

## The gate is data

`AnonymousActions` is an allowlist of action ids, and it is **empty**. This
module has no auth model — a mount wraps whatever gate it wants around the
routes it is handed — so every address arrives gated and a mount names what
it wants to open.

It names what is public, never what is protected. An operation added to the
spec later and not named there arrives gated, so the failure direction is a
401 rather than the whole table on the public internet.
`TestNothingIsPublicUntilSomethingSaysSo` pins the current state, and
`TestEveryAnonymousActionNamesADeclaredOperation` catches an allowlisted id
that no operation declares any more.

## Errors

`operations.json`'s `errors` map gives every one of the 14 sentinels in
`errors.go` a status. An unmapped sentinel is redacted to a 500 with
`"internal error"`, which is catalog's CAT7 — a 400-shaped refusal arriving
as an unexplained 500 — so `TestEverySuppliedSentinelHasADeclaredStatus`
holds the two sets together.

| Status | Sentinels |
| ------ | --------- |
| 404 | `ErrLocationNotFound`, `ErrAssetNotFound`, `ErrMovementNotFound`, `ErrHoldNotFound` |
| 409 | `ErrLocationNotEmpty`, `ErrAssetTrackingModeImmutable`, `ErrAssetHasOpenHolds`, `ErrMovementAlreadyReversed`, `ErrInvalidHoldStatusTransition`, `ErrInsufficientAvailable` |
| 400 | `ErrInvalidLocation`, `ErrInvalidAsset`, `ErrAssetSerialTagExists`, `ErrInvalidMovement`, `ErrInvalidHold` |

Before the conversion each aggregate had its own table. Folding them into
one was safe because the three sentinels that appeared in more than one
(`ErrAssetNotFound`, `ErrLocationNotFound`, `ErrInvalidMovement`) carried
the same status in each.

## The address outranks the body

`PATCH /assets/{assetID}` and `PATCH /locations/{locationID}` bind
`{"from": "path", "into": "ID"}`, applied after the body is decoded. Without
it a caller could `PATCH /assets/a` with a body claiming `b`.

`routes/a12_caller_supplied_id_test.go` covers the create side of the same
property: an id in a create body is never honoured, because the server mints
it.

## `required` is not a gate

`dispatch.Arg.Required` is read by the MCP tool schema, not by the HTTP
path — an absent required argument binds to the zero value and the call
proceeds. That is filed as shared's **S35**, where the decision belongs,
because `mwanachama-backend-permissions` depends on the other reading.

Until it is decided, this module refuses in its own validate:
`GetAssetBalance` refuses an empty location with `ErrInvalidAsset`, and
`ListHoldsExpiredAsOf` refuses an unparseable cutoff with `ErrInvalidHold`.
Both were handler pre-checks before, and both would otherwise have become a
200 with a meaningless zero and a 500 respectively.

## Wire changes the conversion accepted

Both widenings — nothing that succeeded before now fails.

- **A PATCH body decodes the whole model.** The deleted handlers used
  narrow `assetEditBody`/`locationEditBody` structs, and `httpwire.ReadJSON`
  refuses unknown fields, so `tracking_mode` in a PATCH body was a 400. The
  dispatcher binds a whole struct to the method's parameter and has no way
  to decode a subset of its keys, so six more keys are now accepted. Five
  are inert — `UpdateAsset` ignores them — and `tracking_mode` now answers
  409 `ErrAssetTrackingModeImmutable` where it was refused outright.
- **`root_only` parses as a boolean.** It was `q.Get("root_only") ==
  "true"`, so `root_only=1` was silently false. It now parses, which means
  `1` works and garbage is a 400 rather than a silent false.

## No MCP surface

This module registers no MCP tools, so there is no `mcp/` package and no
`routes/tools.go`. If one is ever wanted, it comes from `dispatch.Tools`
over this same operations file — one route table, two surfaces — not from
hand-registered tool functions.
