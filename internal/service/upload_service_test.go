package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/aws/smithy-go"
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/dto"
	"github.com/phamtuanha21092003/ecommerce-be/internal/mapper"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/platform/storage"
)

func TestPlanUploadParts(t *testing.T) {
	size, count, err := planUploadParts(1024)
	if err != nil || size != 1024 || count != 1 {
		t.Fatalf("small file = %d parts %d err %v", size, count, err)
	}

	size, count, err = planUploadParts(10 * 1024 * 1024)
	if err != nil || size != targetPartSize || count != 2 {
		t.Fatalf("10MiB = %d parts %d err %v", size, count, err)
	}

	if _, _, err := planUploadParts(maxFileSize + 1); err == nil {
		t.Fatal("expected file size limit")
	}
}

func TestUploadCreateRollsBackRemoteSession(t *testing.T) {
	objects := &fakeStorage{uploadID: "remote-1"}
	repo := &fakeUploadRepo{createErr: errCreate}
	svc := NewUploadService(zap.NewNop(), repo, objects, "https://static.example.com")

	_, err := svc.Create(context.Background(), &dto.CreateUploadRequest{
		FileName:    "photo.jpg",
		ContentType: "image/jpeg",
		FileSize:    1024,
	})
	if err == nil {
		t.Fatal("expected create error")
	}
	if !objects.aborted {
		t.Fatal("remote multipart upload was not aborted")
	}
}

func TestUploadPresignAndComplete(t *testing.T) {
	objects := &fakeStorage{uploadID: "remote-1"}
	repo := &fakeUploadRepo{}
	svc := NewUploadService(zap.NewNop(), repo, objects, "https://static.example.com/")

	created, err := svc.Create(context.Background(), &dto.CreateUploadRequest{
		FileName:    "a/photo.jpg",
		ContentType: "image/jpeg",
		FileSize:    10 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.PartCount != 2 || created.PartSize != targetPartSize {
		t.Fatalf("plan = %+v", created)
	}

	if _, err := svc.PresignParts(context.Background(), created.UploadID, &dto.PresignPartsRequest{
		PartNumbers: []int32{3},
	}); err == nil {
		t.Fatal("expected part number rejection")
	}

	presigned, err := svc.PresignParts(context.Background(), created.UploadID, &dto.PresignPartsRequest{
		PartNumbers: []int32{2, 1},
	})
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if len(presigned.Parts) != 2 || presigned.Parts[0].PartNumber != 2 {
		t.Fatalf("presign order = %+v", presigned.Parts)
	}

	objects.parts = []storage.ObjectPart{
		{PartNumber: 1, ETag: `"aaa"`, Size: targetPartSize},
		{PartNumber: 2, ETag: `"bbb"`, Size: 2 * 1024 * 1024},
	}
	if _, err := svc.Complete(context.Background(), created.UploadID, &dto.CompleteUploadRequest{
		Parts: []dto.CompleteUploadPart{{PartNumber: 1, ETag: "aaa"}, {PartNumber: 2, ETag: `"ccc"`}},
	}); err == nil {
		t.Fatal("expected etag mismatch")
	}

	completed, err := svc.Complete(context.Background(), created.UploadID, &dto.CompleteUploadRequest{
		Parts: []dto.CompleteUploadPart{{PartNumber: 1, ETag: "aaa"}, {PartNumber: 2, ETag: `"bbb"`}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !objects.completed {
		t.Fatal("remote upload was not completed")
	}
	if completed.URL != "https://static.example.com/"+completed.ObjectKey {
		t.Fatalf("url = %s", completed.URL)
	}

	again, err := svc.Complete(context.Background(), created.UploadID, &dto.CompleteUploadRequest{
		Parts: []dto.CompleteUploadPart{{PartNumber: 1, ETag: "aaa"}},
	})
	if err != nil || again.URL != completed.URL {
		t.Fatalf("idempotent complete = %+v err %v", again, err)
	}
}

func TestUploadAbortCompletedSession(t *testing.T) {
	objects := &fakeStorage{uploadID: "remote-1"}
	repo := &fakeUploadRepo{}
	svc := NewUploadService(zap.NewNop(), repo, objects, "https://static.example.com")
	created, err := svc.Create(context.Background(), &dto.CreateUploadRequest{
		FileName: "note.txt", ContentType: "text/plain", FileSize: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.sessions[created.UploadID].Status = model.UploadStatusCompleted

	err = svc.Abort(context.Background(), created.UploadID)
	var appErr *base.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Code != 409 || objects.aborted {
		t.Fatalf("abort completed = %v aborted %v", err, objects.aborted)
	}
}

type fakeStorage struct {
	uploadID  string
	parts     []storage.ObjectPart
	completed bool
	aborted   bool
	exists    bool
}

func (f *fakeStorage) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return f.uploadID, nil
}

func (f *fakeStorage) PresignUploadPart(context.Context, string, string, int32, time.Duration) (string, error) {
	return "https://r2.example/part", nil
}

func (f *fakeStorage) ListParts(context.Context, string, string) ([]storage.ObjectPart, error) {
	if f.completed {
		return nil, apiError{code: "NoSuchUpload"}
	}
	return f.parts, nil
}

func (f *fakeStorage) CompleteMultipartUpload(context.Context, string, string, []storage.ObjectPart) error {
	f.completed = true
	f.exists = true
	return nil
}

func (f *fakeStorage) AbortMultipartUpload(context.Context, string, string) error {
	f.aborted = true
	return nil
}

func (f *fakeStorage) ObjectExists(context.Context, string) (bool, error) {
	return f.exists, nil
}

type apiError struct{ code string }

func (e apiError) Error() string                 { return e.code }
func (e apiError) ErrorCode() string             { return e.code }
func (e apiError) ErrorMessage() string          { return e.code }
func (e apiError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

var errCreate = errString("insert failed")

type errString string

func (e errString) Error() string { return string(e) }

type fakeUploadRepo struct {
	mu        sync.Mutex
	sessions  map[uuid.UUID]*mapper.UploadSessionMapper
	createErr error
}

func (r *fakeUploadRepo) GetTableName() string { return "storage.upload_sessions" }

func (r *fakeUploadRepo) Create(_ context.Context, session *model.UploadSessionModel) (*mapper.UploadSessionMapper, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions == nil {
		r.sessions = map[uuid.UUID]*mapper.UploadSessionMapper{}
	}
	row := &mapper.UploadSessionMapper{
		ID:          session.ID,
		ObjectKey:   session.ObjectKey,
		S3UploadID:  session.S3UploadID,
		FileName:    session.FileName,
		ContentType: session.ContentType,
		FileSize:    session.FileSize,
		PartSize:    session.PartSize,
		PartCount:   session.PartCount,
		Status:      model.UploadStatusPending,
		ExpiresAt:   session.ExpiresAt,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	r.sessions[row.ID] = row
	return row, nil
}

func (r *fakeUploadRepo) GetByID(_ context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.sessions[id]
	if !ok {
		return nil, base.ErrNotFound
	}
	copy := *row
	return &copy, nil
}

func (r *fakeUploadRepo) MarkCompleted(_ context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	return r.mark(id, model.UploadStatusCompleted)
}

func (r *fakeUploadRepo) MarkAborted(_ context.Context, id uuid.UUID) (*mapper.UploadSessionMapper, error) {
	return r.mark(id, model.UploadStatusAborted)
}

func (r *fakeUploadRepo) mark(id uuid.UUID, status model.UploadStatus) (*mapper.UploadSessionMapper, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.sessions[id]
	if !ok {
		return nil, base.ErrNotFound
	}
	if row.Status != model.UploadStatusPending {
		copy := *row
		return &copy, base.ErrConflict
	}
	row.Status = status
	if status == model.UploadStatusCompleted {
		now := time.Now().UTC()
		row.CompletedAt = &now
	}
	copy := *row
	return &copy, nil
}
