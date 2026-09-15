// movement.go — HTTP routes over movement.go's append-only ledger:
// PostMovement, GetMovement, ListMovements, ReverseMovement. See doc.go for
// scope.
package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// movementStatusTable maps this package's Movement error sentinels to a
// status code. ErrAssetNotFound/ErrLocationNotFound are included because
// PostMovement returns them raw (not wrapped in ErrInvalidMovement) when
// the referenced asset itself doesn't exist — see movement.go's
// PostMovement.
var movementStatusTable = map[error]int{
	mwanachamaassetmanager.ErrMovementNotFound:        http.StatusNotFound,
	mwanachamaassetmanager.ErrAssetNotFound:           http.StatusNotFound,
	mwanachamaassetmanager.ErrLocationNotFound:        http.StatusNotFound,
	mwanachamaassetmanager.ErrMovementAlreadyReversed: http.StatusConflict,
	mwanachamaassetmanager.ErrInvalidMovement:         http.StatusBadRequest,
}

func writeMovementErr(w http.ResponseWriter, err error) {
	code := httpwire.StatusFor(err, movementStatusTable, http.StatusInternalServerError)
	if code == http.StatusInternalServerError {
		httpwire.WriteErr(w, code, "internal error")
		return
	}
	httpwire.WriteErr(w, code, err.Error())
}

// PostMovement handles POST — decode, post, encode.
func PostMovement(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mwanachamaassetmanager.Movement
		if err := httpwire.ReadJSON(r, &in); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.PostMovement(r.Context(), in)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusCreated, out)
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
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// ListMovements handles GET — optionally filtered by the query params
// asset_id and kind.
func ListMovements(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := mwanachamaassetmanager.MovementFilter{
			AssetID: q.Get("asset_id"),
			Kind:    mwanachamaassetmanager.MovementKind(q.Get("kind")),
		}
		out, err := am.ListMovements(r.Context(), filter)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
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
		if err := httpwire.ReadJSON(r, &body); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.ReverseMovement(r.Context(), r.PathValue("movementID"), body.PerformedBy, body.Note)
		if err != nil {
			writeMovementErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusCreated, out)
	}
}
