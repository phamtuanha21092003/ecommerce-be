package controller

import (
	"errors"
	"net/http"
	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/service"
	"github.com/phamtuanha21092003/ecommerce-be/internal/transport/httpx"
)

type (
	IStoreController interface {
		Create(w http.ResponseWriter, r *http.Request)
		Get(w http.ResponseWriter, r *http.Request)
	}

	StoreController struct {
		logger *zap.Logger

		storeService service.IStoreService
	}
)

func NewStoreController(logger *zap.Logger, storeService service.IStoreService) IStoreController {
	return &StoreController{
		logger:       logger,
		storeService: storeService,
	}
}

func (c *StoreController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateStoreRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	store, err := c.storeService.Create(r.Context(), &req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Created(w, store)
}

func (c *StoreController) Get(w http.ResponseWriter, r *http.Request) {
	storeID, err := parseUUIDParam(r, "storeID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	store, err := c.storeService.Get(r.Context(), storeID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Success(w, store)
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(httpx.Param(r, name))
	if err != nil {
		return uuid.Nil(), base.BadRequest(errors.New("invalid " + name))
	}
	return id, nil
}
