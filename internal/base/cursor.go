package base

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
	"uuid"
)

type cursorPayload struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

// EncodeCursor builds an opaque keyset cursor from the last row of a page.
// The timestamp is stored in UTC so the same row always produces the same cursor.
func EncodeCursor(createdAt time.Time, id string) string {
	raw, err := json.Marshal(cursorPayload{
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		ID:        id,
	})
	if err != nil {
		panic(fmt.Sprintf("encode cursor: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor reads a cursor produced by EncodeCursor.
// A malformed cursor returns an error wrapping ErrInvalidInput.
func DecodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%w: malformed cursor", ErrInvalidInput)
	}

	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return time.Time{}, "", fmt.Errorf("%w: malformed cursor", ErrInvalidInput)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil || createdAt.IsZero() {
		return time.Time{}, "", fmt.Errorf("%w: malformed cursor", ErrInvalidInput)
	}

	if _, err := uuid.Parse(payload.ID); err != nil {
		return time.Time{}, "", fmt.Errorf("%w: malformed cursor", ErrInvalidInput)
	}

	return createdAt, payload.ID, nil
}
