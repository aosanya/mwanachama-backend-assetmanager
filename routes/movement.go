// movement.go — HTTP routes over movement.go's append-only ledger:
// PostMovement, GetMovement, ListMovements, ReverseMovement. See doc.go for
// scope.
package routes

import (
	"errors"
	"net/http"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// movementStatusFor maps this package's Movement error sentinels to a
// status code. ErrAssetNotFound/ErrLocationNotFound are included because
// PostMovement returns them raw (not wrapped in ErrInvalidMovement) when
// the referenced asset itself doesn't exist — see movement.go's
// PostMovement.
func movementStatusFor(err error) int {
	switch {
	case errors.Is(err, mwanachamaassetmanager.ErrMovementNotFound),
		errors.Is(err, mwanachamaassetmanager.ErrAssetNotFound),
		errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound):
		return http.StatusNotFound
	case errors.Is(err, mwanachamaassetmanager.ErrMovementAlreadyReversed):
		return http.StatusConflict
	case errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeMovementErr(w http.ResponseWriter, err error) {
	code := movementStatusFor(err)
	if code == http.StatusInternalServerError {
		writeErr(w, code, "internal error")
		return
	}
	writeErr(w, code, err.Error())
}

// PostMovement handles POST — decode, post, encode.
func PostMovement(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in models.Movement
		if err := readJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.PostMovement(r.Context(), in)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// GetMovement handles GET {movementID}.
func GetMovement(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.GetMovement(r.Context(), r.PathValue("movementID"))
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ListMovements handles GET — optionally filtered by the query params
// asset_id and kind.
func ListMovements(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := mwanachamaassetmanager.MovementFilter{
			AssetID: q.Get("asset_id"),
			Kind:    models.MovementKind(q.Get("kind")),
		}
		out, err := am.ListMovements(r.Context(), filter)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// reverseMovementBody is the wire shape for ReverseMovement.
type reverseMovementBody struct {
	PerformedBy string `json:"performed_by"`
	Note        string `json:"note"`
}

// ReverseMovement handles POST {movementID}/reverse — decode, reverse,
// encode. performed_by is required (see ReverseMovement's own validation);
// no caller-identity gate here decides who performed_by names, per
// decision #11 — it is exactly the string the request body sends.
func ReverseMovement(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body reverseMovementBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.ReverseMovement(r.Context(), r.PathValue("movementID"), body.PerformedBy, body.Note)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}
