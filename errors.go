package mwanachamaassetmanager

import "errors"

// ErrLocationNotFound is returned when a Location does not exist for the
// given agencyID and locationID combination.
var ErrLocationNotFound = errors.New("location not found")

// ErrInvalidLocation is returned when a Location is missing required
// fields, or names a parent that creates a cycle.
var ErrInvalidLocation = errors.New("invalid location: missing required fields or a parent cycle")

// ErrLocationNotEmpty is returned by [AssetManager.DeleteLocation] when the
// location still has child locations or assets currently denormalized to
// it. Delete or move those first.
var ErrLocationNotEmpty = errors.New("location has child locations or assets and cannot be deleted")

// ErrAssetNotFound is returned when an Asset does not exist for the given
// agencyID and assetID combination.
var ErrAssetNotFound = errors.New("asset not found")

// ErrInvalidAsset is returned when an Asset is missing required fields,
// carries an unrecognized TrackingMode, or (for a serialized Asset) an
// empty SerialTag.
var ErrInvalidAsset = errors.New("invalid asset: missing required fields")

// ErrAssetSerialTagExists is returned by [AssetManager.CreateAsset] when a
// serialized Asset's SerialTag is already in use by another non-deleted
// Asset in the same agency.
var ErrAssetSerialTagExists = errors.New("asset serial tag already in use")

// ErrAssetTrackingModeImmutable is returned by [AssetManager.UpdateAsset]
// when the caller attempts to change TrackingMode after creation — doing so
// would silently invalidate every prior Movement's balance semantics.
var ErrAssetTrackingModeImmutable = errors.New("asset tracking mode cannot be changed after creation")

// ErrAssetHasOpenHolds is returned by [AssetManager.DeleteAsset] when the
// asset has one or more Holds still in a non-terminal (reserved) state.
var ErrAssetHasOpenHolds = errors.New("asset has open holds and cannot be deleted")

// ErrMovementNotFound is returned when a Movement does not exist for the
// given agencyID and movementID combination.
var ErrMovementNotFound = errors.New("movement not found")

// ErrInvalidMovement is returned when a Movement's Kind/Quantity/location
// fields don't satisfy the shape that Kind requires — see [MovementKind]'s
// doc comments and PostMovement's validation.
var ErrInvalidMovement = errors.New("invalid movement: kind, quantity or location fields inconsistent")

// ErrMovementAlreadyReversed is returned by [AssetManager.ReverseMovement]
// when the target Movement already has a reversing entry on record — a
// Movement may be reversed at most once.
var ErrMovementAlreadyReversed = errors.New("movement already reversed")

// ErrHoldNotFound is returned when a Hold does not exist for the given
// agencyID and holdID combination.
var ErrHoldNotFound = errors.New("hold not found")

// ErrInvalidHold is returned when a Hold is missing required fields, has a
// non-positive Quantity, or an ExpiresAt that is not a valid future
// timestamp.
var ErrInvalidHold = errors.New("invalid hold: missing required fields or non-future expiry")

// ErrInvalidHoldStatusTransition is returned by [AssetManager.CommitHold]
// and [AssetManager.ReleaseHold] when the Hold is not in
// [HoldStatusReserved] — see [HoldStatus.CanTransitionTo].
var ErrInvalidHoldStatusTransition = errors.New("invalid hold status transition")

// ErrInsufficientAvailable is returned by [AssetManager.CreateHold] when the
// requested quantity exceeds what's available at the given (asset,
// location) pair once existing open holds are accounted for.
var ErrInsufficientAvailable = errors.New("insufficient available quantity")
