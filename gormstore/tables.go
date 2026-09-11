package gormstore

import (
	"fmt"

	"gorm.io/gorm"
)

// TableNames configures which physical tables an AssetManager reads and
// writes.
type TableNames struct {
	Locations     string
	Assets        string
	Movements     string
	Holds         string
	CodeSequences string
}

// DefaultTableNames builds the conventional table set for one mounted
// instance of this package, e.g. DefaultTableNames("assetit") yields
// assetit_locations, assetit_assets, assetit_movements, assetit_holds,
// assetit_code_sequences.
func DefaultTableNames(instance string) TableNames {
	return TableNames{
		Locations:     instance + "_locations",
		Assets:        instance + "_assets",
		Movements:     instance + "_movements",
		Holds:         instance + "_holds",
		CodeSequences: instance + "_code_sequences",
	}
}

// Migrate creates or updates the four tables t names, via GORM's
// AutoMigrate scoped to each table name in turn. Callers run this once at
// startup (or in test setup) before constructing an AssetManager with the
// same db and t.
func Migrate(db *gorm.DB, t TableNames) error {
	if err := db.Table(t.Locations).AutoMigrate(&LocationRow{}); err != nil {
		return err
	}
	if err := db.Table(t.Assets).AutoMigrate(&AssetRow{}); err != nil {
		return err
	}
	if err := syncSerialTagUniqueIndex(db, t.Assets); err != nil {
		return err
	}
	if err := db.Table(t.Movements).AutoMigrate(&MovementRow{}); err != nil {
		return err
	}
	if err := db.Table(t.Holds).AutoMigrate(&HoldRow{}); err != nil {
		return err
	}
	if err := db.Table(t.CodeSequences).AutoMigrate(&CodeSequenceRow{}); err != nil {
		return err
	}
	if err := BackfillCodes(db, t.Locations, t.CodeSequences, "location", "L"); err != nil {
		return err
	}
	if err := BackfillCodes(db, t.Assets, t.CodeSequences, "asset", "A"); err != nil {
		return err
	}
	return nil
}

// syncSerialTagUniqueIndex creates a partial unique index over
// AssetRow.SerialTag, scoped to non-deleted rows with a non-empty tag. This
// is the database-level half of CreateAsset's uniqueness check (see
// asset.go) — a pre-check query alone lets two concurrent creates both pass
// before either commits; only this index stops that race. Partial so a
// fungible asset (SerialTag always empty) and a soft-deleted serialized
// asset never collide with a live one.
func syncSerialTagUniqueIndex(db *gorm.DB, table string) error {
	idx := fmt.Sprintf("%s_serial_tag_uniq", table)
	sql := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (serial_tag) WHERE deleted = false AND serial_tag <> ''",
		idx, table,
	)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("syncSerialTagUniqueIndex: %s: %w", idx, err)
	}
	return nil
}
