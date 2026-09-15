// hold.go — HTTP routes over hold.go's Hold CRUD and lifecycle: CreateHold,
// GetHold, CommitHold, ReleaseHold, ListHolds, plus hold_watchdog.go's
// ListHoldsExpiredAsOf. See doc.go for scope.
package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// holdStatusTable maps this package's Hold error sentinels to a status
// code. ErrAssetNotFound/ErrLocationNotFound are included because
// CreateHold returns them raw (not wrapped in ErrInvalidHold) when the
// referenced asset or location doesn't exist — see hold.go's CreateHold.
// ErrInvalidMovement is included because CommitHold posts a Movement
// internally and returns that error unwrapped when its own kind check
// fails.
var holdStatusTable = map[error]int{
	mwanachamaassetmanager.ErrHoldNotFound:                http.StatusNotFound,
	mwanachamaassetmanager.ErrAssetNotFound:               http.StatusNotFound,
	mwanachamaassetmanager.ErrLocationNotFound:            http.StatusNotFound,
	mwanachamaassetmanager.ErrInvalidHoldStatusTransition: http.StatusConflict,
	mwanachamaassetmanager.ErrInsufficientAvailable:       http.StatusConflict,
	mwanachamaassetmanager.ErrInvalidHold:                 http.StatusBadRequest,
	mwanachamaassetmanager.ErrInvalidMovement:             http.StatusBadRequest,
}

func writeHoldErr(w http.ResponseWriter, err error) {
	code := httpwire.StatusFor(err, holdStatusTable, http.StatusInternalServerError)
	if code == http.StatusInternalServerError {
		httpwire.WriteErr(w, code, "internal error")
		return
	}
	httpwire.WriteErr(w, code, err.Error())
}

// CreateHold handles POST — decode, create, encode.
func CreateHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mwanachamaassetmanager.Hold
		if err := httpwire.ReadJSON(r, &in); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.CreateHold(r.Context(), in)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusCreated, out)
	}
}

// GetHold handles GET {holdID}.
func GetHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.GetHold(r.Context(), r.PathValue("holdID"))
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// commitHoldBody is the wire shape for CommitHold.
type commitHoldBody struct {
	Quantity     int64                               `json:"quantity"`
	Kind         mwanachamaassetmanager.MovementKind `json:"kind"`
	ToLocationID string                              `json:"to_location_id"`
	PerformedBy  string                              `json:"performed_by"`
}

// commitHoldResponse is CommitHold's two return values, encoded together —
// a caller needs both the hold's post-commit state and the movement it
// posted.
type commitHoldResponse struct {
	Hold     mwanachamaassetmanager.Hold     `json:"hold"`
	Movement mwanachamaassetmanager.Movement `json:"movement"`
}

// CommitHold handles POST {holdID}/commit — decode, commit, encode both
// the updated Hold and the Movement it posted.
func CommitHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body commitHoldBody
		if err := httpwire.ReadJSON(r, &body); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		hold, movement, err := am.CommitHold(r.Context(), r.PathValue("holdID"), body.Quantity, body.Kind, body.ToLocationID, body.PerformedBy)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, commitHoldResponse{Hold: hold, Movement: movement})
	}
}

// ReleaseHold handles POST {holdID}/release.
func ReleaseHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.ReleaseHold(r.Context(), r.PathValue("holdID"))
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// ListHolds handles GET — optionally filtered by the query params
// asset_id, location_id, and status.
func ListHolds(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := mwanachamaassetmanager.HoldFilter{
			AssetID:    q.Get("asset_id"),
			LocationID: q.Get("location_id"),
			Status:     mwanachamaassetmanager.HoldStatus(q.Get("status")),
		}
		out, err := am.ListHolds(r.Context(), filter)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// ListHoldsExpiredAsOf handles GET /expired?cutoff=<RFC 3339> — the
// watchdog query hold_watchdog.go exposes. cutoff is required; there is no
// sensible default (a sweeper always has its own idea of "now" to pass, and
// defaulting to the server's clock here would hide that choice).
func ListHoldsExpiredAsOf(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cutoff := r.URL.Query().Get("cutoff")
		if cutoff == "" {
			httpwire.WriteErr(w, http.StatusBadRequest, "cutoff is required")
			return
		}
		out, err := am.ListHoldsExpiredAsOf(r.Context(), cutoff)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}
