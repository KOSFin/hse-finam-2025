package transporthttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"finamhackbackend/internal/config"
	"finamhackbackend/internal/radar"
)

func TestRadarEndpoint(t *testing.T) {
	source, err := radar.NewStaticFileSource("sample", filepath.Join("..", "..", "..", "data", "sample_news.json"))
	if err != nil {
		t.Fatalf("static source: %v", err)
	}

	sources, err := radar.NewSourceRegistry(source)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	pipeline, err := radar.NewPipeline(sources, radar.DefaultClusterer(), radar.DefaultScorer())
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	srv := NewServer(pipeline, config.Config{DefaultWindow: 24 * time.Hour, TopK: 2})

	req := httptest.NewRequest(http.MethodGet, "/radar?limit=2&from=2025-10-02T23:00:00Z&to=2025-10-04T00:00:00Z", nil).WithContext(context.Background())
	rec := httptest.NewRecorder()

	srv.handleRadar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload struct {
		Events []radar.Event `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload.Events) == 0 {
		t.Fatalf("expected at least one event")
	}
}
