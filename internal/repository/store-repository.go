package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

type (
	IStoreRepository interface {
		base.IBaseRepositorySqlx[model.StoreModel, uuid.UUID]

		Create(ctx context.Context, store *model.StoreModel) (*mapper.StoreMapper, error)
		GetByID(ctx context.Context, id uuid.UUID) (*mapper.StoreMapper, error)
		Exists(ctx context.Context, id uuid.UUID) (bool, error)
	}

	StoreRepository struct {
		*base.BaseRepositorySqlx[model.StoreModel, uuid.UUID]

		logger *zap.Logger
	}
)

func NewStoreRepository(logger *zap.Logger, db *database.SqlxDatabase) IStoreRepository {
	return &StoreRepository{
		BaseRepositorySqlx: base.NewBaseRepositorySqlx[model.StoreModel, uuid.UUID](db, "platform.stores"),
		logger:             logger,
	}
}

func (r *StoreRepository) Create(ctx context.Context, store *model.StoreModel) (*mapper.StoreMapper, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`, r.GetTableName())

	var row mapper.StoreMapper
	if err := r.Sqlx.GetContext(ctx, &row, query, store.Name); err != nil {
		r.logger.Error("failed to create store", zap.Error(err))
		return nil, fmt.Errorf("create store: %w", err)
	}
	return &row, nil
}

func (r *StoreRepository) GetByID(ctx context.Context, id uuid.UUID) (*mapper.StoreMapper, error) {
	query := fmt.Sprintf(`
		SELECT id, name, created_at, updated_at
		FROM %s
		WHERE id = $1
	`, r.GetTableName())

	var row mapper.StoreMapper
	err := r.Sqlx.GetContext(ctx, &row, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, base.ErrNotFound
	}
	if err != nil {
		r.logger.Error("failed to get store", zap.Error(err))
		return nil, fmt.Errorf("get store: %w", err)
	}
	return &row, nil
}

func (r *StoreRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)`, r.GetTableName())

	var exists bool
	if err := r.Sqlx.GetContext(ctx, &exists, query, id); err != nil {
		r.logger.Error("failed to check store", zap.Error(err))
		return false, fmt.Errorf("store exists: %w", err)
	}
	return exists, nil
}
