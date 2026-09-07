// location.go — HTTP routes over location.go's Location CRUD and tree
// queries: CreateLocation, GetLocation, UpdateLocation, DeleteLocation,
// ListLocations, ListDescendantLocations. See doc.go for scope.
package routes

import (
	"errors"
	"net/http"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// locationStatusFor maps this package's Location error sentinels to a
// status code.
func locationStatusFor(err error) int {
	switch {
	case errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound):
		return http.StatusNotFound
	case errors.Is(err, mwanachamaassetmanager.ErrLocationNotEmpty):
		return http.StatusConflict
	case errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeLocationErr(w http.ResponseWriter, err error) {
	code := locationStatusFor(err)
	if code == http.StatusInternalServerError {
		writeErr(w, code, "internal error")
		return
	}
	writeErr(w, code, err.Error())
}

// CreateLocation handles POST — decode, create, encode.
func CreateLocation(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mwanachamaassetmanager.Location
		if err := readJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := am.CreateLocation(r.Context(), in)
		if err != nil {
			writeLocationErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// GetLocation handles GET {locationID}.
func GetLocation(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.GetLocation(r.Context(), r.PathValue("locationID"))
		if err != nil {
			writeLocationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// locationEditBody is the wire shape for UpdateLocation — Name/Kind/
// ParentLocationID, the three fields UpdateLocation accepts. A distinct
// type (rather than models.Location itself) so the id always comes from
// the path, never the body.
type locationEditBody struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	ParentLocationID string `json:"parent_location_id"`
}

// UpdateLocation handles PATCH {locationID} — decode, update (patching
// Name/Kind and re-parenting if ParentLocationID changed), encode.
func UpdateLocation(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body locationEditBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		in := mwanachamaassetmanager.Location{
			ID:               r.PathValue("locationID"),
			Name:             body.Name,
			Kind:             body.Kind,
			ParentLocationID: body.ParentLocationID,
		}
		out, err := am.UpdateLocation(r.Context(), in)
		if err != nil {
			writeLocationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// DeleteLocation handles DELETE {locationID}.
func DeleteLocation(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := am.DeleteLocation(r.Context(), r.PathValue("locationID")); err != nil {
			writeLocationErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListLocations handles GET — optionally filtered by the query params
// parent_location_id and root_only ("true" for root-only; any other value,
// including absent, is ignored).
func ListLocations(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := mwanachamaassetmanager.LocationFilter{
			ParentLocationID: r.URL.Query().Get("parent_location_id"),
			RootOnly:         r.URL.Query().Get("root_only") == "true",
		}
		out, err := am.ListLocations(r.Context(), filter)
		if err != nil {
			writeLocationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ListDescendantLocations handles GET {locationID}/descendants.
func ListDescendantLocations(am mwanachamaassetmanager.AssetManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := am.ListDescendantLocations(r.Context(), r.PathValue("locationID"))
		if err != nil {
			writeLocationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
