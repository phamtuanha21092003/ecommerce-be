package model

import (
	"time"
	"uuid"

	"github.com/shopspring/decimal"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
)

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusArchived ProductStatus = "archived"
)

type (
	ProductStatus string

	// ProductModel is the write-side entity for catalog.products.
	// Repository read methods return mapper.ProductMapper, not this struct.
	ProductModel struct {
		base.BaseModel[uuid.UUID]

		StoreID     uuid.UUID       `json:"storeId" db:"store_id"`
		SKU         string          `json:"sku" db:"sku"`
		Name        string          `json:"name" db:"name"`
		Description *string         `json:"description" db:"description"`
		Price       decimal.Decimal `json:"price" db:"price"`
		Currency    string          `json:"currency" db:"currency"`
		Status      ProductStatus   `json:"status" db:"status"`
		ArchivedAt  *time.Time      `json:"archivedAt" db:"archived_at"`
		Version     int             `json:"version" db:"version"`
	}
)
