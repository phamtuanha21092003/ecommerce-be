package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/internal/repository"
	"github.com/phamtuanha21092003/ecommerce-be/platform/storage"
)

const (
	minPartSize      int64 = 5 * 1024 * 1024
	targetPartSize   int64 = 8 * 1024 * 1024
	maxPartCount           = 10000
	maxFileSize      int64 = 5 * 1024 * 1024 * 1024
	uploadSessionTTL       = 24 * time.Hour
	presignTTL             = 15 * time.Minute
)

var contentTypePattern = regexp.MustCompile(`^[a-z0-9][-a-z0-9.+]{0,126}/[a-z0-9][-a-z0-9.+]{0,126}$`)

type (
	IUploadService interface {
		Create(ctx context.Context, req *dto.CreateUploadRequest) (*dto.CreateUploadResponse, error)
		PresignParts(ctx context.Context, id uuid.UUID, req *dto.PresignPartsRequest) (*dto.PresignPartsResponse, error)
		Complete(ctx context.Context, id uuid.UUID, req *dto.CompleteUploadRequest) (*dto.CompleteUploadResponse, error)
		Abort(ctx context.Context, id uuid.UUID) error
	}

	UploadService struct {
		logger *zap.Logger

		uploadRepository repository.IUploadRepository
		objects          storage.MultipartStorage
		staticURL        string
	}
)

func NewUploadService(
	logger *zap.Logger,
	uploadRepository repository.IUploadRepository,
	objects storage.MultipartStorage,
	staticURL string,
) IUploadService {
	return &UploadService{
		logger:           logger,
		uploadRepository: uploadRepository,
		objects:          objects,
		staticURL:        strings.TrimRight(staticURL, "/"),
	}
}

func (s *UploadService) Create(ctx context.Context, req *dto.CreateUploadRequest) (*dto.CreateUploadResponse, error) {
	fileName, objectName, err := normalizeFileName(req.FileName)
	if err != nil {
		return nil, err
	}
	contentType, err := normalizeContentType(req.ContentType)
	if err != nil {
		return nil, err
	}
	partSize, partCount, err := planUploadParts(req.FileSize)
	if err != nil {
		return nil, err
	}

	id := uuid.NewV7()

	now := time.Now().UTC()
	objectKey := fmt.Sprintf("uploads/%s/%s/%s", now.Format("2006/01/02"), id.String(), objectName)
	expiresAt := now.Add(uploadSessionTTL)

	s3UploadID, err := s.objects.CreateMultipartUpload(ctx, objectKey, contentType)
	if err != nil {
		s.logger.Error("create multipart upload", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}

	row, err := s.uploadRepository.Create(ctx, &model.UploadSessionModel{
		ID:          id,
		ObjectKey:   objectKey,
		S3UploadID:  s3UploadID,
		FileName:    fileName,
		ContentType: contentType,
		FileSize:    req.FileSize,
		PartSize:    partSize,
		PartCount:   partCount,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		if abortErr := s.objects.AbortMultipartUpload(ctx, objectKey, s3UploadID); abortErr != nil {
			s.logger.Error("abort multipart upload after database failure", zap.Error(abortErr))
		}
		s.logger.Error("save upload session", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}

	return &dto.CreateUploadResponse{
		UploadID:  row.ID,
		ObjectKey: row.ObjectKey,
		PartSize:  row.PartSize,
		PartCount: row.PartCount,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *UploadService) PresignParts(ctx context.Context, id uuid.UUID, req *dto.PresignPartsRequest) (*dto.PresignPartsResponse, error) {
	row, err := s.loadOpen(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validatePartNumbers(req.PartNumbers, row.PartCount); err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(presignTTL)
	parts := make([]dto.PresignPartResponse, 0, len(req.PartNumbers))
	for _, number := range req.PartNumbers {
		url, err := s.objects.PresignUploadPart(ctx, row.ObjectKey, row.S3UploadID, number, presignTTL)
		if err != nil {
			s.logger.Error("presign upload part", zap.Error(err), zap.Int32("part", number))
			return nil, base.InternalServerError(errors.New("internal server error"))
		}
		parts = append(parts, dto.PresignPartResponse{
			PartNumber: number,
			URL:        url,
			ExpiresAt:  expiresAt,
		})
	}

	return &dto.PresignPartsResponse{Parts: parts}, nil
}

func (s *UploadService) Complete(ctx context.Context, id uuid.UUID, req *dto.CompleteUploadRequest) (*dto.CompleteUploadResponse, error) {
	row, err := s.uploadRepository.GetByID(ctx, id)
	if errors.Is(err, base.ErrNotFound) {
		return nil, base.NotFound(errors.New("upload not found"))
	}
	if err != nil {
		s.logger.Error("get upload session", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	if row.Status == model.UploadStatusCompleted {
		return s.completedResponse(row), nil
	}
	if row.Status == model.UploadStatusAborted {
		return nil, base.Conflict(errors.New("upload was aborted"))
	}
	if err := s.expireIfNeeded(ctx, row); err != nil {
		return nil, err
	}

	remote, err := s.objects.ListParts(ctx, row.ObjectKey, row.S3UploadID)
	if storage.IsNotFound(err) {
		return s.finishIfObjectExists(ctx, row)
	}
	if err != nil {
		s.logger.Error("list upload parts", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}

	completed, err := matchParts(row, req.Parts, remote)
	if err != nil {
		return nil, err
	}
	if err := s.objects.CompleteMultipartUpload(ctx, row.ObjectKey, row.S3UploadID, completed); err != nil {
		if storage.IsNotFound(err) {
			return s.finishIfObjectExists(ctx, row)
		}
		s.logger.Error("complete multipart upload", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}

	return s.markCompleted(ctx, row)
}

func (s *UploadService) Abort(ctx context.Context, id uuid.UUID) error {
	row, err := s.uploadRepository.GetByID(ctx, id)
	if errors.Is(err, base.ErrNotFound) {
		return base.NotFound(errors.New("upload not found"))
	}
	if err != nil {
		s.logger.Error("get upload session", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}
	if row.Status == model.UploadStatusAborted {
		return nil
	}
	if row.Status == model.UploadStatusCompleted {
		return base.Conflict(errors.New("upload is already completed"))
	}

	if err := s.objects.AbortMultipartUpload(ctx, row.ObjectKey, row.S3UploadID); err != nil {
		s.logger.Error("abort multipart upload", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}

	updated, err := s.uploadRepository.MarkAborted(ctx, row.ID)
	if errors.Is(err, base.ErrConflict) {
		if updated != nil && updated.Status == model.UploadStatusAborted {
			return nil
		}
		return base.Conflict(errors.New("upload is already completed"))
	}
	if err != nil {
		s.logger.Error("mark upload aborted", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}
	return nil
}

func (s *UploadService) loadOpen(ctx context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	row, err := s.uploadRepository.GetByID(ctx, id)
	if errors.Is(err, base.ErrNotFound) {
		return nil, base.NotFound(errors.New("upload not found"))
	}
	if err != nil {
		s.logger.Error("get upload session", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	switch row.Status {
	case model.UploadStatusCompleted:
		return nil, base.Conflict(errors.New("upload is already completed"))
	case model.UploadStatusAborted:
		return nil, base.Conflict(errors.New("upload was aborted"))
	}
	if err := s.expireIfNeeded(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *UploadService) expireIfNeeded(ctx context.Context, row *mapper.UploadSessionMapper) error {
	if time.Now().Before(row.ExpiresAt) {
		return nil
	}
	if err := s.objects.AbortMultipartUpload(ctx, row.ObjectKey, row.S3UploadID); err != nil {
		s.logger.Error("abort expired multipart upload", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}
	if _, err := s.uploadRepository.MarkAborted(ctx, row.ID); err != nil && !errors.Is(err, base.ErrConflict) {
		s.logger.Error("mark expired upload aborted", zap.Error(err))
		return base.InternalServerError(errors.New("internal server error"))
	}
	return base.Conflict(errors.New("upload has expired"))
}

func (s *UploadService) finishIfObjectExists(ctx context.Context, row *mapper.UploadSessionMapper) (*dto.CompleteUploadResponse, error) {
	exists, err := s.objects.ObjectExists(ctx, row.ObjectKey)
	if err != nil {
		s.logger.Error("head uploaded object", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	if !exists {
		return nil, base.Conflict(errors.New("upload is no longer active"))
	}
	return s.markCompleted(ctx, row)
}

func (s *UploadService) markCompleted(ctx context.Context, row *mapper.UploadSessionMapper) (*dto.CompleteUploadResponse, error) {
	updated, err := s.uploadRepository.MarkCompleted(ctx, row.ID)
	if errors.Is(err, base.ErrConflict) {
		if updated != nil && updated.Status == model.UploadStatusCompleted {
			return s.completedResponse(updated), nil
		}
		return nil, base.Conflict(errors.New("upload is not pending"))
	}
	if err != nil {
		s.logger.Error("mark upload completed", zap.Error(err))
		return nil, base.InternalServerError(errors.New("internal server error"))
	}
	return s.completedResponse(updated), nil
}

func (s *UploadService) completedResponse(row *mapper.UploadSessionMapper) *dto.CompleteUploadResponse {
	return &dto.CompleteUploadResponse{
		UploadID:    row.ID,
		ObjectKey:   row.ObjectKey,
		URL:         s.staticURL + "/" + row.ObjectKey,
		FileSize:    row.FileSize,
		ContentType: row.ContentType,
	}
}

func planUploadParts(fileSize int64) (int64, int, error) {
	if fileSize <= 0 {
		return 0, 0, base.BadRequest(errors.New("fileSize must be greater than 0"))
	}
	if fileSize > maxFileSize {
		return 0, 0, base.BadRequest(errors.New("fileSize exceeds 5 GiB"))
	}
	if fileSize <= minPartSize {
		return fileSize, 1, nil
	}

	partSize := targetPartSize
	partCount := int((fileSize + partSize - 1) / partSize)
	if partCount > maxPartCount {
		partSize = (fileSize + maxPartCount - 1) / maxPartCount
		if partSize < minPartSize {
			partSize = minPartSize
		}
		partCount = int((fileSize + partSize - 1) / partSize)
	}
	return partSize, partCount, nil
}

func normalizeFileName(raw string) (string, string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 255 {
		return "", "", base.BadRequest(errors.New("fileName must be 1-255 characters"))
	}
	fileBase := filepath.Base(name)
	if fileBase == "." || fileBase == ".." {
		return "", "", base.BadRequest(errors.New("fileName is invalid"))
	}

	var safe strings.Builder
	for _, r := range fileBase {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			safe.WriteRune(r)
		default:
			safe.WriteByte('_')
		}
	}
	objectName := strings.Trim(safe.String(), "._")
	if objectName == "" {
		objectName = "file"
	}
	if len(objectName) > 200 {
		objectName = objectName[:200]
	}
	return name, objectName, nil
}

func normalizeContentType(raw string) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(raw))
	if !contentTypePattern.MatchString(contentType) {
		return "", base.BadRequest(errors.New("contentType must be a media type such as image/jpeg"))
	}
	return contentType, nil
}

func validatePartNumbers(numbers []int32, partCount int) error {
	seen := make(map[int32]struct{}, len(numbers))
	for _, number := range numbers {
		if number < 1 || int(number) > partCount {
			return base.BadRequest(fmt.Errorf("partNumber %d is outside 1-%d", number, partCount))
		}
		if _, ok := seen[number]; ok {
			return base.BadRequest(fmt.Errorf("partNumber %d is duplicated", number))
		}
		seen[number] = struct{}{}
	}
	return nil
}

func matchParts(row *mapper.UploadSessionMapper, claimed []dto.CompleteUploadPart, remote []storage.ObjectPart) ([]storage.ObjectPart, error) {
	if len(claimed) != row.PartCount {
		return nil, base.BadRequest(fmt.Errorf("expected %d parts", row.PartCount))
	}

	claimedByNumber := make(map[int32]string, len(claimed))
	for _, part := range claimed {
		if part.PartNumber < 1 || int(part.PartNumber) > row.PartCount {
			return nil, base.BadRequest(fmt.Errorf("partNumber %d is outside 1-%d", part.PartNumber, row.PartCount))
		}
		if _, ok := claimedByNumber[part.PartNumber]; ok {
			return nil, base.BadRequest(fmt.Errorf("partNumber %d is duplicated", part.PartNumber))
		}
		claimedByNumber[part.PartNumber] = normalizeETag(part.ETag)
	}

	remoteByNumber := make(map[int32]storage.ObjectPart, len(remote))
	for _, part := range remote {
		if _, ok := remoteByNumber[part.PartNumber]; ok {
			return nil, base.BadRequest(fmt.Errorf("partNumber %d was uploaded more than once", part.PartNumber))
		}
		remoteByNumber[part.PartNumber] = part
	}
	if len(remoteByNumber) != row.PartCount {
		return nil, base.BadRequest(fmt.Errorf("expected %d uploaded parts", row.PartCount))
	}

	completed := make([]storage.ObjectPart, 0, row.PartCount)
	for number := int32(1); number <= int32(row.PartCount); number++ {
		uploaded, ok := remoteByNumber[number]
		if !ok {
			return nil, base.BadRequest(fmt.Errorf("part %d has not been uploaded", number))
		}
		if normalizeETag(uploaded.ETag) != claimedByNumber[number] {
			return nil, base.BadRequest(fmt.Errorf("part %d etag does not match", number))
		}
		expectedSize := row.PartSize
		if int(number) == row.PartCount {
			expectedSize = row.FileSize - row.PartSize*int64(row.PartCount-1)
		}
		if uploaded.Size != expectedSize {
			return nil, base.BadRequest(fmt.Errorf("part %d has unexpected size", number))
		}
		completed = append(completed, storage.ObjectPart{
			PartNumber: number,
			ETag:       uploaded.ETag,
			Size:       uploaded.Size,
		})
	}
	return completed, nil
}

func normalizeETag(etag string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(etag), `"`))
}
