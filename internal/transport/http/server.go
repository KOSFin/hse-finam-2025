package transporthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finamhackbackend/internal/config"
	"finamhackbackend/internal/radar"
)

type Server struct {
	pipeline      *radar.Pipeline
	defaultWindow time.Duration
	defaultLimit  int
	ingest        *radar.IngestSource
}

func NewServer(pipeline *radar.Pipeline, cfg config.Config, ingest *radar.IngestSource) *Server {
	return &Server{
		pipeline:      pipeline,
		defaultWindow: cfg.DefaultWindow,
		defaultLimit:  cfg.TopK,
		ingest:        ingest,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/radar", s.handleRadar)
	mux.HandleFunc("/news", s.handleIngest)
	mux.HandleFunc("/swagger/openapi.yaml", serveSwaggerYAML)
	mux.HandleFunc("/swagger", serveSwaggerUI)
	mux.HandleFunc("/swagger/", serveSwaggerUI)
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

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.ingest == nil {
		s.writeError(w, http.StatusServiceUnavailable, "ingest disabled")
		return
	}

	var payload struct {
		ID            string   `json:"id"`
		Headline      string   `json:"headline"`
		Summary       string   `json:"summary"`
		Body          string   `json:"body"`
		Source        string   `json:"source"`
		URL           string   `json:"url"`
		Language      string   `json:"language"`
		PublishedAt   string   `json:"published_at"`
		Tickers       []string `json:"tickers"`
		Entities      []string `json:"entities"`
		Country       string   `json:"country"`
		Category      string   `json:"category"`
		Sentiment     *float64 `json:"sentiment"`
		ImportanceTag string   `json:"importance_tag"`
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if payload.Headline == "" || payload.URL == "" {
		s.writeError(w, http.StatusBadRequest, "headline and url are required")
		return
	}

	published := time.Now().UTC()
	if payload.PublishedAt != "" {
		ts, err := time.Parse(time.RFC3339, payload.PublishedAt)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "published_at must be RFC3339")
			return
		}
		published = ts
	}

	news := radar.NewsItem{
		ID:            payload.ID,
		Headline:      payload.Headline,
		Summary:       payload.Summary,
		Body:          payload.Body,
		Source:        defaultString(payload.Source, "ingest"),
		URL:           payload.URL,
		Language:      defaultString(payload.Language, "en"),
		PublishedAt:   published,
		Tickers:       dedupeStrings(payload.Tickers),
		Entities:      dedupeStrings(payload.Entities),
		Country:       payload.Country,
		Category:      payload.Category,
		ImportanceTag: payload.ImportanceTag,
	}
	if payload.Sentiment != nil {
		news.Sentiment = *payload.Sentiment
	}

	stored := s.ingest.Add(news)

	response := map[string]any{
		"status":       "accepted",
		"id":           stored.ID,
		"published_at": stored.PublishedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(response)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func dedupeStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToUpper(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
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
