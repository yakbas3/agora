package models

import (
	"time"

	"github.com/google/uuid"
)

type Facilitator struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	Name         string     `db:"name"          json:"name"`
	Chain        string     `db:"chain"         json:"chain"`
	Address      string     `db:"address"       json:"address"`
	LastSyncedAt *time.Time `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
}
