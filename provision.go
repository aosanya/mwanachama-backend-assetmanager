package mwanachamaassetmanager

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const retiredReversesIndex = "reverses"

func Provision(db *gorm.DB, s *spec.Spec) error {
	if err := dropDuplicateReversals(db, s); err != nil {
		return err
	}
	if err := spec.Migrate(db, s); err != nil {
		return err
	}
	return dropRetiredReversesIndex(db, s)
}

func dropDuplicateReversals(db *gorm.DB, s *spec.Spec) error {
	for _, o := range s.Objects {
		if o.Role != roleMovement {
			continue
		}
		table := s.TableFor(o)
		if !db.Migrator().HasTable(table) {
			return nil
		}
		stmt := fmt.Sprintf(
			"delete from %[1]s where reverses_movement_id <> '' and id not in "+
				"(select min(id) from %[1]s where reverses_movement_id <> '' group by reverses_movement_id)", table)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("assetmanager: drop duplicate reversals: %w", err)
		}
	}
	return nil
}

func dropRetiredReversesIndex(db *gorm.DB, s *spec.Spec) error {
	for _, o := range s.Objects {
		if o.Role != roleMovement {
			continue
		}
		stmt := "drop index if exists " + s.IndexFor(o, spec.Index{Name: retiredReversesIndex})
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("assetmanager: drop retired reverses index: %w", err)
		}
	}
	return nil
}
