package db

import (
	"context"

	"gorm.io/gorm"
)

type Join struct {
	Query string
	Args  []any
}

func NewJoin(query string, args ...any) Join {
	return Join{
		Query: query,
		Args:  args,
	}
}

func Select(ctx context.Context, db *gorm.DB, table string, columns []string, joins ...Join) *gorm.DB {
	query := db.WithContext(ctx).Table(table)
	if len(columns) > 0 {
		query = query.Select(columns)
	}
	for _, join := range joins {
		query = query.Joins(join.Query, join.Args...)
	}
	return query
}
