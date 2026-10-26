package base

import (
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

// Could not use base repo to query
type (
	IBaseRepositorySqlx[T any, ID any] interface {
		GetTableName() string
	}

	BaseRepositorySqlx[T any, ID any] struct {
		tableName string

		Sqlx *database.SqlxDatabase
	}
)

func NewBaseRepositorySqlx[T, ID any](db *database.SqlxDatabase, tableName string) *BaseRepositorySqlx[T, ID] {
	repo := &BaseRepositorySqlx[T, ID]{Sqlx: db, tableName: tableName}
	// check compile-time with type assertion
	var _ IBaseRepositorySqlx[T, ID] = repo

	return repo
}

func (repo *BaseRepositorySqlx[T, ID]) GetTableName() string {
	return repo.tableName
}
