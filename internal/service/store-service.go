package service

import (
	"context"
	"errors"
	"strings"
	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/internal/repository"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

type (
	IStoreService interface {
		Create(ctx context.Context, req *dto.CreateStoreRequest) (*dto.StoreResponse, error)
		Get(ctx context.Context, id uuid.UUID) (*dto.StoreResponse, error)
	}

	StoreService struct {
		logger *zap.Logger

		storeRepository repository.IStoreRepository
	}
)

func NewStoreService(logger *zap.Logger, storeRepository repository.IStoreRepository) IStoreService {
	return &StoreService{
		logger:          logger,
		storeRepository: storeRepository,
	}
}

func (s *StoreService) Create(ctx context.Context, req *dto.CreateStoreRequest) (*dto.StoreResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 255 {
		return nil, base.BadRequest(errors.New("name must be 1-255 characters"))
	}

	row, err := s.storeRepository.Create(ctx, &model.StoreModel{Name: name})
	if database.IsCheckViolation(err) {
		return nil, base.BadRequest(errors.New("name must be 1-255 characters"))
	}
	if err != nil {
		s.logger.Error("create store", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	response := toStoreResponse(row)
	return &response, nil
}

func (s *StoreService) Get(ctx context.Context, id uuid.UUID) (*dto.StoreResponse, error) {
	row, err := s.storeRepository.GetByID(ctx, id)
	if errors.Is(err, base.ErrNotFound) {
		return nil, base.NotFound(errors.New("store not found"))
	}
	if err != nil {
		s.logger.Error("get store", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	response := toStoreResponse(row)
	return &response, nil
}

func toStoreResponse(row *mapper.StoreMapper) dto.StoreResponse {
	return dto.StoreResponse{
		BaseSchema: base.BaseSchema[uuid.UUID]{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		},
		Name: row.Name,
	}
}
