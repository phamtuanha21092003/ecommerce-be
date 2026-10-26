package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"uuid"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/internal/repository"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

var (
	skuPattern      = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

type (
	IProductService interface {
		Create(ctx context.Context, storeID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error)
		GetProducts(ctx context.Context, storeID uuid.UUID, query *dto.GetProductsQuery) (*base.CursorPagingResponseDto[dto.ProductResponse], error)
		GetProduct(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error)
		Update(ctx context.Context, storeID, productID uuid.UUID, version int, req *dto.UpdateProductRequest) (*dto.ProductResponse, error)
		Archive(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error)
		Restore(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error)
	}

	ProductService struct {
		logger *zap.Logger

		productRepository repository.IProductRepository
	}
)

func NewProductService(logger *zap.Logger, productRepository repository.IProductRepository) IProductService {
	return &ProductService{
		logger:            logger,
		productRepository: productRepository,
	}
}

func (s *ProductService) Create(ctx context.Context, storeID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
	sku, err := normalizeSKU(req.SKU)
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}
	description, err := normalizeDescription(req.Description)
	if err != nil {
		return nil, err
	}
	currency, err := normalizeCurrency(req.Currency)
	if err != nil {
		return nil, err
	}
	if req.Price == nil {
		return nil, base.BadRequest(errors.New("price is required"))
	}
	if err := validatePrice(*req.Price); err != nil {
		return nil, err
	}

	row, err := s.productRepository.Create(ctx, &model.ProductModel{
		StoreID:     storeID,
		SKU:         sku,
		Name:        name,
		Description: description,
		Price:       *req.Price,
		Currency:    currency,
	})
	if err != nil {
		return nil, s.mapErr(err, "store not found", "sku already exists in this store")
	}
	response := toProductResponse(row)
	return &response, nil
}

func (s *ProductService) GetProducts(
	ctx context.Context,
	storeID uuid.UUID,
	query *dto.GetProductsQuery,
) (*base.CursorPagingResponseDto[dto.ProductResponse], error) {
	query.Normalize()

	status := model.ProductStatusActive
	if query.Status != "" {
		status = model.ProductStatus(query.Status)
	}

	filter := mapper.GetProductsFilter{
		StoreID: storeID,
		Status:  status,
		Search:  strings.TrimSpace(query.Q),
		Limit:   query.Limit,
	}
	if query.Cursor != "" {
		createdAt, id, err := base.DecodeCursor(query.Cursor)
		if err != nil {
			return nil, base.BadRequest(errors.New("invalid cursor"))
		}
		cursorID, err := uuid.Parse(id)
		if err != nil {
			return nil, base.BadRequest(errors.New("invalid cursor"))
		}
		filter.CursorCreatedAt = &createdAt
		filter.CursorID = &cursorID
	}

	rows, err := s.productRepository.GetProducts(ctx, filter)
	if err != nil {
		s.logger.Error("get products", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}

	hasNext := len(rows) > query.Limit
	if hasNext {
		rows = rows[:query.Limit]
	}

	items := make([]dto.ProductResponse, 0, len(rows))
	for i := range rows {
		items = append(items, toProductResponse(&rows[i]))
	}

	var nextCursor *string
	if hasNext && len(rows) > 0 {
		last := rows[len(rows)-1]
		encoded := base.EncodeCursor(last.CreatedAt, last.ID.String())
		nextCursor = &encoded
	}

	return &base.CursorPagingResponseDto[dto.ProductResponse]{
		Items:      items,
		Limit:      query.Limit,
		HasNext:    hasNext,
		NextCursor: nextCursor,
	}, nil
}

func (s *ProductService) GetProduct(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error) {
	row, err := s.productRepository.GetByID(ctx, storeID, productID)
	if err != nil {
		return nil, s.mapErr(err, "product not found", "")
	}
	response := toProductResponse(row)
	return &response, nil
}

func (s *ProductService) Update(
	ctx context.Context,
	storeID, productID uuid.UUID,
	version int,
	req *dto.UpdateProductRequest,
) (*dto.ProductResponse, error) {
	input, err := buildUpdate(storeID, productID, version, req)
	if err != nil {
		return nil, err
	}

	row, err := s.productRepository.Update(ctx, input)
	if err != nil {
		return nil, s.mapErr(err, "product not found", "sku already exists in this store")
	}
	response := toProductResponse(row)
	return &response, nil
}

func (s *ProductService) Archive(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error) {
	return s.setStatus(ctx, storeID, productID, model.ProductStatusArchived)
}

func (s *ProductService) Restore(ctx context.Context, storeID, productID uuid.UUID) (*dto.ProductResponse, error) {
	return s.setStatus(ctx, storeID, productID, model.ProductStatusActive)
}

func (s *ProductService) setStatus(
	ctx context.Context,
	storeID, productID uuid.UUID,
	status model.ProductStatus,
) (*dto.ProductResponse, error) {
	row, err := s.productRepository.SetStatus(ctx, storeID, productID, status)
	if err != nil {
		return nil, s.mapErr(err, "product not found", "")
	}
	response := toProductResponse(row)
	return &response, nil
}

func (s *ProductService) mapErr(err error, notFoundMsg, conflictMsg string) error {
	switch {
	case errors.Is(err, base.ErrNotFound):
		return base.NotFound(errors.New(notFoundMsg))
	case errors.Is(err, base.ErrPreconditionFailed):
		return base.PreconditionFailed(errors.New("product was modified by another request"))
	case database.IsUniqueViolation(err, "products_store_sku_unique"):
		return base.Conflict(errors.New(conflictMsg))
	case database.IsForeignKeyViolation(err):
		return base.NotFound(errors.New("store not found"))
	case database.IsCheckViolation(err):
		return base.BadRequest(errors.New("product data violates a database constraint"))
	default:
		s.logger.Error("product service", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}
}

func buildUpdate(storeID, productID uuid.UUID, version int, req *dto.UpdateProductRequest) (mapper.UpdateProductInput, error) {
	if !req.SKU.Set && !req.Name.Set && !req.Description.Set && !req.Price.Set && !req.Currency.Set {
		return mapper.UpdateProductInput{}, base.BadRequest(errors.New("at least one field is required"))
	}

	input := mapper.UpdateProductInput{
		StoreID: storeID,
		ID:      productID,
		Version: version,
	}

	if req.SKU.Set {
		if req.SKU.Null {
			return mapper.UpdateProductInput{}, base.BadRequest(errors.New("sku cannot be null"))
		}
		sku, err := normalizeSKU(req.SKU.Value)
		if err != nil {
			return mapper.UpdateProductInput{}, err
		}
		input.SKU = &sku
	}

	if req.Name.Set {
		if req.Name.Null {
			return mapper.UpdateProductInput{}, base.BadRequest(errors.New("name cannot be null"))
		}
		name, err := normalizeName(req.Name.Value)
		if err != nil {
			return mapper.UpdateProductInput{}, err
		}
		input.Name = &name
	}

	if req.Description.Set {
		if req.Description.Null {
			input.ClearDescription = true
		} else {
			description, err := normalizeDescription(&req.Description.Value)
			if err != nil {
				return mapper.UpdateProductInput{}, err
			}
			if description == nil {
				input.ClearDescription = true
			} else {
				input.Description = description
			}
		}
	}

	if req.Price.Set {
		if req.Price.Null {
			return mapper.UpdateProductInput{}, base.BadRequest(errors.New("price cannot be null"))
		}
		if err := validatePrice(req.Price.Value); err != nil {
			return mapper.UpdateProductInput{}, err
		}
		price := req.Price.Value
		input.Price = &price
	}

	if req.Currency.Set {
		if req.Currency.Null {
			return mapper.UpdateProductInput{}, base.BadRequest(errors.New("currency cannot be null"))
		}
		currency, err := normalizeCurrency(req.Currency.Value)
		if err != nil {
			return mapper.UpdateProductInput{}, err
		}
		input.Currency = &currency
	}

	return input, nil
}

func normalizeSKU(raw string) (string, error) {
	sku := strings.ToUpper(strings.TrimSpace(raw))
	if !skuPattern.MatchString(sku) {
		return "", base.BadRequest(errors.New("sku must be 1-64 characters using A-Z, 0-9, _ or -"))
	}
	return sku, nil
}

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 255 {
		return "", base.BadRequest(errors.New("name must be 1-255 characters"))
	}
	return name, nil
}

func normalizeDescription(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	description := strings.TrimSpace(*raw)
	if description == "" {
		return nil, nil
	}
	if len(description) > 5000 {
		return nil, base.BadRequest(errors.New("description must be at most 5000 characters"))
	}
	return &description, nil
}

func normalizeCurrency(raw string) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(raw))
	if !currencyPattern.MatchString(currency) {
		return "", base.BadRequest(errors.New("currency must be a 3-letter ISO code"))
	}
	return currency, nil
}

func validatePrice(price decimal.Decimal) error {
	if price.IsNegative() {
		return base.BadRequest(errors.New("price must be greater than or equal to 0"))
	}
	if price.Exponent() < -4 {
		return base.BadRequest(errors.New("price supports at most 4 decimal places"))
	}
	digits := len(price.Coefficient().Text(10))
	if digits > 19 {
		return base.BadRequest(errors.New("price exceeds NUMERIC(19,4)"))
	}
	return nil
}

func toProductResponse(row *mapper.ProductMapper) dto.ProductResponse {
	return dto.ProductResponse{
		BaseSchema: base.BaseSchema[uuid.UUID]{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		},
		StoreID:     row.StoreID,
		SKU:         row.SKU,
		Name:        row.Name,
		Description: row.Description,
		Price:       row.Price,
		Currency:    strings.TrimSpace(row.Currency),
		Status:      string(row.Status),
		ArchivedAt:  row.ArchivedAt,
		Version:     row.Version,
	}
}
