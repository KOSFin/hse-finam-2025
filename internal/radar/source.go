package radar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// Source defines a pluggable upstream provider capable of fetching news items within a window.
type Source interface {
	Name() string
	Fetch(ctx context.Context, from, to time.Time) ([]NewsItem, error)
}

// SourceRegistry keeps track of available sources and enables simple configuration.
type SourceRegistry struct {
	sources []Source
}

// NewSourceRegistry builds a registry with the provided sources.
func NewSourceRegistry(sources ...Source) (*SourceRegistry, error) {
	if len(sources) == 0 {
		return nil, errors.New("radar: at least one source is required")
	}
	return &SourceRegistry{sources: sources}, nil
}

// Add registers a new source instance.
func (r *SourceRegistry) Add(source Source) {
	r.sources = append(r.sources, source)
}

// FetchAll aggregates items from each registered source.
func (r *SourceRegistry) FetchAll(ctx context.Context, from, to time.Time) ([]NewsItem, error) {
	var results []NewsItem
	for _, src := range r.sources {
		items, err := src.Fetch(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("fetch from %s: %w", src.Name(), err)
		}
		results = append(results, items...)
	}
	return results, nil
}

// StaticFileSource serves NewsItem documents from a JSON file.
type StaticFileSource struct {
	name string
	path string
}

// NewStaticFileSource returns a new StaticFileSource referencing the given file.
func NewStaticFileSource(name, path string) (*StaticFileSource, error) {
	if name == "" {
		return nil, errors.New("static source requires a name")
	}
	if path == "" {
		return nil, errors.New("static source requires a path")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("static source: %w", err)
	}
	return &StaticFileSource{name: name, path: path}, nil
}

// Name returns the source name.
func (s *StaticFileSource) Name() string { return s.name }

// Fetch reads the JSON file and filters items by timeframe.
func (s *StaticFileSource) Fetch(ctx context.Context, from, to time.Time) ([]NewsItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read static file %s: %w", s.path, err)
	}

	items, err := decodeNewsItems(raw)
	if err != nil {
		return nil, fmt.Errorf("decode static file %s: %w", s.path, err)
	}

	var filtered []NewsItem
	for _, item := range items {
		if !item.PublishedAt.Before(from) && !item.PublishedAt.After(to) {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}
