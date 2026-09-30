package mwanachamaassetmanager

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type codeSequence struct {
	EntityType string `json:"entity_type"`
	NextNumber int64  `json:"next_number"`
}

func (m *assetManager) nextCode(ctx context.Context, tx *gorm.DB, entityType, prefix string) (string, error) {
	o := m.store.Object(roleCodeSequence)
	table := m.store.Table(roleCodeSequence)

	var rows []map[string]any
	err := tx.WithContext(ctx).Table(table).Where("entity_type = ?", entityType).Limit(1).Find(&rows).Error
	if err != nil {
		return "", fmt.Errorf("nextCode: %w", err)
	}

	seq := codeSequence{EntityType: entityType, NextNumber: 1}
	if len(rows) == 0 {
		row, err := encode(o, seq)
		if err != nil {
			return "", fmt.Errorf("nextCode: %w", err)
		}
		if err := tx.WithContext(ctx).Table(table).Create(row).Error; err != nil {
			return "", fmt.Errorf("nextCode: %w", err)
		}
	} else if err := decode(o, rows[0], &seq); err != nil {
		return "", fmt.Errorf("nextCode: %w", err)
	}

	n := seq.NextNumber
	if err := tx.WithContext(ctx).Table(table).Where("entity_type = ?", entityType).
		Update("next_number", n+1).Error; err != nil {
		return "", fmt.Errorf("nextCode: %w", err)
	}
	return fmt.Sprintf("%s-%d", prefix, n), nil
}
