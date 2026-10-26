package model

import (
	"time"
	"uuid"
)

// StoreModel is the write-side entity for platform.stores.
// Read paths return mapper.StoreMapper instead of this struct.
type StoreModel struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
