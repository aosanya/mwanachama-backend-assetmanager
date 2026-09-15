// asset.go — HTTP routes over asset.go's Asset CRUD: CreateAsset, GetAsset,
// UpdateAsset, DeleteAsset, ListAssets, plus GetAssetBalance (movement.go's
// ledger fold, nested here under the asset it reads). See doc.go for scope.
package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// assetStatusTable maps this package's Asset error sentinels to a status
// code.
var assetStatusTable = map[error]int{
	mwanachamaassetmanager.ErrAssetNotFound:              http.StatusNotFound,
	mwanachamaassetmanager.ErrLocationNotFound:           http.StatusNotFound,
	mwanachamaassetmanager.ErrAssetTrackingModeImmutable: http.StatusConflict,
	mwanachamaassetmanager.ErrAssetHasOpenHolds:          http.StatusConflict,
	mwanachamaassetmanager.ErrInvalidAsset:               http.StatusBadRequest,
	mwanachamaassetmanager.ErrAssetSerialTagExists:       http.StatusBadRequest,
}

func writeAssetErr(w http.ResponseWriter, err error) {
	code := httpwire.StatusFor(err, assetStatusTable, http.StatusInternalServerError)
	if code == http.StatusInternalServerError {
		httpwire.WriteErr(w, code, "internal error")
		return
	}
	httpwire.WriteErr(w, code, err.Error())
}

// CreateAsset handles POST — decode, create, encode.
func CreateAsset(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mwanachamaassetmanager.Asset
		if err := httpwire.ReadJSON(r, &in); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.CreateAsset(r.Context(), in)
		if err != nil {
			writeAssetErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusCreated, out)
	}
}

// GetAsset handles GET {assetID}.
func GetAsset(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.GetAsset(r.Context(), r.PathValue("assetID"))
		if err != nil {
			writeAssetErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// assetEditBody is the wire shape for UpdateAsset — Name/Category/
// AttributesJSON/SerialTag, the only fields UpdateAsset accepts.
// TrackingMode and LocationID are deliberately absent: UpdateAsset itself
// refuses to change TrackingMode and never accepts LocationID as a
// caller-writable field (see asset.go's doc), so this body has no key that
// could look like it works and silently does nothing.
type assetEditBody struct {
	Name           string `json:"name"`
	Category       string `json:"category"`
	AttributesJSON string `json:"attributes_json"`
	SerialTag      string `json:"serial_tag"`
}

// UpdateAsset handles PATCH {assetID} — decode, update, encode.
func UpdateAsset(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body assetEditBody
		if err := httpwire.ReadJSON(r, &body); err != nil {
			httpwire.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		in := mwanachamaassetmanager.Asset{
			ID:             r.PathValue("assetID"),
			Name:           body.Name,
			Category:       body.Category,
			AttributesJSON: body.AttributesJSON,
			SerialTag:      body.SerialTag,
		}
		out, err := am.UpdateAsset(r.Context(), in)
		if err != nil {
			writeAssetErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// DeleteAsset handles DELETE {assetID}.
func DeleteAsset(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := am.DeleteAsset(r.Context(), r.PathValue("assetID")); err != nil {
			writeAssetErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListAssets handles GET — optionally filtered by the query params
// category, tracking_mode, and location_id.
func ListAssets(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := mwanachamaassetmanager.AssetFilter{
			Category:     q.Get("category"),
			TrackingMode: mwanachamaassetmanager.AssetTrackingMode(q.Get("tracking_mode")),
			LocationID:   q.Get("location_id"),
		}
		out, err := am.ListAssets(r.Context(), filter)
		if err != nil {
			writeAssetErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, out)
	}
}

// GetAssetBalance handles GET {assetID}/balance?location_id=... — the
// ledger fold at one location, from movement.go's GetAssetBalance.
// location_id is required; an absent one is refused rather than read as
// "balance nowhere", which would just always be zero and mask a caller
// mistake.
func GetAssetBalance(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		locationID := r.URL.Query().Get("location_id")
		if locationID == "" {
			httpwire.WriteErr(w, http.StatusBadRequest, "location_id is required")
			return
		}
		balance, err := am.GetAssetBalance(r.Context(), r.PathValue("assetID"), locationID)
		if err != nil {
			writeAssetErr(w, err)
			return
		}
		httpwire.WriteJSON(w, http.StatusOK, map[string]int64{"balance": balance})
	}
}
