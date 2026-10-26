package mapper

import (
	"time"
	"uuid"
)

// StoreMapper is the row shape returned by store queries.
type StoreMapper struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
