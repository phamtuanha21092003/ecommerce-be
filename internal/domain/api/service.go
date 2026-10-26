package api

import (
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/service"
	"github.com/phamtuanha21092003/ecommerce-be/platform/storage"
)

type serviceApi struct {
	storeService   service.IStoreService
	productService service.IProductService
	uploadService  service.IUploadService
}

func newService(repository *repositoryApi, logger *zap.Logger, objects storage.MultipartStorage, staticURL string) *serviceApi {
	return &serviceApi{
		storeService:   service.NewStoreService(logger, repository.storeRepository),
		productService: service.NewProductService(logger, repository.productRepository),
		uploadService:  service.NewUploadService(logger, repository.uploadRepository, objects, staticURL),
	}
}
