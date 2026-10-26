package mapper

import (
	"time"

	"uuid"

	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
)

// UploadSessionMapper is the row returned by upload session queries.
type UploadSessionMapper struct {
	ID          uuid.UUID          `db:"id"`
	ObjectKey   string             `db:"object_key"`
	S3UploadID  string             `db:"s3_upload_id"`
	FileName    string             `db:"file_name"`
	ContentType string             `db:"content_type"`
	FileSize    int64              `db:"file_size"`
	PartSize    int64              `db:"part_size"`
	PartCount   int                `db:"part_count"`
	Status      model.UploadStatus `db:"status"`
	ExpiresAt   time.Time          `db:"expires_at"`
	CompletedAt *time.Time         `db:"completed_at"`
	CreatedAt   time.Time          `db:"created_at"`
	UpdatedAt   time.Time          `db:"updated_at"`
}
