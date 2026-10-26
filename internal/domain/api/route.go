package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/transport/httpx"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

func newRouter(logger *zap.Logger, db *database.SqlxDatabase, controllers *controllerApi) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(httpx.CorsMiddleware)
	r.Use(httpx.RequestLoggerMiddleware(logger))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			httpx.Error(w, base.ServiceUnavailable(err))
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	stores := controllers.storeController
	products := controllers.productController
	uploads := controllers.uploadController

	r.Post("/api/v1/uploads", uploads.Create)
	r.Post("/api/v1/uploads/{uploadID}/parts/presign", uploads.PresignParts)
	r.Post("/api/v1/uploads/{uploadID}/complete", uploads.Complete)
	r.Delete("/api/v1/uploads/{uploadID}", uploads.Abort)

	r.Post("/api/v1/stores", stores.Create)
	r.Get("/api/v1/stores/{storeID}", stores.Get)
	r.Post("/api/v1/stores/{storeID}/products", products.Create)
	r.Get("/api/v1/stores/{storeID}/products", products.GetProducts)
	r.Get("/api/v1/stores/{storeID}/products/{productID}", products.GetProduct)
	r.Patch("/api/v1/stores/{storeID}/products/{productID}", products.Update)
	r.Post("/api/v1/stores/{storeID}/products/{productID}/archive", products.Archive)
	r.Post("/api/v1/stores/{storeID}/products/{productID}/restore", products.Restore)

	return r
}
