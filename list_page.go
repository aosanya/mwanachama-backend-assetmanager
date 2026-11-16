package mwanachamaassetmanager

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const maxListPage = 500

var ErrInvalidFilter = errors.New("invalid list filter")

func page(q *gorm.DB, limit, offset int) (*gorm.DB, error) {
	if limit < 0 {
		return nil, fmt.Errorf("%w: limit %d", ErrInvalidFilter, limit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("%w: offset %d", ErrInvalidFilter, offset)
	}
	if limit == 0 || limit > maxListPage {
		limit = maxListPage
	}
	q = q.Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	return q, nil
}
