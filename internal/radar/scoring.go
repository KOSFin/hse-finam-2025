package radar

import (
	"math"
	"sort"
	"strings"
)

// Scorer evaluates clusters and returns Event representations sorted by hotness.
type Scorer struct {
	SourceWeights map[string]float64
	TagWeights    map[string]float64
}

// ScoreClusters computes hotness metrics and returns sorted events.
func (s Scorer) ScoreClusters(clusters []Cluster) []Event {
	if len(clusters) == 0 {
		return nil
	}

	events := make([]Event, 0, len(clusters))
	for _, cluster := range clusters {
		event := s.buildEvent(cluster)
		if event.Hotness <= 0 {
			continue
		}
		events = append(events, event)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Hotness > events[j].Hotness
	})

	return events
}

func (s Scorer) buildEvent(cluster Cluster) Event {
	items := cluster.Items
	if len(items) == 0 {
		return Event{}
	}

	sources := make([]SourceRef, 0, len(items))
	var tickers []string
	var entities []string
	var totalSentiment float64
	var negativeCount int
	var earliest = items[0].PublishedAt
	var latest = items[0].PublishedAt

	tickerSet := make(map[string]struct{})
	entitySet := make(map[string]struct{})

	for _, item := range items {
		sources = append(sources, SourceRef{
			Title:     item.Headline,
			Source:    item.Source,
			URL:       item.URL,
			Published: item.PublishedAt,
		})
		for _, ticker := range item.Tickers {
			t := strings.ToUpper(ticker)
			if _, ok := tickerSet[t]; !ok {
				tickerSet[t] = struct{}{}
				tickers = append(tickers, t)
			}
		}
		for _, entity := range item.Entities {
			en := normalizeEntity(entity)
			if _, ok := entitySet[en]; !ok {
				entitySet[en] = struct{}{}
				entities = append(entities, entity)
			}
		}
		totalSentiment += math.Abs(item.Sentiment)
		if item.Sentiment < 0 {
			negativeCount++
		}
		if item.PublishedAt.Before(earliest) {
			earliest = item.PublishedAt
		}
		if item.PublishedAt.After(latest) {
			latest = item.PublishedAt
		}
	}

	sort.Strings(tickers)
	sort.Strings(entities)

	coverage := float64(len(items))
	reach := float64(len(tickers))
	novelty := 1.0
	if coverage > 1 {
		novelty = 1.0 - math.Min(0.6, (coverage-1.0)*0.12)
	}

	velocity := 1.0
	window := latest.Sub(earliest)
	if window > 0 {
		hours := window.Hours()
		velocity = math.Max(0.2, math.Min(1.0, 6.0/(hours+1)))
	}

	sourceScore := s.averageSourceWeight(items)
	sentimentScore := math.Min(1.0, totalSentiment/float64(len(items)))
	if negativeCount > 0 && negativeCount == len(items) {
		sentimentScore = math.Min(1.0, sentimentScore+0.15)
	}

	tagScore := s.tagWeight(items)
	breadthScore := math.Min(1.0, reach/4.0)
	extentScore := math.Min(1.0, float64(len(entities))/6.0)

	hotness := weightedSum(map[string]float64{
		"coverage":    math.Min(1.0, coverage/4.0),
		"velocity":    velocity,
		"credibility": sourceScore,
		"sentiment":   sentimentScore,
		"tag":         tagScore,
		"breadth":     0.6*breadthScore + 0.4*extentScore,
		"novelty":     novelty,
	})

	whyNow := s.composeWhyNow(coverage, reach, velocity, sourceScore)
	if cluster.Annotations != nil {
		llmWhy := bilingual(cluster.Annotations.WhyNowEN, cluster.Annotations.WhyNowRU)
		if strings.TrimSpace(llmWhy) != "" {
			if strings.TrimSpace(whyNow) != "" {
				whyNow = llmWhy + " | " + whyNow
			} else {
				whyNow = llmWhy
			}
		}
	}
	draft := buildDraft(cluster, entities, tickers, sources, whyNow)
	timeline := buildTimeline(cluster)

	return Event{
		DedupGroup: cluster.ID,
		Headline:   cluster.Primary.Headline,
		Hotness:    roundTo(hotness, 3),
		WhyNow:     whyNow,
		Entities:   entities,
		Tickers:    tickers,
		Sources:    sources,
		Timeline:   timeline,
		Draft:      draft,
	}
}

func (s Scorer) averageSourceWeight(items []NewsItem) float64 {
	if len(items) == 0 {
		return 0.3
	}
	var total float64
	for _, item := range items {
		if w, ok := s.SourceWeights[strings.ToLower(item.Source)]; ok {
			total += w
			continue
		}
		total += 0.5
	}
	return math.Min(1.0, total/float64(len(items)))
}

func (s Scorer) tagWeight(items []NewsItem) float64 {
	var best float64
	for _, item := range items {
		if w, ok := s.TagWeights[item.ImportanceTag]; ok && w > best {
			best = w
		}
	}
	if best == 0 {
		return 0.45
	}
	return best
}

func (s Scorer) composeWhyNow(coverage, reach, velocity, sourceScore float64) string {
	var notes []string
	if coverage > 1 {
		notes = append(notes, bilingual("multiple confirmations", "несколько подтверждений"))
	}
	if reach >= 2 {
		notes = append(notes, bilingual("broad asset impact", "широкое влияние на активы"))
	}
	if velocity > 0.8 {
		notes = append(notes, bilingual("fast-moving timeline", "быстро развивающийся таймлайн"))
	}
	if sourceScore > 0.7 {
		notes = append(notes, bilingual("high-credibility sources", "источники с высоким доверием"))
	}
	if len(notes) == 0 {
		notes = append(notes, bilingual("fresh development", "свежее развитие событий"))
	}
	return strings.Join(notes, "; ")
}

func weightedSum(weights map[string]float64) float64 {
	// static weights derived heuristically
	return clamp01(weights["coverage"]*0.18 +
		weights["velocity"]*0.18 +
		weights["credibility"]*0.15 +
		weights["sentiment"]*0.12 +
		weights["tag"]*0.18 +
		weights["breadth"]*0.12 +
		weights["novelty"]*0.07)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func roundTo(v float64, prec int) float64 {
	p := math.Pow10(prec)
	return math.Round(v*p) / p
}

func normalizeEntity(entity string) string {
	return strings.TrimSpace(strings.ToLower(entity))
}
