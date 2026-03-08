// cmd/agora/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yamanakbas/agora/internal/api"
	"github.com/yamanakbas/agora/internal/config"
	"github.com/yamanakbas/agora/internal/crawler"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/indexer"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "migrate":
		runMigrate(cfg)
	case "crawl":
		runCrawl(cfg)
	case "index":
		runIndex(cfg)
	case "serve":
		runServe(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: agora <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  migrate   Run database migrations")
	fmt.Fprintln(os.Stderr, "  crawl     Crawl the Bazaar API and populate the database")
	fmt.Fprintln(os.Stderr, "  index     Index on-chain x402 transactions from Base")
	fmt.Fprintln(os.Stderr, "  serve     Start the REST API server")
}

func runServe(cfg *config.Config) {
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := database.NewRepository(pool)
	srv := api.NewServer(repo, cfg.EmbedURL, cfg.APIPort)

	log.Printf("Embed sidecar URL: %s", cfg.EmbedURL)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runMigrate(cfg *config.Config) {
	log.Println("Running database migrations...")
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations complete.")
}

func runCrawl(cfg *config.Config) {
	if cfg.BazaarAPIURL == "" {
		log.Fatal("BAZAAR_API_URL is required for crawling. Set it in .env or environment.")
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := database.NewRepository(pool)
	client := crawler.NewClient(cfg.BazaarAPIURL, cfg.BazaarPageSize, cfg.CrawlerMaxRetries)
	runner := crawler.NewRunner(client, repo)

	log.Printf("Starting crawl (concurrency=%d, pageSize=%d)",
		cfg.CrawlerConcurrency, cfg.BazaarPageSize)

	if err := runner.Run(ctx, cfg.CrawlerConcurrency); err != nil {
		log.Fatalf("Crawl failed: %v", err)
	}

	log.Println("Done.")
}

func runIndex(cfg *config.Config) {
	if cfg.BaseRPCURL == "" {
		log.Fatal("BASE_RPC_URL is required for indexing. Set it in .env or environment.")
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	eth, err := indexer.NewEthClient(cfg.BaseRPCURL)
	if err != nil {
		log.Fatalf("Failed to connect to Base RPC: %v", err)
	}
	defer eth.Close()

	repo := database.NewRepository(pool)
	runner := indexer.NewRunner(eth, repo, cfg.IndexerBlockRange, cfg.IndexerStartBlock)

	log.Printf("Starting indexer (blockRange=%d, startBlock=%d)", cfg.IndexerBlockRange, cfg.IndexerStartBlock)

	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Indexing failed: %v", err)
	}
}
