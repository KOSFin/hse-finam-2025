package radar

import (
	"fmt"
	"sort"
	"strings"
)

func buildDraft(cluster Cluster, entities, tickers []string, sources []SourceRef, whyNow string) Draft {
	primary := cluster.Primary
	bullets := make([]string, 0, 3)

	if len(entities) > 0 {
		bullets = append(bullets, fmt.Sprintf("%s: %s", bilingual("Impacts", "Влияние"), strings.Join(entities, ", ")))
	}
	if len(tickers) > 0 {
		bullets = append(bullets, fmt.Sprintf("%s: %s", bilingual("Tickers in focus", "Ключевые тикеры"), strings.Join(tickers, ", ")))
	}
	bullets = append(bullets, fmt.Sprintf("%s: %s", bilingual("Why now", "Почему сейчас"), whyNow))

	quote := selectQuote(sources)
	lead := primary.Summary
	if strings.TrimSpace(lead) == "" {
		lead = truncate(primary.Body, 240)
	}
	if cluster.Annotations != nil {
		llmLead := bilingual(cluster.Annotations.SummaryEN, cluster.Annotations.SummaryRU)
		if strings.TrimSpace(llmLead) != "" {
			lead = llmLead
		}
	}

	return Draft{
		Title:   primary.Headline,
		Lead:    lead,
		Bullets: bullets,
		Quote:   quote,
	}
}

func selectQuote(sources []SourceRef) string {
	if len(sources) == 0 {
		return ""
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Published.Before(sources[j].Published)
	})
	source := sources[0]

	return fmt.Sprintf("%s — %s", source.Source, source.Title)
}

func truncate(text string, max int) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= max {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:max])) + "…"
}
