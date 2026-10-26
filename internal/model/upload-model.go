package model

import (
	"time"

	"uuid"
)

const (
	UploadStatusPending   UploadStatus = "pending"
	UploadStatusCompleted UploadStatus = "completed"
	UploadStatusAborted   UploadStatus = "aborted"
)

type (
	UploadStatus string

	// UploadSessionModel is the write-side session for one multipart upload.
	UploadSessionModel struct {
		ID          uuid.UUID `db:"id"`
		ObjectKey   string    `db:"object_key"`
		S3UploadID  string    `db:"s3_upload_id"`
		FileName    string    `db:"file_name"`
		ContentType string    `db:"content_type"`
		FileSize    int64     `db:"file_size"`
		PartSize    int64     `db:"part_size"`
		PartCount   int       `db:"part_count"`
		ExpiresAt   time.Time `db:"expires_at"`
	}
)
