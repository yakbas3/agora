// internal/config/config.go
package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DatabaseURL        string `envconfig:"DATABASE_URL" required:"true"`
	BazaarAPIURL       string `envconfig:"BAZAAR_API_URL"`
	BazaarPageSize     int    `envconfig:"BAZAAR_PAGE_SIZE" default:"100"`
	CrawlerConcurrency int    `envconfig:"CRAWLER_CONCURRENCY" default:"3"`
	CrawlerMaxRetries  int    `envconfig:"CRAWLER_MAX_RETRIES" default:"5"`
	OpenAIAPIKey       string `envconfig:"OPENAI_API_KEY"`
	BaseRPCURL         string `envconfig:"BASE_RPC_URL"`
	IndexerBlockRange  int64  `envconfig:"INDEXER_BLOCK_RANGE" default:"10"`
	IndexerStartBlock  int64  `envconfig:"INDEXER_START_BLOCK" default:"25000000"`
	EmbedURL           string `envconfig:"EMBED_URL" default:"http://localhost:8100"`
	APIPort            string `envconfig:"API_PORT" default:"8080"`
	CDPAPIKeyID        string `envconfig:"CDP_API_KEY_ID"`
	CDPAPIKeySecret    string `envconfig:"CDP_API_KEY_SECRET"`
}

func Load() (*Config, error) {
	// Best-effort .env loading — ignore error if file doesn't exist
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
