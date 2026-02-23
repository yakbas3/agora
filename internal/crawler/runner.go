package crawler

import (
	"context"
	"fmt"
	"log"

	"github.com/yamanakbas/agora/internal/database"
)

type Runner struct {
	client *Client
	repo   *database.Repository
}

func NewRunner(client *Client, repo *database.Repository) *Runner {
	return &Runner{client: client, repo: repo}
}

// Run executes a full crawl: fetches all pages, normalizes, and upserts into the database.
func (r *Runner) Run(ctx context.Context, concurrency int) error {
	crawlID, err := r.repo.StartCrawlRun(ctx)
	if err != nil {
		return fmt.Errorf("start crawl run: %w", err)
	}
	log.Printf("Started crawl run %s", crawlID)

	items, err := r.client.FetchAllPages(ctx, concurrency)
	if err != nil {
		_ = r.repo.FailCrawlRun(ctx, crawlID, err.Error())
		return fmt.Errorf("fetch pages: %w", err)
	}
	log.Printf("Fetched %d items from Bazaar API", len(items))

	newCount := 0
	updatedCount := 0
	skippedCount := 0

	for i, item := range items {
		endpoint, options, err := NormalizeItem(item)
		if err != nil {
			log.Printf("WARN: skip item %q: %v", item.Resource, err)
			skippedCount++
			continue
		}

		isNew, isUpdated, err := r.repo.UpsertEndpoint(ctx, endpoint, options)
		if err != nil {
			log.Printf("WARN: upsert failed for %q: %v", item.Resource, err)
			skippedCount++
			continue
		}

		if isNew {
			newCount++
		} else if isUpdated {
			updatedCount++
		}

		if (i+1)%500 == 0 {
			log.Printf("Progress: %d/%d items processed (%d new, %d updated, %d skipped)",
				i+1, len(items), newCount, updatedCount, skippedCount)
		}
	}

	log.Printf("Crawl complete: %d total, %d new, %d updated, %d skipped",
		len(items), newCount, updatedCount, skippedCount)

	if err := r.repo.CompleteCrawlRun(ctx, crawlID, len(items), newCount, updatedCount); err != nil {
		return fmt.Errorf("complete crawl run: %w", err)
	}

	return nil
}
