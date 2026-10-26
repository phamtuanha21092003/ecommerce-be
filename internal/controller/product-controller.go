package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/service"
	"github.com/phamtuanha21092003/ecommerce-be/internal/transport/httpx"
)

type (
	IProductController interface {
		Create(w http.ResponseWriter, r *http.Request)
		GetProducts(w http.ResponseWriter, r *http.Request)
		GetProduct(w http.ResponseWriter, r *http.Request)
		Update(w http.ResponseWriter, r *http.Request)
		Archive(w http.ResponseWriter, r *http.Request)
		Restore(w http.ResponseWriter, r *http.Request)
	}

	ProductController struct {
		logger *zap.Logger

		productService service.IProductService
	}
)

func NewProductController(logger *zap.Logger, productService service.IProductService) IProductController {
	return &ProductController{
		logger:         logger,
		productService: productService,
	}
}

func (c *ProductController) Create(w http.ResponseWriter, r *http.Request) {
	storeID, err := parseUUIDParam(r, "storeID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var req dto.CreateProductRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	product, err := c.productService.Create(r.Context(), storeID, &req)
	writeProduct(w, product, err, true)
}

func (c *ProductController) GetProducts(w http.ResponseWriter, r *http.Request) {
	storeID, err := parseUUIDParam(r, "storeID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var query dto.GetProductsQuery
	if err := httpx.Query(r, &query); err != nil {
		httpx.Error(w, err)
		return
	}

	page, err := c.productService.GetProducts(r.Context(), storeID, &query)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Success(w, page)
}

func (c *ProductController) GetProduct(w http.ResponseWriter, r *http.Request) {
	storeID, productID, err := parseProductIDs(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	product, err := c.productService.GetProduct(r.Context(), storeID, productID)
	writeProduct(w, product, err, false)
}

func (c *ProductController) Update(w http.ResponseWriter, r *http.Request) {
	storeID, productID, err := parseProductIDs(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	version, err := parseIfMatch(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var req dto.UpdateProductRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	product, err := c.productService.Update(r.Context(), storeID, productID, version, &req)
	writeProduct(w, product, err, false)
}

func (c *ProductController) Archive(w http.ResponseWriter, r *http.Request) {
	c.changeStatus(w, r, true)
}

func (c *ProductController) Restore(w http.ResponseWriter, r *http.Request) {
	c.changeStatus(w, r, false)
}

func (c *ProductController) changeStatus(w http.ResponseWriter, r *http.Request, archive bool) {
	storeID, productID, err := parseProductIDs(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var product *dto.ProductResponse
	if archive {
		product, err = c.productService.Archive(r.Context(), storeID, productID)
	} else {
		product, err = c.productService.Restore(r.Context(), storeID, productID)
	}
	writeProduct(w, product, err, false)
}

func parseProductIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	storeID, err := parseUUIDParam(r, "storeID")
	if err != nil {
		return uuid.Nil(), uuid.Nil(), err
	}
	productID, err := parseUUIDParam(r, "productID")
	if err != nil {
		return uuid.Nil(), uuid.Nil(), err
	}
	return storeID, productID, nil
}

func writeProduct(w http.ResponseWriter, product *dto.ProductResponse, err error, created bool) {
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.Itoa(product.Version)))
	if created {
		httpx.Created(w, product)
		return
	}
	httpx.Success(w, product)
}

func parseIfMatch(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		return 0, base.PreconditionRequired(errors.New("If-Match header is required"))
	}

	raw = strings.TrimPrefix(raw, "W/")
	raw = strings.Trim(raw, `"`)
	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 {
		return 0, base.BadRequest(errors.New("invalid If-Match"))
	}
	return version, nil
}
