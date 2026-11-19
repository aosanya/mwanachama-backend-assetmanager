package mwanachamaassetmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-shared/versioncheck"
)

const longAgo = "2001-01-01T00:00:00Z"

func renamed(a models.Asset, name, version string) models.AssetUpdate {
	return models.AssetUpdate{ID: a.ID, Name: name, Category: a.Category, SerialTag: a.SerialTag, AttributesJSON: a.AttributesJSON, Version: version}
}

func TestAnAssetUpdateCarryingTheCurrentVersionIsSaved(t *testing.T) {
	f := newHoldFixture(t, 0)
	saved, err := f.m.UpdateAsset(context.Background(), renamed(f.asset, "duck eggs", f.asset.UpdatedAt))
	if err != nil {
		t.Fatalf("UpdateAsset: %v", err)
	}
	if saved.Name != "duck eggs" {
		t.Fatalf("saved name %q", saved.Name)
	}
}

func TestAnAssetUpdateWithNoVersionIsSavedAsBefore(t *testing.T) {
	f := newHoldFixture(t, 0)
	if _, err := f.m.UpdateAsset(context.Background(), renamed(f.asset, "duck eggs", "")); err != nil {
		t.Fatalf("UpdateAsset: %v", err)
	}
}

func TestAnAssetUpdateOnAnOldVersionIsRefusedNamingTheChangedField(t *testing.T) {
	f := newHoldFixture(t, 0)
	_, err := f.m.UpdateAsset(context.Background(), renamed(f.asset, "duck eggs", longAgo))
	stale, ok := versioncheck.IsStale(err)
	if !ok {
		t.Fatalf("UpdateAsset = %v; want a stale refusal", err)
	}
	if stale.Field != "name" || err.Error() != "name changed since you read it" {
		t.Fatalf("refusal %q naming %q", err, stale.Field)
	}
	kept, err := f.m.GetAsset(context.Background(), f.asset.ID)
	if err != nil || kept.Name != f.asset.Name {
		t.Fatalf("stale save was written: %+v, %v", kept, err)
	}
}

func TestAStaleAssetUpdateNamesTheFirstFieldTheCallerChanged(t *testing.T) {
	f := newHoldFixture(t, 0)
	change := renamed(f.asset, f.asset.Name, longAgo)
	change.Category = "dairy"
	_, err := f.m.UpdateAsset(context.Background(), change)
	if err == nil || err.Error() != "category changed since you read it" {
		t.Fatalf("UpdateAsset = %v", err)
	}
}

func TestAnAssetUpdateOnAMissingAssetStillSaysNotFound(t *testing.T) {
	f := newHoldFixture(t, 0)
	_, err := f.m.UpdateAsset(context.Background(), models.AssetUpdate{ID: "nope", Name: "x", Version: longAgo})
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("UpdateAsset = %v", err)
	}
}
