package transporthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"finamhackbackend/internal/config"
	"finamhackbackend/internal/radar"
)

type Server struct {
	pipeline      *radar.Pipeline
	defaultWindow time.Duration
	defaultLimit  int
}

func NewServer(pipeline *radar.Pipeline, cfg config.Config) *Server {
	return &Server{
		pipeline:      pipeline,
		defaultWindow: cfg.DefaultWindow,
		defaultLimit:  cfg.TopK,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/radar", s.handleRadar)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleRadar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params := s.parseParams(r)
	paramsCtx := radar.QueryParams{
		From:     params.from,
		To:       params.to,
		Limit:    params.limit,
		Language: params.language,
	}

	events, err := s.pipeline.Run(ctx, paramsCtx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]any{
		"as_of":  time.Now().UTC(),
		"from":   paramsCtx.From,
		"to":     paramsCtx.To,
		"events": events,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// nothing we can do; connection likely closed
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

type timeframe struct {
	from     time.Time
	to       time.Time
	limit    int
	language string
}

func (s *Server) parseParams(r *http.Request) timeframe {
	values := r.URL.Query()

	limit := s.defaultLimit
	if v := values.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	now := time.Now().UTC()
	to := now
	if v := values.Get("to"); v != "" {
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			to = parsed
		}
	}

	from := to.Add(-s.defaultWindow)

	if v := values.Get("window_hours"); v != "" {
		if hrs, err := strconv.Atoi(v); err == nil && hrs > 0 {
			from = to.Add(-time.Duration(hrs) * time.Hour)
		}
	}

	if v := values.Get("from"); v != "" {
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			from = parsed
		}
	}

	if from.After(to) {
		from = to.Add(-s.defaultWindow)
	}

	language := values.Get("lang")

	return timeframe{from: from, to: to, limit: limit, language: language}
}
