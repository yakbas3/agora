package api

import (
	"log"
	"net/http"

	"github.com/yamanakbas/agora/internal/database"
)

type Server struct {
	handlers *Handlers
	port     string
}

func NewServer(repo *database.Repository, embedURL string, port string) *Server {
	return &Server{
		handlers: NewHandlers(repo, NewEmbedClient(embedURL)),
		port:     port,
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handlers.handleSearch)
	mux.HandleFunc("GET /api/endpoints", s.handlers.handleEndpoints)
	mux.HandleFunc("GET /api/endpoints/{id}", s.handlers.handleEndpointByID)
	mux.HandleFunc("GET /api/stats", s.handlers.handleStats)
	mux.HandleFunc("GET /api/facilitators", s.handlers.handleFacilitators)
	mux.HandleFunc("GET /api/transactions", s.handlers.handleTransactions)

	log.Printf("API server starting on :%s", s.port)
	return http.ListenAndServe(":"+s.port, corsMiddleware(mux))
}
