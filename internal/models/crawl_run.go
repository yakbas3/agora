package models

import (
	"time"

	"github.com/google/uuid"
)

type CrawlRun struct {
	ID               uuid.UUID  `db:"id"`
	StartedAt        time.Time  `db:"started_at"`
	CompletedAt      *time.Time `db:"completed_at"`
	TotalFetched     int        `db:"total_fetched"`
	NewEndpoints     int        `db:"new_endpoints"`
	UpdatedEndpoints int        `db:"updated_endpoints"`
	Status           string     `db:"status"`
	Error            *string    `db:"error"`
}
