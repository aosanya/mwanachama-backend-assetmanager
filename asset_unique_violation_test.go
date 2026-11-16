package mwanachamaassetmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func resetAssetCodeCounter(t *testing.T, m AssetManager) {
	t.Helper()
	am := m.(*assetManager)
	err := am.db.Table(am.store.Table(roleCodeSequence)).
		Where("entity_type = ?", "asset").
		Update("next_number", 1).Error
	if err != nil {
		t.Fatal(err)
	}
}

func TestACodeCollisionIsNotAnsweredAsSerialTagInUse(t *testing.T) {
	for name, managersOf := range serialRaceStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m := managersOf()[0]
			first, err := m.CreateAsset(ctx, models.Asset{Name: "maize", TrackingMode: models.AssetTrackingModeFungible})
			if err != nil {
				t.Fatal(err)
			}
			for _, loser := range []models.Asset{
				{Name: "bag of rice", TrackingMode: models.AssetTrackingModeFungible},
				{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "ear-fresh"},
			} {
				resetAssetCodeCounter(t, m)
				_, err = m.CreateAsset(ctx, loser)
				if err == nil {
					t.Fatalf("%s: a second asset took the code %s", loser.TrackingMode, first.Code)
				}
				if errors.Is(err, ErrAssetSerialTagExists) {
					t.Fatalf("%s: a code collision was answered %v, want an error that does not blame the serial tag", loser.TrackingMode, err)
				}
			}
		})
	}
}

func TestARealSerialTagClashStillAnswersSerialTagInUse(t *testing.T) {
	for name, managersOf := range serialRaceStores(t) {
		t.Run(name, func(t *testing.T) {
			m := managersOf()[0]
			ctx := context.Background()
			if _, err := m.CreateAsset(ctx, models.Asset{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "ear-1"}); err != nil {
				t.Fatal(err)
			}
			if _, err := m.CreateAsset(ctx, models.Asset{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "ear-1"}); !errors.Is(err, ErrAssetSerialTagExists) {
				t.Fatalf("a second create of the tag answered %v, want ErrAssetSerialTagExists", err)
			}
		})
	}
}

func TestSerialTagIndexIsRecognisedByTheNameEachDatabaseGives(t *testing.T) {
	m := serialRaceStores(t)["memory"]()[0].(*assetManager)
	table := m.store.Table(roleAsset)
	cases := []struct {
		violated string
		want     bool
	}{
		{table + ".serial_tag (2067)", true},
		{table + ".serial_tag", true},
		{table + "_serial_tag_idx", true},
		{table + ".code (2067)", false},
		{table + "_code_key", false},
		{table + ".id (1555)", false},
		{"other_assets.serial_tag (2067)", false},
		{"", false},
	}
	for _, c := range cases {
		if got := m.namesSerialTagIndex(c.violated); got != c.want {
			t.Errorf("namesSerialTagIndex(%q) = %v, want %v", c.violated, got, c.want)
		}
	}
}
