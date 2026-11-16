package routes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

const listPageSeeded = 5

var listPageActions = map[string]string{
	"/assets":    "assetmanager.asset.list",
	"/movements": "assetmanager.movement.list",
	"/holds":     "assetmanager.hold.list",
	"/locations": "assetmanager.location.list",
}

type listPageWalk struct {
	am         mwanachamaassetmanager.AssetManager
	assetID    string
	locationID string
}

func newListPageWalk(t *testing.T) listPageWalk {
	t.Helper()
	am := newTestManager(t)
	ctx := context.Background()
	asset := newAsset(t, am, "Chair", mwanachamaassetmanager.AssetTrackingModeFungible)
	loc := newLocation(t, am, "Store", "")
	for i := 1; i < listPageSeeded; i++ {
		newAsset(t, am, fmt.Sprintf("Chair %d", i), mwanachamaassetmanager.AssetTrackingModeFungible)
		newLocation(t, am, fmt.Sprintf("Store %d", i), "")
	}
	for i := 0; i < listPageSeeded; i++ {
		_, err := am.PostMovement(ctx, mwanachamaassetmanager.Movement{
			AssetID: asset.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor,
		})
		if err != nil {
			t.Fatalf("seed PostMovement: %v", err)
		}
		_, err = am.CreateHold(ctx, mwanachamaassetmanager.Hold{
			AssetID: asset.ID, LocationID: loc.ID, Quantity: 1, PlacedBy: testActor, ExpiresAt: futureRFC3339(t),
		})
		if err != nil {
			t.Fatalf("seed CreateHold: %v", err)
		}
	}
	return listPageWalk{am: am, assetID: asset.ID, locationID: loc.ID}
}

func (w listPageWalk) get(t *testing.T, list, query string) *httptest.ResponseRecorder {
	t.Helper()
	handler := handlerFor(t, w.am, listPageActions[list])
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, list+query, nil))
	return rec
}

func (w listPageWalk) ids(t *testing.T, list, query string) []string {
	t.Helper()
	rec := w.get(t, list, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s%s: %d %s", list, query, rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("GET %s%s: %v", list, query, err)
	}
	var ids []string
	for _, row := range rows {
		id, _ := row["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func TestListPage_ALimitAnswersThatManyRowsOnEveryList(t *testing.T) {
	w := newListPageWalk(t)
	for list := range listPageActions {
		if got := len(w.ids(t, list, "?limit=2")); got != 2 {
			t.Errorf("GET %s?limit=2 answered %d rows; want 2", list, got)
		}
	}
}

func TestListPage_AnOffsetStepsPastTheFirstRowsOnEveryList(t *testing.T) {
	w := newListPageWalk(t)
	for list := range listPageActions {
		all := w.ids(t, list, "")
		if len(all) != listPageSeeded {
			t.Fatalf("GET %s answered %d rows; want the %d seeded", list, len(all), listPageSeeded)
		}
		second := w.ids(t, list, "?limit=2&offset=2")
		if len(second) != 2 || second[0] != all[2] || second[1] != all[3] {
			t.Errorf("GET %s?limit=2&offset=2 answered %v; want rows 3 and 4 of %v", list, second, all)
		}
		last := w.ids(t, list, "?limit=2&offset=4")
		if len(last) != 1 || last[0] != all[4] {
			t.Errorf("GET %s?limit=2&offset=4 answered %v; want the last row of %v", list, last, all)
		}
	}
}

func TestListPage_ANegativeLimitOrOffsetIsRefusedOnEveryList(t *testing.T) {
	w := newListPageWalk(t)
	for list := range listPageActions {
		for _, query := range []string{"?limit=-1", "?offset=-1"} {
			if rec := w.get(t, list, query); rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s%s: %d; want 400", list, query, rec.Code)
			}
		}
	}
}

func TestListPage_ABalanceStillFoldsTheWholeLedger(t *testing.T) {
	w := newListPageWalk(t)
	handler := handlerFor(t, w.am, "assetmanager.asset.balance")
	rec := httptest.NewRecorder()
	req := withPathValue(httptest.NewRequest(http.MethodGet, "/assets/"+w.assetID+"/balance?location_id="+w.locationID, nil), "assetID", w.assetID)
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("balance: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("balance: %v", err)
	}
	if got, _ := out["balance"].(float64); got != listPageSeeded*5 {
		t.Errorf("balance is %v; want %d from %d arrivals of 5", out["balance"], listPageSeeded*5, listPageSeeded)
	}
}

func TestListPage_ANoLimitListIsCappedAtFiveHundredRows(t *testing.T) {
	w := listPageWalk{am: newTestManager(t)}
	for i := 0; i < 505; i++ {
		newLocation(t, w.am, fmt.Sprintf("Bulk %d", i), "")
	}
	if got := len(w.ids(t, "/locations", "")); got != 500 {
		t.Errorf("GET /locations with no limit answered %d rows; want the cap of 500", got)
	}
	if got := len(w.ids(t, "/locations", "?limit=1000")); got != 500 {
		t.Errorf("GET /locations?limit=1000 answered %d rows; want the cap of 500", got)
	}
	if got := len(w.ids(t, "/locations", "?offset=500")); got != 5 {
		t.Errorf("GET /locations?offset=500 answered %d rows; want the remaining 5", got)
	}
}
