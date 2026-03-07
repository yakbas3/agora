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

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handlers.handleSearch)
	mux.HandleFunc("GET /api/endpoints", s.handlers.handleEndpoints)
	mux.HandleFunc("GET /api/endpoints/{id}", s.handlers.handleEndpointByID)

	log.Printf("API server starting on :%s", s.port)
	return http.ListenAndServe(":"+s.port, mux)
}
