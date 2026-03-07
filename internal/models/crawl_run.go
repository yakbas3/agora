package models

import (
	"time"

	"github.com/google/uuid"
)

type CrawlRun struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	StartedAt        time.Time  `db:"started_at" json:"started_at"`
	CompletedAt      *time.Time `db:"completed_at" json:"completed_at"`
	TotalFetched     int        `db:"total_fetched" json:"total_fetched"`
	NewEndpoints     int        `db:"new_endpoints" json:"new_endpoints"`
	UpdatedEndpoints int        `db:"updated_endpoints" json:"updated_endpoints"`
	Status           string     `db:"status" json:"status"`
	Error            *string    `db:"error" json:"error,omitempty"`
}
