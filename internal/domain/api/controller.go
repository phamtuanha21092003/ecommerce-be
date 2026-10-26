package api

import (
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/controller"
)

type controllerApi struct {
	storeController   controller.IStoreController
	productController controller.IProductController
	uploadController  controller.IUploadController
}

func newController(logger *zap.Logger, services *serviceApi) *controllerApi {
	return &controllerApi{
		storeController:   controller.NewStoreController(logger, services.storeService),
		productController: controller.NewProductController(logger, services.productService),
		uploadController:  controller.NewUploadController(logger, services.uploadService),
	}
}
