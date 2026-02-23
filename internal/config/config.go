// internal/config/config.go
package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DatabaseURL        string `envconfig:"DATABASE_URL" required:"true"`
	BazaarAPIURL       string `envconfig:"BAZAAR_API_URL" required:"true"`
	BazaarPageSize     int    `envconfig:"BAZAAR_PAGE_SIZE" default:"100"`
	CrawlerConcurrency int    `envconfig:"CRAWLER_CONCURRENCY" default:"3"`
	CrawlerMaxRetries  int    `envconfig:"CRAWLER_MAX_RETRIES" default:"5"`
	OpenAIAPIKey       string `envconfig:"OPENAI_API_KEY"`
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
