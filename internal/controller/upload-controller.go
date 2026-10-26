package controller

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/service"
	"github.com/phamtuanha21092003/ecommerce-be/internal/transport/httpx"
)

type (
	IUploadController interface {
		Create(w http.ResponseWriter, r *http.Request)
		PresignParts(w http.ResponseWriter, r *http.Request)
		Complete(w http.ResponseWriter, r *http.Request)
		Abort(w http.ResponseWriter, r *http.Request)
	}

	UploadController struct {
		logger *zap.Logger

		uploadService service.IUploadService
	}
)

func NewUploadController(logger *zap.Logger, uploadService service.IUploadService) IUploadController {
	return &UploadController{
		logger:        logger,
		uploadService: uploadService,
	}
}

func (c *UploadController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUploadRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	upload, err := c.uploadService.Create(r.Context(), &req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Created(w, upload)
}

func (c *UploadController) PresignParts(w http.ResponseWriter, r *http.Request) {
	uploadID, err := parseUUIDParam(r, "uploadID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var req dto.PresignPartsRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	parts, err := c.uploadService.PresignParts(r.Context(), uploadID, &req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Success(w, parts)
}

func (c *UploadController) Complete(w http.ResponseWriter, r *http.Request) {
	uploadID, err := parseUUIDParam(r, "uploadID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	var req dto.CompleteUploadRequest
	if err := httpx.Body(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	upload, err := c.uploadService.Complete(r.Context(), uploadID, &req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.Success(w, upload)
}

func (c *UploadController) Abort(w http.ResponseWriter, r *http.Request) {
	uploadID, err := parseUUIDParam(r, "uploadID")
	if err != nil {
		httpx.Error(w, err)
		return
	}

	if err := c.uploadService.Abort(r.Context(), uploadID); err != nil {
		httpx.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
