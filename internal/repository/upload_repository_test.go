package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"uuid"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/base"
	"github.com/phamtuanha21092003/ecommerce-be/internal/model"
	"github.com/phamtuanha21092003/ecommerce-be/internal/testutil"
)

func TestUploadRepositoryStatusTransition(t *testing.T) {
	db := testutil.Open(t)
	if _, err := db.Exec(`TRUNCATE TABLE storage.upload_sessions`); err != nil {
		t.Fatal(err)
	}

	repo := NewUploadRepository(zap.NewNop(), db)
	id := uuid.NewV7()
	row, err := repo.Create(context.Background(), &model.UploadSessionModel{
		ID:          id,
		ObjectKey:   "uploads/test/" + id.String() + "/file.bin",
		S3UploadID:  "remote-upload",
		FileName:    "file.bin",
		ContentType: "application/octet-stream",
		FileSize:    1024,
		PartSize:    1024,
		PartCount:   1,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != model.UploadStatusPending || row.CompletedAt != nil {
		t.Fatalf("created = %+v", row)
	}

	completed, err := repo.MarkCompleted(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != model.UploadStatusCompleted || completed.CompletedAt == nil {
		t.Fatalf("completed = %+v", completed)
	}

	again, err := repo.MarkCompleted(context.Background(), id)
	if !errors.Is(err, base.ErrConflict) || again.Status != model.UploadStatusCompleted {
		t.Fatalf("second complete = %+v err %v", again, err)
	}

	_, err = repo.GetByID(context.Background(), uuid.NewV7())
	if !errors.Is(err, base.ErrNotFound) {
		t.Fatalf("missing session err = %v", err)
	}
}
