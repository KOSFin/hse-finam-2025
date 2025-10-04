package radar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type rawNewsItem struct {
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
	Sentiment     float64  `json:"sentiment"`
	ImportanceTag string   `json:"importance_tag"`
}

func decodeNewsItems(data []byte) ([]NewsItem, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var raws []rawNewsItem
	if err := decoder.Decode(&raws); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	items := make([]NewsItem, 0, len(raws))
	for _, r := range raws {
		if r.Headline == "" || r.URL == "" {
			continue
		}
		published, err := time.Parse(time.RFC3339, r.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse time for %s: %w", r.ID, err)
		}
		items = append(items, NewsItem{
			ID:            r.ID,
			Headline:      r.Headline,
			Summary:       r.Summary,
			Body:          r.Body,
			Source:        r.Source,
			URL:           r.URL,
			Language:      r.Language,
			PublishedAt:   published,
			Tickers:       dedupeStrings(r.Tickers),
			Entities:      dedupeStrings(r.Entities),
			Country:       r.Country,
			Category:      r.Category,
			Sentiment:     r.Sentiment,
			ImportanceTag: r.ImportanceTag,
		})
	}

	return items, nil
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
