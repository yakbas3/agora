// cmd/agora/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/yamanakbas/agora/internal/api"
	"github.com/yamanakbas/agora/internal/cdp"
	"github.com/yamanakbas/agora/internal/config"
	"github.com/yamanakbas/agora/internal/crawler"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/indexer"
	"github.com/yamanakbas/agora/internal/prober"
	"github.com/yamanakbas/agora/internal/sync"
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
	case "sync":
		runSync(cfg)
	case "probe":
		runProbe(cfg)
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
	fmt.Fprintln(os.Stderr, "  sync [--missing|name]  Sync V1 transactions from CDP SQL API. No arg = all, --missing = only facilitators with 0 txns, name = filter by facilitator name")
	fmt.Fprintln(os.Stderr, "  probe     Probe endpoints for x402 health and compliance")
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

func runProbe(cfg *config.Config) {
	// Optional positional arg: cap the number of endpoints probed (smoke test).
	//   `agora probe`       → probe all endpoints
	//   `agora probe 100`   → probe at most 100 endpoints
	maxEndpoints := 0
	if len(os.Args) >= 3 {
		n, err := strconv.Atoi(os.Args[2])
		if err != nil || n <= 0 {
			log.Fatalf("probe: optional limit must be a positive integer, got %q", os.Args[2])
		}
		maxEndpoints = n
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := database.NewRepository(pool)
	client := prober.NewClient(time.Duration(cfg.ProberTimeoutSecs) * time.Second)
	runner := prober.NewRunner(client, repo, prober.Config{
		Concurrency:  cfg.ProberConcurrency,
		BatchSize:    cfg.ProberBatchSize,
		DomainDelay:  time.Duration(cfg.ProberDomainDelayMs) * time.Millisecond,
		MaxEndpoints: maxEndpoints,
	})

	if maxEndpoints > 0 {
		log.Printf("Starting probe (max=%d, concurrency=%d, timeout=%ds, domainDelay=%dms)",
			maxEndpoints, cfg.ProberConcurrency, cfg.ProberTimeoutSecs, cfg.ProberDomainDelayMs)
	} else {
		log.Printf("Starting probe (concurrency=%d, timeout=%ds, domainDelay=%dms)",
			cfg.ProberConcurrency, cfg.ProberTimeoutSecs, cfg.ProberDomainDelayMs)
	}

	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Probe failed: %v", err)
	}
	log.Println("Done.")
}

func runSync(cfg *config.Config) {
	if cfg.CDPAPIKeyID == "" || cfg.CDPAPIKeySecret == "" {
		log.Fatal("CDP_API_KEY_ID and CDP_API_KEY_SECRET are required for syncing. Set them in .env or environment.")
	}

	// Optional positional arg:
	//   `agora sync`            → sync all facilitators
	//   `agora sync --missing`  → sync only facilitators with zero transactions
	//   `agora sync Coinbase`   → sync facilitators matching the given name
	var arg string
	if len(os.Args) >= 3 {
		arg = os.Args[2]
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	cdpClient, err := cdp.NewClient(cfg.CDPAPIKeyID, cfg.CDPAPIKeySecret)
	if err != nil {
		log.Fatalf("Failed to create CDP client: %v", err)
	}

	repo := database.NewRepository(pool)
	runner := sync.NewRunner(cdpClient, repo)

	switch arg {
	case "":
		log.Println("Starting V1 transaction sync...")
		if err := runner.Run(ctx); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
	case "--missing":
		log.Println("Starting V1 transaction sync (missing facilitators only)...")
		if err := runner.RunMissing(ctx); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
	default:
		log.Printf("Starting V1 transaction sync (facilitator filter: %q)...", arg)
		if err := runner.RunFiltered(ctx, arg); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
	}
	log.Println("Done.")
}
