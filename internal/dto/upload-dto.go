package dto

import (
	"time"

	"uuid"
)

type (
	CreateUploadRequest struct {
		FileName    string `json:"fileName" validate:"required"`
		ContentType string `json:"contentType" validate:"required"`
		FileSize    int64  `json:"fileSize" validate:"required,gt=0"`
	}

	CreateUploadResponse struct {
		UploadID  uuid.UUID `json:"uploadId"`
		ObjectKey string    `json:"objectKey"`
		PartSize  int64     `json:"partSize"`
		PartCount int       `json:"partCount"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	PresignPartsRequest struct {
		PartNumbers []int32 `json:"partNumbers" validate:"required,min=1,max=100,dive,gte=1"`
	}

	PresignPartResponse struct {
		PartNumber int32     `json:"partNumber"`
		URL        string    `json:"url"`
		ExpiresAt  time.Time `json:"expiresAt"`
	}

	PresignPartsResponse struct {
		Parts []PresignPartResponse `json:"parts"`
	}

	CompleteUploadPart struct {
		PartNumber int32  `json:"partNumber" validate:"required,gte=1"`
		ETag       string `json:"etag" validate:"required"`
	}

	CompleteUploadRequest struct {
		Parts []CompleteUploadPart `json:"parts" validate:"required,min=1,dive"`
	}

	CompleteUploadResponse struct {
		UploadID    uuid.UUID `json:"uploadId"`
		ObjectKey   string    `json:"objectKey"`
		URL         string    `json:"url"`
		FileSize    int64     `json:"fileSize"`
		ContentType string    `json:"contentType"`
	}
)
