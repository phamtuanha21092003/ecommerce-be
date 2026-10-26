package dto

import (
	"time"
	"uuid"

	"github.com/shopspring/decimal"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
)

type (
	CreateProductRequest struct {
		SKU         string           `json:"sku" validate:"required"`
		Name        string           `json:"name" validate:"required"`
		Description *string          `json:"description" validate:"omitempty,max=5000"`
		Price       *decimal.Decimal `json:"price" validate:"required"`
		Currency    string           `json:"currency" validate:"required"`
	}

	// UpdateProductRequest uses Optional so omitted, null, and empty are different cases.
	UpdateProductRequest struct {
		SKU         base.Optional[string]          `json:"sku"`
		Name        base.Optional[string]          `json:"name"`
		Description base.Optional[string]          `json:"description"`
		Price       base.Optional[decimal.Decimal] `json:"price"`
		Currency    base.Optional[string]          `json:"currency"`
	}

	GetProductsQuery struct {
		base.CursorPagingInput
		Status string `query:"status" validate:"omitempty,oneof=active archived"`
		Q      string `query:"q" validate:"omitempty,max=200"`
	}

	ProductResponse struct {
		base.BaseSchema[uuid.UUID]
		StoreID     uuid.UUID       `json:"storeId"`
		SKU         string          `json:"sku"`
		Name        string          `json:"name"`
		Description *string         `json:"description"`
		Price       decimal.Decimal `json:"price"`
		Currency    string          `json:"currency"`
		Status      string          `json:"status"`
		ArchivedAt  *time.Time      `json:"archivedAt"`
		Version     int             `json:"version"`
	}
)
