package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

type Endpoint struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	ResourceURL  string          `db:"resource_url" json:"resource_url"`
	Domain       string          `db:"domain" json:"domain"`
	Type         string          `db:"type" json:"type"`
	X402Version  int             `db:"x402_version" json:"x402_version"`
	Description  string          `db:"description" json:"description"`
	HTTPMethod   string          `db:"http_method" json:"http_method"`
	InputSchema  json.RawMessage `db:"input_schema" json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `db:"output_schema" json:"output_schema,omitempty"`
	RawMetadata  json.RawMessage `db:"raw_metadata" json:"raw_metadata,omitempty"`
	LastUpdated  time.Time       `db:"last_updated" json:"last_updated"`
	FirstSeen    time.Time       `db:"first_seen" json:"first_seen"`
	LastCrawled  time.Time       `db:"last_crawled" json:"last_crawled"`
	Embedding        pgvector.Vector `db:"embedding" json:"-"`
	ReliabilityScore float64         `db:"reliability_score" json:"reliability_score"`
}
