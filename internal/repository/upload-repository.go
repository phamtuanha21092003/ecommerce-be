package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

const uploadSessionColumns = `
	id,
	object_key,
	s3_upload_id,
	file_name,
	content_type,
	file_size,
	part_size,
	part_count,
	status,
	expires_at,
	completed_at,
	created_at,
	updated_at
`

type (
	IUploadRepository interface {
		base.IBaseRepositorySqlx[model.UploadSessionModel, uuid.UUID]

		Create(ctx context.Context, session *model.UploadSessionModel) (*mapper.UploadSessionMapper, error)
		GetByID(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error)
		MarkCompleted(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error)
		MarkAborted(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error)
	}

	UploadRepository struct {
		*base.BaseRepositorySqlx[model.UploadSessionModel, uuid.UUID]

		logger *zap.Logger
	}
)

func NewUploadRepository(logger *zap.Logger, db *database.SqlxDatabase) IUploadRepository {
	return &UploadRepository{
		BaseRepositorySqlx: base.NewBaseRepositorySqlx[model.UploadSessionModel, uuid.UUID](db, "storage.upload_sessions"),
		logger:             logger,
	}
}

func (r *UploadRepository) Create(ctx context.Context, session *model.UploadSessionModel) (*mapper.UploadSessionMapper, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, object_key, s3_upload_id, file_name, content_type,
			file_size, part_size, part_count, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING %s
	`, r.GetTableName(), uploadSessionColumns)

	var row mapper.UploadSessionMapper
	err := r.Sqlx.GetContext(ctx, &row, query,
		session.ID,
		session.ObjectKey,
		session.S3UploadID,
		session.FileName,
		session.ContentType,
		session.FileSize,
		session.PartSize,
		session.PartCount,
		session.ExpiresAt,
	)
	if err != nil {
		r.logger.Error("failed to create upload session", zap.Error(err))
		return nil, fmt.Errorf("create upload session: %w", err)
	}
	return &row, nil
}

func (r *UploadRepository) GetByID(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE id = $1
	`, uploadSessionColumns, r.GetTableName())

	var row mapper.UploadSessionMapper
	err := r.Sqlx.GetContext(ctx, &row, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, base.ErrNotFound
	}
	if err != nil {
		r.logger.Error("failed to get upload session", zap.Error(err))
		return nil, fmt.Errorf("get upload session: %w", err)
	}
	return &row, nil
}

func (r *UploadRepository) MarkCompleted(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = 'completed',
		    completed_at = now()
		WHERE id = $1 AND status = 'pending'
		RETURNING %s
	`, r.GetTableName(), uploadSessionColumns)
	return r.mark(ctx, id, query, "complete upload session")
}

func (r *UploadRepository) MarkAborted(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = 'aborted'
		WHERE id = $1 AND status = 'pending'
		RETURNING %s
	`, r.GetTableName(), uploadSessionColumns)
	return r.mark(ctx, id, query, "abort upload session")
}

func (r *UploadRepository) mark(ctx context.Context, id uuid.UUID, query, action string) (*mapper.UploadSessionMapper, error) {
	var row mapper.UploadSessionMapper
	err := r.Sqlx.GetContext(ctx, &row, query, id)
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		r.logger.Error("failed to "+action, zap.Error(err))
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	current, getErr := r.GetByID(ctx, id)
	if getErr != nil {
		return nil, getErr
	}
	return current, base.ErrConflict
}
