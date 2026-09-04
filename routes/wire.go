package routes

import (
	"encoding/json"
	"net/http"
)

// writeJSON and writeErr mirror mwanachama-backend-actor/routes' wire.go
// byte-for-byte on purpose — a shared wire convention across every package
// built this way.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes {"error": msg}.
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// readJSON decodes a JSON body into v, refusing unknown fields.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
