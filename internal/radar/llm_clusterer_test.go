package radar

import (
	"context"
	"errors"
	"testing"
	"time"

	"finamhackbackend/internal/llm"
)

type fakeChatClient struct {
	response string
	err      error
}

func (f *fakeChatClient) ChatCompletion(ctx context.Context, req llm.ChatCompletionRequest) (*llm.ChatCompletionResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	choice := llm.Choice{}
	choice.Message.Content = f.response
	return &llm.ChatCompletionResponse{Choices: []llm.Choice{choice}}, nil
}

func TestLLMClustererUsesResponse(t *testing.T) {
	items := []NewsItem{
		{
			ID:          "n1",
			Headline:    "Company A cuts guidance",
			Summary:     "",
			Source:      "Reuters",
			URL:         "https://example.com/a",
			PublishedAt: time.Date(2025, 10, 3, 8, 0, 0, 0, time.UTC),
			Tickers:     []string{"CMA"},
			Entities:    []string{"Company A"},
		},
		{
			ID:          "n2",
			Headline:    "Factory fire hits Company A supplier",
			Summary:     "",
			Source:      "Bloomberg",
			URL:         "https://example.com/b",
			PublishedAt: time.Date(2025, 10, 3, 9, 30, 0, 0, time.UTC),
			Tickers:     []string{"CMA"},
			Entities:    []string{"Company A"},
		},
	}

	fake := &fakeChatClient{response: `{
		"clusters": [
			{
				"id": "event_supply",
				"news_ids": ["n1", "n2"],
				"primary_news_id": "n1",
				"summary_en": "Company A faces supply disruption",
				"summary_ru": "Компания A сталкивается с перебоями поставок",
				"why_now_en": "Guidance cut confirmed by operational hit",
				"why_now_ru": "Снижение прогноза подтверждается операционными проблемами",
				"entities": ["Company A"],
				"tickers": ["CMA"]
			}
		]
	}`}

	heuristic := NewHeuristicClusterer(6*time.Hour, 0.45)
	clusterer := LLMClusterer{
		Client:      fake,
		Model:       "gemini-2.5-flash",
		Temperature: 0.2,
		MaxTokens:   512,
		MaxItems:    10,
		Fallback:    heuristic,
	}

	clusters, err := clusterer.BuildClusters(context.Background(), items)
	if err != nil {
		t.Fatalf("BuildClusters: %v", err)
	}

	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}

	if clusters[0].ID != "event_supply" {
		t.Errorf("unexpected cluster id: %s", clusters[0].ID)
	}

	if clusters[0].Annotations == nil {
		t.Fatalf("expected annotations from LLM")
	}

	expectedWhy := "Guidance cut confirmed by operational hit / Снижение прогноза подтверждается операционными проблемами"
	if bilingual(clusters[0].Annotations.WhyNowEN, clusters[0].Annotations.WhyNowRU) != expectedWhy {
		t.Errorf("unexpected why now annotation")
	}
}

func TestLLMClustererFallsBack(t *testing.T) {
	items := []NewsItem{{ID: "n1", Headline: "One"}}
	heuristic := NewHeuristicClusterer(6*time.Hour, 0.45)
	clusterer := LLMClusterer{
		Client:   &fakeChatClient{err: errors.New("boom")},
		Model:    "gemini-2.5-flash",
		Fallback: heuristic,
	}

	clusters, err := clusterer.BuildClusters(context.Background(), items)
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}

	if len(clusters) == 0 {
		t.Fatalf("expected fallback clusters")
	}
}
