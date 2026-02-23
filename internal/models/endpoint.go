package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID           uuid.UUID       `db:"id"`
	ResourceURL  string          `db:"resource_url"`
	Domain       string          `db:"domain"`
	Type         string          `db:"type"`
	X402Version  int             `db:"x402_version"`
	Description  string          `db:"description"`
	HTTPMethod   string          `db:"http_method"`
	InputSchema  json.RawMessage `db:"input_schema"`
	OutputSchema json.RawMessage `db:"output_schema"`
	RawMetadata  json.RawMessage `db:"raw_metadata"`
	LastUpdated  time.Time       `db:"last_updated"`
	FirstSeen    time.Time       `db:"first_seen"`
	LastCrawled  time.Time       `db:"last_crawled"`
}
