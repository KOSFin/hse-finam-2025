package radar

import (
	"fmt"
	"sort"
)

func buildTimeline(cluster Cluster) []TimelineEntry {
	if len(cluster.Items) == 0 {
		return nil
	}

	items := make([]NewsItem, len(cluster.Items))
	copy(items, cluster.Items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.Before(items[j].PublishedAt)
	})

	timeline := make([]TimelineEntry, 0, len(items))

	for idx, item := range items {
		label := bilingual("Update", "Обновление")
		if idx == 0 {
			label = bilingual("Initial", "Старт")
		} else if idx == len(items)-1 {
			label = bilingual("Latest", "Финал")
		}

		timeline = append(timeline, TimelineEntry{
			Label:     label,
			Source:    item.Source,
			URL:       item.URL,
			Timestamp: item.PublishedAt,
		})
	}

	if len(timeline) >= 3 {
		for i := 1; i < len(timeline)-1; i++ {
			timeline[i].Label = bilingual(fmt.Sprintf("Update %d", i), fmt.Sprintf("Обновление %d", i))
		}
	}

	return timeline
}
