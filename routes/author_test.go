package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

type ledgerAuthors struct {
	mwanachamaassetmanager.AssetManager
	postedBy, reversedBy, committedBy, placedBy string
}

func (l *ledgerAuthors) PostMovement(_ context.Context, mv models.Movement) (models.Movement, error) {
	l.postedBy = mv.PerformedBy
	return models.Movement{}, nil
}

func (l *ledgerAuthors) ReverseMovement(_ context.Context, _, by, _ string) (models.Movement, error) {
	l.reversedBy = by
	return models.Movement{}, nil
}

func (l *ledgerAuthors) CreateHold(_ context.Context, h models.Hold) (models.Hold, error) {
	l.placedBy = h.PlacedBy
	return models.Hold{}, nil
}

func (l *ledgerAuthors) CommitHold(_ context.Context, _ string, _ int64, _ models.MovementKind, _, by string) (models.Hold, models.Movement, error) {
	l.committedBy = by
	return models.Hold{}, models.Movement{}, nil
}

var theSession = Mount{Caller: func(context.Context) string { return "the-session" }}

func routeFor(t *testing.T, manager mwanachamaassetmanager.AssetManager, action string) Route {
	t.Helper()
	for _, route := range RoutesFor(manager, theSession) {
		if route.Action == action {
			return route
		}
	}
	t.Fatalf("no route for %s", action)
	return Route{}
}

func post(handler http.HandlerFunc, body string, pathValues map[string]string) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	handler(httptest.NewRecorder(), req)
}

func TestAReversalIsRecordedByTheCallerNotTheBody(t *testing.T) {
	manager := &ledgerAuthors{}
	post(routeFor(t, manager, "assetmanager.movement.reverse").Handler, `{"performed_by":"forged-clerk"}`, map[string]string{"movementID": "m1"})
	if manager.reversedBy != "the-session" {
		t.Errorf("the reversal names %q; want the-session, never the body's forged-clerk", manager.reversedBy)
	}
}

func TestACommitIsTakenByTheCallerNotTheBody(t *testing.T) {
	manager := &ledgerAuthors{}
	post(routeFor(t, manager, "assetmanager.hold.commit").Handler, `{"quantity":1,"kind":"departed","performed_by":"forged-clerk"}`, map[string]string{"holdID": "h1"})
	if manager.committedBy != "the-session" {
		t.Errorf("the commit names %q; want the-session, never the body's forged-clerk", manager.committedBy)
	}
}

func TestAMovementIsPostedByTheCallerNotTheBody(t *testing.T) {
	manager := &ledgerAuthors{}
	post(routeFor(t, manager, "assetmanager.movement.post").Handler, `{"asset_id":"a1","kind":"arrived","quantity":1,"to_location_id":"l1","performed_by":"forged-clerk"}`, nil)
	if manager.postedBy != "the-session" {
		t.Errorf("the entry names %q; want the-session, never the body's forged-clerk", manager.postedBy)
	}
}

func TestAHoldIsPlacedByTheCallerNotTheBody(t *testing.T) {
	manager := &ledgerAuthors{}
	post(routeFor(t, manager, "assetmanager.hold.create").Handler, `{"asset_id":"a1","location_id":"l1","quantity":1,"expires_at":"2099-01-01T00:00:00Z","placed_by":"forged-clerk"}`, nil)
	if manager.placedBy != "the-session" {
		t.Errorf("the hold names %q; want the-session, never the body's forged-clerk", manager.placedBy)
	}
}

func TestLedgerToolsDoNotAskForTheirAuthor(t *testing.T) {
	tools, err := BuildTools(&ledgerAuthors{}, theSession)
	if err != nil {
		t.Fatalf("BuildTools: %v", err)
	}
	byName := map[string]Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	for name, field := range map[string]string{
		"assetmanager_movement_reverse": "performed_by",
		"assetmanager_hold_commit":      "performed_by",
		"assetmanager_movement_post":    "performed_by",
		"assetmanager_hold_create":      "placed_by",
	} {
		tool, ok := byName[name]
		if !ok {
			t.Fatalf("%s is not a tool", name)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatalf("%s schema: %v", name, err)
		}
		if _, asked := schema.Properties[field]; asked {
			t.Errorf("%s still takes %s from the request; the caller signs it", name, field)
		}
	}
}

func TestLedgerToolsSignWithTheCaller(t *testing.T) {
	manager := &ledgerAuthors{}
	tools, err := BuildTools(manager, theSession)
	if err != nil {
		t.Fatalf("BuildTools: %v", err)
	}
	call := func(name, arguments string) {
		for _, tool := range tools {
			if tool.Name == name {
				if _, err := tool.Handle(context.Background(), dispatch.Request{Arguments: json.RawMessage(arguments)}); err != nil {
					t.Errorf("%s: %v", name, err)
				}
				return
			}
		}
		t.Errorf("%s is not a tool", name)
	}
	call("assetmanager_movement_reverse", `{"movementID":"m1"}`)
	call("assetmanager_hold_commit", `{"holdID":"h1","quantity":1,"kind":"departed"}`)
	call("assetmanager_movement_post", `{"asset_id":"a1","kind":"arrived","quantity":1,"to_location_id":"l1"}`)
	call("assetmanager_hold_create", `{"asset_id":"a1","location_id":"l1","quantity":1,"expires_at":"2099-01-01T00:00:00Z"}`)
	for field, got := range map[string]string{"reverse": manager.reversedBy, "commit": manager.committedBy, "post": manager.postedBy, "hold": manager.placedBy} {
		if got != "the-session" {
			t.Errorf("%s is signed by %q; want the-session", field, got)
		}
	}
}
