package storage

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// ObjectPart is one uploaded piece of a multipart object.
type ObjectPart struct {
	PartNumber int32
	ETag       string
	Size       int64
}

// MultipartStorage is the R2/S3 multipart surface the upload service needs.
type MultipartStorage interface {
	CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error)
	PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (string, error)
	ListParts(ctx context.Context, key, uploadID string) ([]ObjectPart, error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []ObjectPart) error
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
	ObjectExists(ctx context.Context, key string) (bool, error)
}

var _ MultipartStorage = (*S3Storage)(nil)

func (s *S3Storage) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	out, err := s.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	if out.UploadId == nil || *out.UploadId == "" {
		return "", errors.New("multipart upload id is empty")
	}
	return *out.UploadId, nil
}

func (s *S3Storage) PresignUploadPart(
	ctx context.Context,
	key, uploadID string,
	partNumber int32,
	expires time.Duration,
) (string, error) {
	out, err := s.PresignClient.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(s.Bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (s *S3Storage) ListParts(ctx context.Context, key, uploadID string) ([]ObjectPart, error) {
	pager := s3.NewListPartsPaginator(s.Client, &s3.ListPartsInput{
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})

	parts := make([]ObjectPart, 0)
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, part := range page.Parts {
			if part.PartNumber == nil || part.ETag == nil || part.Size == nil {
				return nil, errors.New("incomplete part metadata")
			}
			parts = append(parts, ObjectPart{
				PartNumber: *part.PartNumber,
				ETag:       *part.ETag,
				Size:       *part.Size,
			})
		}
	}
	return parts, nil
}

func (s *S3Storage) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []ObjectPart) error {
	completed := make([]types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		completed = append(completed, types.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int32(part.PartNumber),
		})
	}

	_, err := s.Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	return err
}

func (s *S3Storage) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := s.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if IsNotFound(err) {
		return nil
	}
	return err
}

func (s *S3Storage) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// IsNotFound reports a missing multipart upload or object.
func IsNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "NoSuchUpload", "NoSuchKey", "NotFound":
		return true
	default:
		return false
	}
}
