package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

type failingLedger struct {
	mwanachamaassetmanager.AssetManager
	err error
}

func (f failingLedger) PostMovement(context.Context, models.Movement) (models.Movement, error) {
	return models.Movement{}, f.err
}

func serve(list []Route, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	for _, rt := range list {
		mux.HandleFunc(rt.Pattern(""), rt.Handler)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func routeOf(t *testing.T, list []Route, action string) Route {
	t.Helper()
	for _, rt := range list {
		if rt.Action == action {
			return rt
		}
	}
	t.Fatalf("no route for %s", action)
	return Route{}
}

const arrivedBody = `{"asset_id":"a1","kind":"arrived","quantity":1,"to_location_id":"l1"}`

func TestBuildYieldsTheSameRoutesAsRoutesFor(t *testing.T) {
	want := len(RoutesFor(&ledgerAuthors{}, theSession))
	if want == 0 {
		t.Fatal("no routes declared")
	}
	built, err := Build(&ledgerAuthors{}, theSession)
	if err != nil || len(built) != want {
		t.Fatalf("Build = %d routes, %v; want %d", len(built), err, want)
	}
	if got := len(Routes(&ledgerAuthors{})); got != want {
		t.Fatalf("Routes = %d routes; want %d", got, want)
	}
}

func TestEveryRouteCarriesAnAssetmanagerAction(t *testing.T) {
	for _, rt := range RoutesFor(&ledgerAuthors{}, theSession) {
		if !strings.HasPrefix(rt.Action, "assetmanager.") {
			t.Errorf("%s %s has action %q", rt.Method, rt.Path, rt.Action)
		}
	}
}

func TestNoActionIsAnonymousSoEverythingIsGated(t *testing.T) {
	split := Split(&ledgerAuthors{}, theSession)
	if len(split.Anonymous) != 0 {
		t.Errorf("%d anonymous routes; want none", len(split.Anonymous))
	}
	if len(split.Gated) != len(RoutesFor(&ledgerAuthors{}, theSession)) {
		t.Errorf("%d gated routes; want every route", len(split.Gated))
	}
	if len(AnonymousActions) != 0 {
		t.Errorf("AnonymousActions = %v; want empty", AnonymousActions)
	}
}

func TestEverySentinelIsMappedToAStatusInTheOperationsFile(t *testing.T) {
	unmapped, err := Table().UnmappedSentinels()
	if err != nil || len(unmapped) != 0 {
		t.Fatalf("unmapped sentinels %v, %v", unmapped, err)
	}
	seen := map[error]string{}
	for name, err := range sentinels {
		if err == nil {
			t.Errorf("%s is nil", name)
			continue
		}
		if other, dup := seen[err]; dup {
			t.Errorf("%s and %s share one error", name, other)
		}
		seen[err] = name
	}
}

func TestAPostedMovementAnswersTheStatusOfTheRefusalTheManagerRaised(t *testing.T) {
	cases := map[string]struct {
		err  error
		code int
	}{
		"asset missing":          {mwanachamaassetmanager.ErrAssetNotFound, http.StatusNotFound},
		"invalid movement":       {mwanachamaassetmanager.ErrInvalidMovement, http.StatusBadRequest},
		"not enough available":   {mwanachamaassetmanager.ErrInsufficientAvailable, http.StatusConflict},
		"already reversed":       {mwanachamaassetmanager.ErrMovementAlreadyReversed, http.StatusConflict},
		"wrapped not found":      {errors.Join(errors.New("while posting"), mwanachamaassetmanager.ErrLocationNotFound), http.StatusNotFound},
		"unknown failure hidden": {errors.New("disk on fire"), http.StatusInternalServerError},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			list := RoutesFor(failingLedger{err: c.err}, theSession)
			rec := serve(list, http.MethodPost, routeOf(t, list, "assetmanager.movement.post").Path, arrivedBody)
			if rec.Code != c.code {
				t.Fatalf("answered %d %s; want %d", rec.Code, rec.Body.String(), c.code)
			}
			if c.code == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "disk on fire") {
				t.Errorf("internal failure text leaked: %s", rec.Body.String())
			}
		})
	}
}

func TestARefusingAuthorizerAnswersForbiddenBeforeTheManagerIsReached(t *testing.T) {
	var asked []string
	refuse := func(_ context.Context, action string) error {
		asked = append(asked, action)
		return errors.New("no")
	}
	mount := Mount{Authorize: refuse, Caller: theSession.Caller}
	manager := &ledgerAuthors{}
	list := RoutesFor(manager, mount)
	rec := serve(list, http.MethodPost, routeOf(t, list, "assetmanager.movement.post").Path, arrivedBody)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("refused post answered %d %s", rec.Code, rec.Body.String())
	}
	if len(asked) != 1 || asked[0] != "assetmanager.movement.post" {
		t.Fatalf("authorizer asked %v", asked)
	}
	if manager.postedBy != "" {
		t.Errorf("the manager was reached by %q despite the refusal", manager.postedBy)
	}
}

func TestAnAllowingAuthorizerLetsThePostThrough(t *testing.T) {
	var asked []string
	allow := func(_ context.Context, action string) error {
		asked = append(asked, action)
		return nil
	}
	manager := &ledgerAuthors{}
	list := RoutesFor(manager, Mount{Authorize: allow, Caller: theSession.Caller})
	rec := serve(list, http.MethodPost, routeOf(t, list, "assetmanager.movement.post").Path, arrivedBody)
	if rec.Code >= 400 {
		t.Fatalf("allowed post answered %d %s", rec.Code, rec.Body.String())
	}
	if len(asked) != 1 || manager.postedBy != "the-session" {
		t.Fatalf("asked %v, posted by %q", asked, manager.postedBy)
	}
}

func TestBuildToolsRefusesNothingAndNamesEveryActionAsATool(t *testing.T) {
	tools, err := BuildTools(&ledgerAuthors{}, theSession)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != len(RoutesFor(&ledgerAuthors{}, theSession)) {
		t.Errorf("%d tools for %d routes", len(tools), len(RoutesFor(&ledgerAuthors{}, theSession)))
	}
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "assetmanager_") {
			t.Errorf("tool %q is not an assetmanager tool", tool.Name)
		}
	}
}
