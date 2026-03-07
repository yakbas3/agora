package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/yamanakbas/agora/internal/database"
)

type Handlers struct {
	repo        *database.Repository
	embedClient *EmbedClient
}

func NewHandlers(repo *database.Repository, embedClient *EmbedClient) *Handlers {
	return &Handlers{repo: repo, embedClient: embedClient}
}

type SearchRequest struct {
	Query   string        `json:"query"`
	Filters SearchFilters `json:"filters"`
	Limit   int           `json:"limit"`
}

type SearchFilters struct {
	Network  string   `json:"network"`
	Method   string   `json:"method"`
	MinPrice *float64 `json:"min_price"`
	MaxPrice *float64 `json:"max_price"`
}

type SearchResponse struct {
	Results     []SearchResultJSON `json:"results"`
	Total       int                `json:"total"`
	QueryTimeMs int64              `json:"query_time_ms"`
}

type SearchResultJSON struct {
	Endpoint   EndpointJSON `json:"endpoint"`
	Similarity float64      `json:"similarity"`
}

type EndpointJSON struct {
	ID           string          `json:"id"`
	ResourceURL  string          `json:"resource_url"`
	Domain       string          `json:"domain"`
	Type         string          `json:"type"`
	X402Version  int             `json:"x402_version"`
	Description  string          `json:"description"`
	HTTPMethod   string          `json:"http_method"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}

func (h *Handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}

	start := time.Now()

	vec, err := h.embedClient.Embed(req.Query)
	if err != nil {
		log.Printf("embedding error: %v", err)
		http.Error(w, "embedding service unavailable", http.StatusServiceUnavailable)
		return
	}

	dbFilters := database.SearchFilters{
		Network:  req.Filters.Network,
		Method:   req.Filters.Method,
		MinPrice: req.Filters.MinPrice,
		MaxPrice: req.Filters.MaxPrice,
	}

	results, err := h.repo.SearchByVector(r.Context(), pgvector.NewVector(vec), dbFilters, req.Limit)
	if err != nil {
		log.Printf("search error: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	resp := SearchResponse{
		Total:       len(results),
		QueryTimeMs: time.Since(start).Milliseconds(),
	}
	for _, sr := range results {
		resp.Results = append(resp.Results, SearchResultJSON{
			Endpoint: EndpointJSON{
				ID:           sr.Endpoint.ID.String(),
				ResourceURL:  sr.Endpoint.ResourceURL,
				Domain:       sr.Endpoint.Domain,
				Type:         sr.Endpoint.Type,
				X402Version:  sr.Endpoint.X402Version,
				Description:  sr.Endpoint.Description,
				HTTPMethod:   sr.Endpoint.HTTPMethod,
				InputSchema:  sr.Endpoint.InputSchema,
				OutputSchema: sr.Endpoint.OutputSchema,
			},
			Similarity: sr.Similarity,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	endpoints, err := h.repo.GetEndpointsWithPayments(r.Context(), limit, offset)
	if err != nil {
		log.Printf("get endpoints error: %v", err)
		http.Error(w, "failed to get endpoints", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetStats(r.Context())
	if err != nil {
		log.Printf("get stats error: %v", err)
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handlers) handleEndpointByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
		return
	}

	endpoint, paymentOptions, err := h.repo.GetEndpointByID(r.Context(), id)
	if err != nil {
		log.Printf("get endpoint error: %v", err)
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	resp := map[string]any{
		"endpoint":        endpoint,
		"payment_options": paymentOptions,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
