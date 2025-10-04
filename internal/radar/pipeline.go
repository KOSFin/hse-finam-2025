package radar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ClusterEngine abstracts the strategy used to group news items into clusters.
type ClusterEngine interface {
	BuildClusters(ctx context.Context, items []NewsItem) ([]Cluster, error)
}

// Pipeline orchestrates fetching, clustering, scoring, and summarisation.
type Pipeline struct {
	Sources   *SourceRegistry
	Clusterer ClusterEngine
	Scorer    Scorer
}

// NewPipeline constructs a new Pipeline.
func NewPipeline(sources *SourceRegistry, clusterer ClusterEngine, scorer Scorer) (*Pipeline, error) {
	if sources == nil {
		return nil, errors.New("pipeline requires sources")
	}
	return &Pipeline{Sources: sources, Clusterer: clusterer, Scorer: scorer}, nil
}

// Run executes the end-to-end flow returning the hottest events.
func (p *Pipeline) Run(ctx context.Context, params QueryParams) ([]Event, error) {
	if params.Limit <= 0 {
		params.Limit = 5
	}
	items, err := p.Sources.FetchAll(ctx, params.From, params.To)
	if err != nil {
		return nil, err
	}
	if params.Language != "" {
		items = filterLanguage(items, params.Language)
	}

	clusters, err := p.Clusterer.BuildClusters(ctx, items)
	if err != nil {
		return nil, err
	}
	fmt.Println("Pipeline: formed", len(clusters), "clusters from", len(items), "items")
	events := p.Scorer.ScoreClusters(clusters)

	if len(events) > params.Limit {
		events = events[:params.Limit]
	}

	return events, nil
}

func filterLanguage(items []NewsItem, lang string) []NewsItem {
	lang = strings.ToLower(lang)
	if lang == "" {
		return items
	}
	var filtered []NewsItem
	for _, item := range items {
		if strings.ToLower(item.Language) == lang {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// DefaultScorer returns a Scorer preloaded with heuristic weights.
func DefaultScorer() Scorer {
	return Scorer{
		SourceWeights: map[string]float64{
			"bloomberg":       0.9,
			"reuters":         0.88,
			"financial times": 0.85,
			"central bank":    0.92,
			"company call":    0.75,
			"marketwatch":     0.7,
			"finchat":         0.45,
		},
		TagWeights: map[string]float64{
			"guidance_cut":       0.95,
			"supply_chain":       0.85,
			"macro_policy":       0.8,
			"flows":              0.6,
			"management_comment": 0.55,
			"positioning":        0.58,
		},
	}
}

// DefaultClusterer returns baseline clustering configuration.
func DefaultClusterer() ClusterEngine {
	return NewHeuristicClusterer(6*time.Hour, 0.45)
}
