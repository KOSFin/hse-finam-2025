package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"finamhackbackend/internal/llm"
)

// LLMClusterer delegates clustering to a large language model via the VibeRouter API.
type LLMClusterer struct {
	Client      llm.ChatClient
	Model       string
	Temperature float64
	MaxTokens   int
	MaxItems    int
	Fallback    ClusterEngine
}

// BuildClusters clusters news items using the configured LLM, optionally falling back to a heuristic strategy.
func (c LLMClusterer) BuildClusters(ctx context.Context, items []NewsItem) ([]Cluster, error) {
	fmt.Println("LLMClusterer.BuildClusters called")
	if len(items) == 0 {
		return nil, nil
	}
	if c.Client == nil || c.Model == "" {
		return c.buildWithFallback(ctx, items, fmt.Errorf("llm clusterer misconfigured"))
	}

	limited := items
	if c.MaxItems > 0 && len(items) > c.MaxItems {
		limited = make([]NewsItem, c.MaxItems)
		copy(limited, items[:c.MaxItems])
	}

	sorted := make([]NewsItem, len(limited))
	copy(sorted, limited)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PublishedAt.Before(sorted[j].PublishedAt)
	})

	payload, err := c.buildPrompt(sorted)
	if err != nil {
		return c.buildWithFallback(ctx, items, err)
	}

	req := llm.ChatCompletionRequest{
		Model:       c.Model,
		Messages:    payload,
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
		TopP:        0.9,
	}

	log.Printf("LLMClusterer: requesting clustering for %d items via %s", len(sorted), c.Model)

	resp, err := c.Client.ChatCompletion(ctx, req)
	if err != nil {
		return c.buildWithFallback(ctx, items, err)
	}
	if len(resp.Choices) == 0 {
		return c.buildWithFallback(ctx, items, fmt.Errorf("llm response missing choices"))
	}

	clusters, err := c.parseResponse(resp.Choices[0].Message.Content, items)
	if err != nil {
		return c.buildWithFallback(ctx, items, err)
	}

	if len(clusters) == 0 {
		return c.buildWithFallback(ctx, items, fmt.Errorf("llm response returned no clusters"))
	}

	return clusters, nil
}

func (c LLMClusterer) buildWithFallback(ctx context.Context, items []NewsItem, cause error) ([]Cluster, error) {
	log.Printf("LLMClusterer fallback: %v", cause)
	if c.Fallback != nil {
		clusters, fbErr := c.Fallback.BuildClusters(ctx, items)
		if fbErr != nil {
			return nil, fmt.Errorf("llm fallback error: %v (original: %w)", fbErr, cause)
		}
		return clusters, nil
	}
	return nil, cause
}

func (c LLMClusterer) buildPrompt(items []NewsItem) ([]llm.Message, error) {
	type promptItem struct {
		ID          string    `json:"id"`
		Headline    string    `json:"headline"`
		Summary     string    `json:"summary"`
		Body        string    `json:"body"`
		Source      string    `json:"source"`
		URL         string    `json:"url"`
		Language    string    `json:"language"`
		PublishedAt time.Time `json:"published_at"`
		Tickers     []string  `json:"tickers"`
		Entities    []string  `json:"entities"`
	}

	payload := struct {
		News []promptItem `json:"news"`
	}{News: make([]promptItem, 0, len(items))}

	for _, item := range items {
		payload.News = append(payload.News, promptItem{
			ID:          item.ID,
			Headline:    item.Headline,
			Summary:     item.Summary,
			Body:        item.Body,
			Source:      item.Source,
			URL:         item.URL,
			Language:    item.Language,
			PublishedAt: item.PublishedAt.UTC(),
			Tickers:     item.Tickers,
			Entities:    item.Entities,
		})
	}

	newsJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("llm prompt marshal: %w", err)
	}

	systemContent := "You are RADAR, an expert financial analyst who groups related financial news into distinct market events. Respond STRICTLY with valid JSON."

	userContent := fmt.Sprintf(`Group the following financial news into coherent events.
Rules:
- Use a stable identifier for each event (e.g. "event_1").
- Include every news id in exactly one cluster.
- Prefer grouping when the news refers to the same company, instrument, regulator, or macro theme even across languages.
- Provide both English and Russian short summaries for each cluster.
- Provide a short justification (English + Russian) why the event matters now.
- Infer entities and tickers from the statements when missing.

Respond with JSON using this schema:
{
  "clusters": [
    {
      "id": "event_1",
      "news_ids": ["id_a", "id_b"],
      "primary_news_id": "id_a",
      "summary_en": "...",
      "summary_ru": "...",
      "why_now_en": "...",
      "why_now_ru": "...",
      "entities": ["..."],
      "tickers": ["..."]
    }
  ]
}

News payload:
%s`, string(newsJSON))

	return []llm.Message{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userContent},
	}, nil
}

func (c LLMClusterer) parseResponse(content string, items []NewsItem) ([]Cluster, error) {
	jsonPayload := extractJSON(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("llm response missing json payload")
	}

	var decoded struct {
		Clusters []struct {
			ID            string   `json:"id"`
			NewsIDs       []string `json:"news_ids"`
			PrimaryNewsID string   `json:"primary_news_id"`
			SummaryEN     string   `json:"summary_en"`
			SummaryRU     string   `json:"summary_ru"`
			WhyNowEN      string   `json:"why_now_en"`
			WhyNowRU      string   `json:"why_now_ru"`
			Entities      []string `json:"entities"`
			Tickers       []string `json:"tickers"`
		} `json:"clusters"`
	}

	if err := json.Unmarshal([]byte(jsonPayload), &decoded); err != nil {
		return nil, fmt.Errorf("llm response decode: %w", err)
	}

	if len(decoded.Clusters) == 0 {
		return nil, fmt.Errorf("llm response contains no clusters")
	}

	itemByID := make(map[string]NewsItem, len(items))
	for _, item := range items {
		itemByID[item.ID] = item
	}

	clusters := make([]Cluster, 0, len(decoded.Clusters))
	for _, cluster := range decoded.Clusters {
		var clusterItems []NewsItem
		for _, id := range cluster.NewsIDs {
			if item, ok := itemByID[id]; ok {
				clusterItems = append(clusterItems, item)
			}
		}
		if len(clusterItems) == 0 {
			continue
		}

		primary := clusterItems[0]
		if cluster.PrimaryNewsID != "" {
			if candidate, ok := itemByID[cluster.PrimaryNewsID]; ok {
				primary = candidate
			}
		}

		sort.Slice(clusterItems, func(i, j int) bool {
			return clusterItems[i].PublishedAt.Before(clusterItems[j].PublishedAt)
		})

		start := clusterItems[0].PublishedAt
		end := clusterItems[len(clusterItems)-1].PublishedAt

		entities := cluster.Entities
		if len(entities) == 0 {
			entities = collectStrings(clusterItems, func(n NewsItem) []string { return n.Entities })
		}

		tickers := cluster.Tickers
		if len(tickers) == 0 {
			tickers = collectStrings(clusterItems, func(n NewsItem) []string { return n.Tickers })
		}

		annotation := &ClusterAnnotations{
			SummaryEN: cluster.SummaryEN,
			SummaryRU: cluster.SummaryRU,
			WhyNowEN:  cluster.WhyNowEN,
			WhyNowRU:  cluster.WhyNowRU,
			Entities:  entities,
			Tickers:   tickers,
		}

		if annotation.SummaryEN != "" && primary.Summary == "" {
			primary.Summary = annotation.SummaryEN
		}

		clusters = append(clusters, Cluster{
			ID:          preferID(cluster.ID, primary.ID),
			Items:       clusterItems,
			Primary:     primary,
			StartTime:   start,
			EndTime:     end,
			Annotations: annotation,
		})
	}

	return clusters, nil
}

func collectStrings(items []NewsItem, selector func(NewsItem) []string) []string {
	set := make(map[string]struct{})
	for _, item := range items {
		for _, val := range selector(item) {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			set[val] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for val := range set {
		out = append(out, val)
	}
	sort.Strings(out)
	return out
}

func preferID(candidate, fallback string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate != "" {
		return candidate
	}
	return fallback
}

func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return content[start : end+1]
}
