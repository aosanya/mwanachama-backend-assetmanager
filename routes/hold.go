// hold.go — HTTP routes over hold.go's Hold CRUD and lifecycle: CreateHold,
// GetHold, CommitHold, ReleaseHold, ListHolds, plus hold_watchdog.go's
// ListHoldsExpiredAsOf. See doc.go for scope.
package routes

import (
	"errors"
	"net/http"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// holdStatusFor maps this package's Hold error sentinels to a status code.
// ErrAssetNotFound/ErrLocationNotFound are included because CreateHold
// returns them raw (not wrapped in ErrInvalidHold) when the referenced
// asset or location doesn't exist — see hold.go's CreateHold.
// ErrInvalidMovement is included because CommitHold posts a Movement
// internally and returns that error unwrapped when its own kind check
// fails.
func holdStatusFor(err error) int {
	switch {
	case errors.Is(err, mwanachamaassetmanager.ErrHoldNotFound),
		errors.Is(err, mwanachamaassetmanager.ErrAssetNotFound),
		errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound):
		return http.StatusNotFound
	case errors.Is(err, mwanachamaassetmanager.ErrInvalidHoldStatusTransition),
		errors.Is(err, mwanachamaassetmanager.ErrInsufficientAvailable):
		return http.StatusConflict
	case errors.Is(err, mwanachamaassetmanager.ErrInvalidHold),
		errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeHoldErr(w http.ResponseWriter, err error) {
	code := holdStatusFor(err)
	if code == http.StatusInternalServerError {
		writeErr(w, code, "internal error")
		return
	}
	writeErr(w, code, err.Error())
}

// CreateHold handles POST — decode, create, encode.
func CreateHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in models.Hold
		if err := readJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.CreateHold(r.Context(), in)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
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
		writeJSON(w, http.StatusOK, out)
	}
}

// commitHoldBody is the wire shape for CommitHold.
type commitHoldBody struct {
	Quantity     int64               `json:"quantity"`
	Kind         models.MovementKind `json:"kind"`
	ToLocationID string              `json:"to_location_id"`
	PerformedBy  string              `json:"performed_by"`
}

// commitHoldResponse is CommitHold's two return values, encoded together —
// a caller needs both the hold's post-commit state and the movement it
// posted.
type commitHoldResponse struct {
	Hold     models.Hold     `json:"hold"`
	Movement models.Movement `json:"movement"`
}

// CommitHold handles POST {holdID}/commit — decode, commit, encode both
// the updated Hold and the Movement it posted.
func CommitHold(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body commitHoldBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		hold, movement, err := am.CommitHold(r.Context(), r.PathValue("holdID"), body.Quantity, body.Kind, body.ToLocationID, body.PerformedBy)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, commitHoldResponse{Hold: hold, Movement: movement})
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
		writeJSON(w, http.StatusOK, out)
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
			Status:     models.HoldStatus(q.Get("status")),
		}
		out, err := am.ListHolds(r.Context(), filter)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
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
			writeErr(w, http.StatusBadRequest, "cutoff is required")
			return
		}
		out, err := am.ListHoldsExpiredAsOf(r.Context(), cutoff)
		if err != nil {
			writeHoldErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
