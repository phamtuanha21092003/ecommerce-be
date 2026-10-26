package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

const productColumns = `
	id,
	store_id,
	sku,
	name,
	description,
	price,
	currency,
	status,
	created_at,
	updated_at,
	archived_at,
	version
`

type (
	IProductRepository interface {
		base.IBaseRepositorySqlx[model.ProductModel, uuid.UUID]

		Create(ctx context.Context, product *model.ProductModel) (*mapper.ProductMapper, error)
		GetByID(ctx context.Context, storeID, id uuid.UUID) (*mapper.ProductMapper, error)
		GetProducts(ctx context.Context, filter mapper.GetProductsFilter) ([]mapper.ProductMapper, error)
		Update(ctx context.Context, input mapper.UpdateProductInput) (*mapper.ProductMapper, error)
		SetStatus(ctx context.Context, storeID, id uuid.UUID, status model.ProductStatus) (*mapper.ProductMapper, error)
	}

	ProductRepository struct {
		*base.BaseRepositorySqlx[model.ProductModel, uuid.UUID]

		logger *zap.Logger
	}
)

func NewProductRepository(logger *zap.Logger, db *database.SqlxDatabase) IProductRepository {
	return &ProductRepository{
		BaseRepositorySqlx: base.NewBaseRepositorySqlx[model.ProductModel, uuid.UUID](db, "catalog.products"),
		logger:             logger,
	}
}

// Create inserts a product. Uniqueness is enforced by products_store_sku_unique,
// so callers must not SELECT the SKU before inserting.
func (r *ProductRepository) Create(ctx context.Context, product *model.ProductModel) (*mapper.ProductMapper, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (store_id, sku, name, description, price, currency)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING %s
	`, r.GetTableName(), productColumns)

	var row mapper.ProductMapper
	err := r.Sqlx.GetContext(ctx, &row, query,
		product.StoreID,
		product.SKU,
		product.Name,
		product.Description,
		product.Price,
		product.Currency,
	)
	if err != nil {
		if !isExpectedConstraint(err) {
			r.logger.Error("failed to create product", zap.Error(err))
		}
		return nil, fmt.Errorf("create product: %w", err)
	}
	return &row, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, storeID, id uuid.UUID) (*mapper.ProductMapper, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE store_id = $1 AND id = $2 AND deleted_at IS NULL
	`, productColumns, r.GetTableName())

	var row mapper.ProductMapper
	err := r.Sqlx.GetContext(ctx, &row, query, storeID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, base.ErrNotFound
	}
	if err != nil {
		r.logger.Error("failed to get product", zap.Error(err))
		return nil, fmt.Errorf("get product: %w", err)
	}
	return &row, nil
}

// GetProducts returns at most filter.Limit+1 rows ordered by created_at DESC, id DESC.
// The extra row lets the caller detect a next page without a second query.
func (r *ProductRepository) GetProducts(ctx context.Context, filter mapper.GetProductsFilter) ([]mapper.ProductMapper, error) {
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	args := make([]any, 0, 6)
	args = append(args, filter.StoreID, filter.Status)
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE store_id = $1
		  AND status = $2
		  AND deleted_at IS NULL
	`, productColumns, r.GetTableName())

	if search := strings.TrimSpace(filter.Search); search != "" {
		escaped := escapeLike(search)
		args = append(args, escaped+"%")
		skuPos := len(args)
		args = append(args, "%"+escaped+"%")
		namePos := len(args)
		query += fmt.Sprintf(`
		  AND (
		    sku ILIKE $%d
		    OR name ILIKE $%d
		  )`, skuPos, namePos)
	}

	if filter.CursorCreatedAt != nil && filter.CursorID != nil {
		args = append(args, *filter.CursorCreatedAt)
		timePos := len(args)
		args = append(args, *filter.CursorID)
		idPos := len(args)
		query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", timePos, idPos)
	}

	args = append(args, limit+1)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))

	products := make([]mapper.ProductMapper, 0)
	if err := r.Sqlx.SelectContext(ctx, &products, query, args...); err != nil {
		r.logger.Error("failed to get products", zap.Error(err))
		return nil, fmt.Errorf("get products: %w", err)
	}
	return products, nil
}

// Update writes the provided columns only when the stored version still matches.
// Zero rows means the product is missing in this store, or another write won the version.
func (r *ProductRepository) Update(ctx context.Context, input mapper.UpdateProductInput) (*mapper.ProductMapper, error) {
	sets := []string{"version = version + 1"}
	args := []any{input.StoreID, input.ID, input.Version}

	appendSet := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if input.SKU != nil {
		appendSet("sku", *input.SKU)
	}
	if input.Name != nil {
		appendSet("name", *input.Name)
	}
	if input.ClearDescription {
		sets = append(sets, "description = NULL")
	} else if input.Description != nil {
		appendSet("description", *input.Description)
	}
	if input.Price != nil {
		appendSet("price", *input.Price)
	}
	if input.Currency != nil {
		appendSet("currency", *input.Currency)
	}

	query := fmt.Sprintf(`
		UPDATE %s
		SET %s
		WHERE store_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NULL
		RETURNING %s
	`, r.GetTableName(), strings.Join(sets, ", "), productColumns)

	var row mapper.ProductMapper
	err := r.Sqlx.GetContext(ctx, &row, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, r.missingOrStale(ctx, input.StoreID, input.ID)
	}
	if err != nil {
		if !isExpectedConstraint(err) {
			r.logger.Error("failed to update product", zap.Error(err))
		}
		return nil, fmt.Errorf("update product: %w", err)
	}
	return &row, nil
}

// SetStatus archives or restores a product. A request that does not change status
// returns the current row and does not bump version.
func (r *ProductRepository) SetStatus(
	ctx context.Context,
	storeID, id uuid.UUID,
	status model.ProductStatus,
) (*mapper.ProductMapper, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = $3,
		    archived_at = CASE
		        WHEN $3 = 'archived' THEN COALESCE(archived_at, now())
		        ELSE NULL
		    END,
		    version = version + 1
		WHERE store_id = $1
		  AND id = $2
		  AND deleted_at IS NULL
		  AND status IS DISTINCT FROM $3
		RETURNING %s
	`, r.GetTableName(), productColumns)

	var row mapper.ProductMapper
	err := r.Sqlx.GetContext(ctx, &row, query, storeID, id, status)
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		r.logger.Error("failed to set product status", zap.Error(err))
		return nil, fmt.Errorf("set product status: %w", err)
	}

	current, getErr := r.GetByID(ctx, storeID, id)
	if getErr != nil {
		return nil, getErr
	}
	return current, nil
}

func (r *ProductRepository) missingOrStale(ctx context.Context, storeID, id uuid.UUID) error {
	_, err := r.GetByID(ctx, storeID, id)
	if errors.Is(err, base.ErrNotFound) {
		return base.ErrNotFound
	}
	if err != nil {
		return err
	}
	return base.ErrPreconditionFailed
}

func isExpectedConstraint(err error) bool {
	return database.IsUniqueViolation(err, "") ||
		database.IsForeignKeyViolation(err) ||
		database.IsCheckViolation(err)
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
