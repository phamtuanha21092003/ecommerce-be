package mapper

import (
	"time"
	"uuid"

	"github.com/shopspring/decimal"

	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
)

// ProductMapper is the row shape returned by product queries.
type ProductMapper struct {
	ID          uuid.UUID           `db:"id"`
	StoreID     uuid.UUID           `db:"store_id"`
	SKU         string              `db:"sku"`
	Name        string              `db:"name"`
	Description *string             `db:"description"`
	Price       decimal.Decimal     `db:"price"`
	Currency    string              `db:"currency"`
	Status      model.ProductStatus `db:"status"`
	CreatedAt   time.Time           `db:"created_at"`
	UpdatedAt   time.Time           `db:"updated_at"`
	ArchivedAt  *time.Time          `db:"archived_at"`
	Version     int                 `db:"version"`
}

// GetProductsFilter is the typed keyset query for one store.
// Limit is the page size; the repository fetches one extra row to detect the next page.
type GetProductsFilter struct {
	StoreID         uuid.UUID
	Status          model.ProductStatus
	Search          string
	CursorCreatedAt *time.Time
	CursorID        *uuid.UUID
	Limit           int
}

// UpdateProductInput carries only the columns a PATCH request actually set.
// ClearDescription stores NULL. A nil Description pointer leaves the column unchanged.
type UpdateProductInput struct {
	StoreID          uuid.UUID
	ID               uuid.UUID
	Version          int
	SKU              *string
	Name             *string
	Description      *string
	ClearDescription bool
	Price            *decimal.Decimal
	Currency         *string
}
