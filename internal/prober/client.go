package prober

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxBodyBytes  = 64 * 1024 // 64 KB
	userAgent     = "agora-prober/1.0"
)

// Client makes HTTP probe requests to x402 endpoints.
type Client struct {
	http    *http.Client
	timeout time.Duration
}

// NewClient creates a prober Client with the given per-request timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		timeout: timeout,
		http: &http.Client{
			Timeout: timeout,
			// Don't follow redirects — capture the 402 directly.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Probe sends an unauthenticated request to the target and classifies the response.
func (c *Client) Probe(ctx context.Context, target ProbeTarget) ProbeOutcome {
	out := ProbeOutcome{
		EndpointID:   target.EndpointID,
		HealthStatus: "unknown",
	}

	method := target.HTTPMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, target.ResourceURL, http.NoBody)
	if err != nil {
		out.HealthStatus = "dead"
		out.ErrorMessage = fmt.Sprintf("build request: %v", err)
		return out
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := c.http.Do(req)
	latencyMs := int(time.Since(start).Milliseconds())
	out.LatencyMs = &latencyMs

	if err != nil {
		out.HealthStatus = "dead"
		out.ErrorMessage = trimError(err)
		return out
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	out.StatusCode = &code

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	switch {
	case code == http.StatusPaymentRequired:
		out.HealthStatus = "alive"
		c.parse402(body, resp.Header, &out, target.PaymentOptions)
	case code >= 200 && code < 400:
		// Endpoint is up but doesn't require payment — alive but not x402-compliant.
		out.HealthStatus = "alive"
		out.IsValid402 = false
	default:
		// 4xx (not 402) or 5xx — treat as dead.
		out.HealthStatus = "dead"
		out.ErrorMessage = fmt.Sprintf("HTTP %d", code)
	}

	return out
}

// parse402 extracts x402 payment info from a 402 response body or headers.
func (c *Client) parse402(body []byte, headers http.Header, out *ProbeOutcome, refs []PaymentRef) {
	accept := extractAccept(body, headers)
	if accept == nil {
		// Got 402 but couldn't parse payment info.
		return
	}

	out.IsValid402 = true
	out.ResponsePayTo = accept.PayTo
	out.ResponseAmountRaw = accept.MaxAmountRequired
	out.ResponseNetwork = accept.Network
	out.ResponseAsset = accept.Asset

	if len(refs) > 0 {
		matchAndDiff(out, accept, refs)
	}
}

// extractAccept tries multiple locations for x402 payment info.
func extractAccept(body []byte, headers http.Header) *x402Accept {
	// 1. JSON body with accepts array.
	if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
		var b x402Body
		if json.Unmarshal(body, &b) == nil {
			if len(b.Accepts) > 0 && b.Accepts[0].PayTo != "" {
				a := b.Accepts[0]
				return &a
			}
			// Top-level flat fields fallback.
			if b.PayTo != "" {
				return &x402Accept{
					PayTo:             b.PayTo,
					MaxAmountRequired: b.MaxAmountRequired,
					Network:           b.Network,
					Asset:             b.Asset,
				}
			}
		}
	}

	// 2. X-Payment-Required header (some implementations).
	if h := headers.Get("X-Payment-Required"); h != "" {
		var a x402Accept
		if json.Unmarshal([]byte(h), &a) == nil && a.PayTo != "" {
			return &a
		}
	}

	// 3. X-Payment header.
	if h := headers.Get("X-Payment"); h != "" {
		var a x402Accept
		if json.Unmarshal([]byte(h), &a) == nil && a.PayTo != "" {
			return &a
		}
	}

	return nil
}

// matchAndDiff checks if the probed payment info matches any stored payment option.
func matchAndDiff(out *ProbeOutcome, accept *x402Accept, refs []PaymentRef) {
	for _, ref := range refs {
		if strings.EqualFold(ref.PayTo, accept.PayTo) &&
			strings.EqualFold(ref.AmountRaw, accept.MaxAmountRequired) {
			out.PriceMatch = true
			return
		}
	}

	// Mismatch — record the discrepancy.
	out.PriceMatch = false
	type diff struct {
		Field    string `json:"field"`
		DB       string `json:"db"`
		Probed   string `json:"probed"`
	}
	var diffs []diff
	if len(refs) > 0 {
		r := refs[0]
		if !strings.EqualFold(r.AmountRaw, accept.MaxAmountRequired) {
			diffs = append(diffs, diff{
				Field:  "max_amount_required",
				DB:     r.AmountRaw,
				Probed: accept.MaxAmountRequired,
			})
		}
		if !strings.EqualFold(r.PayTo, accept.PayTo) {
			diffs = append(diffs, diff{
				Field:  "pay_to",
				DB:     r.PayTo,
				Probed: accept.PayTo,
			})
		}
	}
	if len(diffs) > 0 {
		b, _ := json.Marshal(diffs)
		out.DiscrepancyDetails = b
	}
}

// trimError extracts a short, readable message from an HTTP error.
func trimError(err error) string {
	// Unwrap URL errors to get the underlying cause.
	var urlErr interface{ Unwrap() error }
	if errors.As(err, &urlErr) {
		if inner := urlErr.Unwrap(); inner != nil {
			return inner.Error()
		}
	}
	msg := err.Error()
	if len(msg) > 200 {
		return msg[:200]
	}
	return msg
}
