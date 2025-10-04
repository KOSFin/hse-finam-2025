package radar

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHeuristicClustererBuildsExpectedClusters(t *testing.T) {
	source, err := NewStaticFileSource("sample", testDataPath(t))
	if err != nil {
		t.Fatalf("static source: %v", err)
	}

	sources, err := NewSourceRegistry(source)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	items, err := sources.FetchAll(context.Background(), time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC), time.Date(2025, 10, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	clusterer := NewHeuristicClusterer(8*time.Hour, 0.4)
	clusters, err := clusterer.BuildClusters(context.Background(), items)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}

	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	for _, cluster := range clusters {
		if cluster.ID == "" {
			t.Fatalf("cluster ID should not be empty")
		}
		if len(cluster.Items) == 0 {
			t.Fatalf("cluster should contain items")
		}
	}
}

func TestPipelineRunReturnsRankedEvents(t *testing.T) {
	source, err := NewStaticFileSource("sample", testDataPath(t))
	if err != nil {
		t.Fatalf("static source: %v", err)
	}

	sources, err := NewSourceRegistry(source)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	pipeline, err := NewPipeline(sources, DefaultClusterer(), DefaultScorer())
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	params := QueryParams{
		From:  time.Date(2025, 10, 2, 23, 0, 0, 0, time.UTC),
		To:    time.Date(2025, 10, 3, 23, 59, 0, 0, time.UTC),
		Limit: 2,
	}

	events, err := pipeline.Run(context.Background(), params)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Hotness < events[1].Hotness {
		t.Fatalf("events should be sorted by hotness descending")
	}

	for _, event := range events {
		if event.DedupGroup == "" {
			t.Errorf("dedup group missing")
		}
		if len(event.Timeline) == 0 {
			t.Errorf("timeline missing")
		}
		if len(event.Timeline) > 0 && !strings.Contains(event.Timeline[0].Label, "/") {
			t.Errorf("timeline label should be bilingual, got %q", event.Timeline[0].Label)
		}
		if event.Draft.Title == "" || event.Draft.Lead == "" {
			t.Errorf("draft incomplete")
		}
		if !strings.Contains(event.WhyNow, "/") {
			t.Errorf("why now should include bilingual text, got %q", event.WhyNow)
		}
		for _, bullet := range event.Draft.Bullets {
			if !strings.Contains(bullet, "/") {
				t.Errorf("draft bullet should include bilingual text, got %q", bullet)
			}
		}
	}
}

func testDataPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "data", "sample_news.json")
}
