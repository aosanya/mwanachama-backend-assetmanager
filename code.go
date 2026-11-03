package mwanachamaassetmanager

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const codeSequenceAttempts = 5

type codeSequence struct {
	EntityType string `json:"entity_type"`
	NextNumber int64  `json:"next_number"`
}

func (m *assetManager) nextCode(ctx context.Context, tx *gorm.DB, entityType, prefix string) (string, error) {
	o := m.store.Object(roleCodeSequence)
	table := m.store.Table(roleCodeSequence)

	seed, err := encode(o, codeSequence{EntityType: entityType, NextNumber: 1})
	if err != nil {
		return "", fmt.Errorf("nextCode: %w", err)
	}
	if err := tx.WithContext(ctx).Table(table).
		Clauses(clause.OnConflict{DoNothing: true}).Create(seed).Error; err != nil {
		return "", fmt.Errorf("nextCode: seed the %s counter: %w", entityType, err)
	}

	for attempt := 0; attempt < codeSequenceAttempts; attempt++ {
		var rows []map[string]any
		err := tx.WithContext(ctx).Table(table).Where("entity_type = ?", entityType).Limit(1).Find(&rows).Error
		if err != nil {
			return "", fmt.Errorf("nextCode: %w", err)
		}
		if len(rows) == 0 {
			return "", fmt.Errorf("nextCode: %s: the counter vanished after being seeded", entityType)
		}

		var seq codeSequence
		if err := decode(o, rows[0], &seq); err != nil {
			return "", fmt.Errorf("nextCode: %w", err)
		}

		claimed := tx.WithContext(ctx).Table(table).
			Where("entity_type = ? AND next_number = ?", entityType, seq.NextNumber).
			Update("next_number", seq.NextNumber+1)
		if claimed.Error != nil {
			return "", fmt.Errorf("nextCode: %w", claimed.Error)
		}
		if claimed.RowsAffected == 1 {
			return fmt.Sprintf("%s-%d", prefix, seq.NextNumber), nil
		}
	}
	return "", fmt.Errorf("nextCode: %s: the counter moved under %d attempts: %w", entityType, codeSequenceAttempts, ErrConflict)
}
