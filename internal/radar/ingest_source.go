package radar

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IngestSource stores ad-hoc news items submitted via the API.
type IngestSource struct {
	name  string
	mu    sync.RWMutex
	items []NewsItem
}

// NewIngestSource constructs an empty ingest source.
func NewIngestSource(name string) *IngestSource {
	if name == "" {
		name = "ingest"
	}
	return &IngestSource{name: name}
}

// Name returns the source identifier.
func (s *IngestSource) Name() string { return s.name }

// Add registers a news item in the ingest source, generating defaults when missing.
func (s *IngestSource) Add(item NewsItem) NewsItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.PublishedAt.IsZero() {
		item.PublishedAt = time.Now().UTC()
	}

	// Replace existing record with same ID if found.
	for idx := range s.items {
		if s.items[idx].ID == item.ID {
			s.items[idx] = item
			return s.items[idx]
		}
	}

	s.items = append(s.items, item)
	return item
}

// Fetch returns items within the requested timeframe.
func (s *IngestSource) Fetch(ctx context.Context, from, to time.Time) ([]NewsItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]NewsItem, 0, len(s.items))
	for _, item := range s.items {
		if item.PublishedAt.Before(from) || item.PublishedAt.After(to) {
			continue
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].PublishedAt.Before(out[j].PublishedAt)
	})

	return out, nil
}

// PruneOlderThan drops items published before the provided timestamp and returns the number of removed entries.
func (s *IngestSource) PruneOlderThan(ts time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		return 0
	}

	filtered := s.items[:0]
	removed := 0
	for _, item := range s.items {
		if item.PublishedAt.Before(ts) {
			removed++
			continue
		}
		filtered = append(filtered, item)
	}
	s.items = filtered
	return removed
}
