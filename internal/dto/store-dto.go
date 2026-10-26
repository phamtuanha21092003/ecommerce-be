package dto

import (
	"uuid"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
)

type (
	CreateStoreRequest struct {
		Name string `json:"name" validate:"required"`
	}

	StoreResponse struct {
		base.BaseSchema[uuid.UUID]
		Name string `json:"name"`
	}
)
