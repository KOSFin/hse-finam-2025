package radar

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Cluster represents a deduplicated group of related news items.
type Cluster struct {
	ID          string
	Items       []NewsItem
	Primary     NewsItem
	StartTime   time.Time
	EndTime     time.Time
	Annotations *ClusterAnnotations
}

// ClusterAnnotations captures optional metadata supplied by LLMs.
type ClusterAnnotations struct {
	SummaryEN string
	SummaryRU string
	WhyNowEN  string
	WhyNowRU  string
	Entities  []string
	Tickers   []string
}

// HeuristicClusterer groups news items into deduplicated clusters based on textual similarity and timing.
type HeuristicClusterer struct {
	TimeWindow          time.Duration
	SimilarityThreshold float64
	MaxClusterSize      int
}

// NewHeuristicClusterer constructs a HeuristicClusterer with sane defaults when fields are unset.
func NewHeuristicClusterer(timeWindow time.Duration, threshold float64) HeuristicClusterer {
	if timeWindow == 0 {
		timeWindow = 6 * time.Hour
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.45
	}
	return HeuristicClusterer{TimeWindow: timeWindow, SimilarityThreshold: threshold, MaxClusterSize: 12}
}

// BuildClusters returns clusters of similar news items.
func (c HeuristicClusterer) BuildClusters(_ context.Context, items []NewsItem) ([]Cluster, error) {
	if len(items) == 0 {
		return nil, nil
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.Before(items[j].PublishedAt)
	})

	var clusters []Cluster

	for _, item := range items {
		assigned := false
		for idx := range clusters {
			cluster := &clusters[idx]
			if len(cluster.Items) >= c.MaxClusterSize {
				continue
			}
			if !withinWindow(cluster.StartTime, cluster.EndTime, item.PublishedAt, c.TimeWindow) {
				continue
			}
			if clusterContainsRelated(*cluster, item, c.SimilarityThreshold) {
				cluster.Items = append(cluster.Items, item)
				if item.PublishedAt.Before(cluster.StartTime) {
					cluster.StartTime = item.PublishedAt
				}
				if item.PublishedAt.After(cluster.EndTime) {
					cluster.EndTime = item.PublishedAt
				}
				// prioritise earliest high-credibility item as primary
				if item.PublishedAt.Before(cluster.Primary.PublishedAt) {
					cluster.Primary = item
				}
				assigned = true
				break
			}
		}

		if !assigned {
			clusters = append(clusters, Cluster{
				ID:        uuid.NewString(),
				Items:     []NewsItem{item},
				Primary:   item,
				StartTime: item.PublishedAt,
				EndTime:   item.PublishedAt,
			})
		}
	}

	return clusters, nil
}

func withinWindow(start, end, ts time.Time, window time.Duration) bool {
	if ts.Before(start.Add(-window)) {
		return false
	}
	if ts.After(end.Add(window)) {
		return false
	}
	return true
}

func similarityScore(a, b string) float64 {
	tokensA := tokenize(a)
	tokensB := tokenize(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}

	setA := make(map[string]struct{}, len(tokensA))
	for _, t := range tokensA {
		setA[t] = struct{}{}
	}
	setB := make(map[string]struct{}, len(tokensB))
	for _, t := range tokensB {
		setB[t] = struct{}{}
	}

	var intersection int
	for token := range setA {
		if _, ok := setB[token]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

func clusterContainsRelated(cluster Cluster, candidate NewsItem, threshold float64) bool {
	for _, existing := range cluster.Items {
		if areRelated(existing, candidate, threshold) {
			return true
		}
	}
	return false
}

func areRelated(a, b NewsItem, threshold float64) bool {
	if sharesToken(a.Tickers, b.Tickers) || sharesToken(a.Entities, b.Entities) {
		return true
	}
	return similarityScore(a.Headline, b.Headline) >= threshold
}

func sharesToken(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[strings.ToUpper(v)] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[strings.ToUpper(v)]; ok {
			return true
		}
	}
	return false
}

func tokenize(s string) []string {
	replacer := strings.NewReplacer(
		",", " ", ".", " ", ":", " ", ";", " ", "!", " ", "?", " ",
		"(", " ", ")", " ", "'", " ", "\"", " ", "-", " ", "_", " ",
	)
	normalized := strings.ToLower(replacer.Replace(s))
	parts := strings.Fields(normalized)
	var tokens []string
	for _, p := range parts {
		if len(p) <= 2 {
			continue
		}
		tokens = append(tokens, p)
	}
	return tokens
}
