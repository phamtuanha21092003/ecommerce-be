package base

import (
	"errors"
	"testing"
	"time"
	"uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 10, 26, 9, 8, 7, 123456000, time.FixedZone("ICT", 7*3600))
	id := uuid.MustParse("018f4f3a-6c3a-7c2e-9b1a-6d5e4c3b2a19")

	encoded := EncodeCursor(createdAt, id.String())
	gotTime, gotID, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotTime.Equal(createdAt.UTC()) {
		t.Fatalf("time = %s, want %s", gotTime, createdAt.UTC())
	}
	if gotID != id.String() {
		t.Fatalf("id = %s", gotID)
	}
}

func TestDecodeCursorRejectsMalformed(t *testing.T) {
	cases := []string{
		"",
		"!!!",
		"eyJjcmVhdGVkQXQiOiJub3QtYS10aW1lIiwiaWQiOiJ4In0",
	}
	for _, cursor := range cases {
		_, _, err := DecodeCursor(cursor)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor %q err = %v, want ErrInvalidInput", cursor, err)
		}
	}
}
