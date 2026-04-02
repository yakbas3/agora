package prober

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestClient() *Client {
	return NewClient(5 * time.Second)
}

func TestProbe_402_ValidBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprint(w, `{"accepts":[{"payTo":"0xabc","maxAmountRequired":"1000","network":"base","asset":"0xusdc"}]}`)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
		HTTPMethod:  "GET",
	})

	if out.HealthStatus != "alive" {
		t.Errorf("want alive, got %s", out.HealthStatus)
	}
	if !out.IsValid402 {
		t.Error("want is_valid_402=true")
	}
	if out.ResponsePayTo != "0xabc" {
		t.Errorf("want pay_to 0xabc, got %s", out.ResponsePayTo)
	}
	if out.ResponseAmountRaw != "1000" {
		t.Errorf("want amount_raw 1000, got %s", out.ResponseAmountRaw)
	}
	if out.LatencyMs == nil || *out.LatencyMs < 0 {
		t.Error("want non-negative latency_ms")
	}
}

func TestProbe_402_XPaymentHeader(t *testing.T) {
	accept := x402Accept{
		PayTo:             "0xdef",
		MaxAmountRequired: "500",
		Network:           "base-sepolia",
		Asset:             "0xusdc2",
	}
	headerVal, _ := json.Marshal(accept)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Payment-Required", string(headerVal))
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
	})

	if out.HealthStatus != "alive" {
		t.Errorf("want alive, got %s", out.HealthStatus)
	}
	if !out.IsValid402 {
		t.Error("want is_valid_402=true")
	}
	if out.ResponsePayTo != "0xdef" {
		t.Errorf("want pay_to 0xdef, got %s", out.ResponsePayTo)
	}
}

func TestProbe_402_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
	})

	if out.HealthStatus != "alive" {
		t.Errorf("want alive, got %s", out.HealthStatus)
	}
	if out.IsValid402 {
		t.Error("want is_valid_402=false for empty body")
	}
}

func TestProbe_200_NotX402(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":"ok"}`)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
	})

	if out.HealthStatus != "alive" {
		t.Errorf("want alive, got %s", out.HealthStatus)
	}
	if out.IsValid402 {
		t.Error("want is_valid_402=false for 200 response")
	}
}

func TestProbe_500_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
	})

	if out.HealthStatus != "dead" {
		t.Errorf("want dead, got %s", out.HealthStatus)
	}
}

func TestProbe_404_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
	})

	if out.HealthStatus != "dead" {
		t.Errorf("want dead for 404, got %s", out.HealthStatus)
	}
}

func TestProbe_ConnectionRefused(t *testing.T) {
	// Use a port that's not listening.
	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: "http://127.0.0.1:19999/api",
	})

	if out.HealthStatus != "dead" {
		t.Errorf("want dead for connection refused, got %s", out.HealthStatus)
	}
	if out.ErrorMessage == "" {
		t.Error("want non-empty error message for connection refused")
	}
}

func TestProbe_PriceMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprint(w, `{"accepts":[{"payTo":"0xabc","maxAmountRequired":"1000","network":"base","asset":"0xusdc"}]}`)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
		PaymentOptions: []PaymentRef{
			{PayTo: "0xabc", AmountRaw: "1000"},
		},
	})

	if !out.PriceMatch {
		t.Error("want price_match=true when pay_to and amount match")
	}
	if out.DiscrepancyDetails != nil {
		t.Error("want no discrepancy when prices match")
	}
}

func TestProbe_PriceMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		// Endpoint now charges 9999 but DB has 1000.
		fmt.Fprint(w, `{"accepts":[{"payTo":"0xabc","maxAmountRequired":"9999","network":"base","asset":"0xusdc"}]}`)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
		PaymentOptions: []PaymentRef{
			{PayTo: "0xabc", AmountRaw: "1000"},
		},
	})

	if out.PriceMatch {
		t.Error("want price_match=false when amounts differ")
	}
	if len(out.DiscrepancyDetails) == 0 {
		t.Error("want non-empty discrepancy_details on mismatch")
	}
}

func TestProbe_CaseInsensitivePayTo(t *testing.T) {
	// DB has lower-case pay_to; 402 returns EIP-55 checksummed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprint(w, `{"accepts":[{"payTo":"0xABCDEF","maxAmountRequired":"500","network":"base","asset":"0xusdc"}]}`)
	}))
	defer srv.Close()

	c := newTestClient()
	out := c.Probe(context.Background(), ProbeTarget{
		EndpointID:  uuid.New(),
		ResourceURL: srv.URL,
		PaymentOptions: []PaymentRef{
			{PayTo: "0xabcdef", AmountRaw: "500"},
		},
	})

	if !out.PriceMatch {
		t.Error("want price_match=true for case-insensitive address match")
	}
}
