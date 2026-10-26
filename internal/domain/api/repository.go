package api

import (
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/repository"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

type repositoryApi struct {
	storeRepository   repository.IStoreRepository
	productRepository repository.IProductRepository
	uploadRepository  repository.IUploadRepository
}

func newRepository(db *database.SqlxDatabase, logger *zap.Logger) *repositoryApi {
	return &repositoryApi{
		storeRepository:   repository.NewStoreRepository(logger, db),
		productRepository: repository.NewProductRepository(logger, db),
		uploadRepository:  repository.NewUploadRepository(logger, db),
	}
}
