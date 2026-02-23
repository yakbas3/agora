package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Client is an HTTP client for the Bazaar discovery API.
type Client struct {
	baseURL    string
	pageSize   int
	maxRetries int
	httpClient *http.Client
}

// NewClient creates a new Bazaar API client.
func NewClient(baseURL string, pageSize, maxRetries int) *Client {
	return &Client{
		baseURL:    baseURL,
		pageSize:   pageSize,
		maxRetries: maxRetries,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchPage fetches a single page from the Bazaar API with retry and backoff.
func (c *Client) FetchPage(ctx context.Context, offset int) (*BazaarResponse, error) {
	url := fmt.Sprintf("%s?limit=%d&offset=%d", c.baseURL, c.pageSize, offset)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected HTTP %d from %s", resp.StatusCode, url)
		}

		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		var bazaarResp BazaarResponse
		if err := json.Unmarshal(body, &bazaarResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		return &bazaarResp, nil
	}

	return nil, fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

// FetchAllPages fetches all pages from the Bazaar API using concurrent goroutines.
// concurrency controls how many pages are fetched in parallel.
func (c *Client) FetchAllPages(ctx context.Context, concurrency int) ([]BazaarItem, error) {
	// First, fetch page 0 to learn the total count
	firstResp, err := c.FetchPage(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("fetch first page: %w", err)
	}

	total := firstResp.Pagination.Total
	log.Printf("Bazaar API reports %d total endpoints", total)

	allItems := make([]BazaarItem, 0, total)
	allItems = append(allItems, firstResp.Items...)

	if total <= c.pageSize {
		return allItems, nil
	}

	// Build list of remaining offsets
	var offsets []int
	for offset := c.pageSize; offset < total; offset += c.pageSize {
		offsets = append(offsets, offset)
	}

	// Fetch remaining pages concurrently
	type pageResult struct {
		offset int
		items  []BazaarItem
	}
	results := make([]pageResult, len(offsets))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, offset := range offsets {
		i, offset := i, offset
		g.Go(func() error {
			resp, err := c.FetchPage(ctx, offset)
			if err != nil {
				return fmt.Errorf("fetch page at offset %d: %w", offset, err)
			}
			mu.Lock()
			results[i] = pageResult{offset: offset, items: resp.Items}
			mu.Unlock()
			log.Printf("Fetched page at offset %d (%d items)", offset, len(resp.Items))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, r := range results {
		allItems = append(allItems, r.items...)
	}

	return allItems, nil
}
