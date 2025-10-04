package radar

import "time"

// NewsItem represents a raw news document fetched from an upstream source.
type NewsItem struct {
	ID            string    `json:"id"`
	Headline      string    `json:"headline"`
	Summary       string    `json:"summary"`
	Body          string    `json:"body"`
	Source        string    `json:"source"`
	URL           string    `json:"url"`
	Language      string    `json:"language"`
	PublishedAt   time.Time `json:"published_at"`
	Tickers       []string  `json:"tickers"`
	Entities      []string  `json:"entities"`
	Country       string    `json:"country"`
	Category      string    `json:"category"`
	Sentiment     float64   `json:"sentiment"`
	ImportanceTag string    `json:"importance_tag"`
}

// Event represents an aggregated hot news candidate with scoring metadata.
type Event struct {
	DedupGroup string          `json:"dedup_group"`
	Headline   string          `json:"headline"`
	Hotness    float64         `json:"hotness"`
	WhyNow     string          `json:"why_now"`
	Entities   []string        `json:"entities"`
	Tickers    []string        `json:"tickers"`
	Sources    []SourceRef     `json:"sources"`
	Timeline   []TimelineEntry `json:"timeline"`
	Draft      Draft           `json:"draft"`
}

// SourceRef keeps track of references used to corroborate an event.
type SourceRef struct {
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	Published time.Time `json:"published"`
}

// TimelineEntry captures the key updates within an event cluster.
type TimelineEntry struct {
	Label     string    `json:"label"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	Timestamp time.Time `json:"timestamp"`
}

// Draft is a structured draft for downstream publications.
type Draft struct {
	Title   string   `json:"title"`
	Lead    string   `json:"lead"`
	Bullets []string `json:"bullets"`
	Quote   string   `json:"quote"`
}

// QueryParams encapsulates the timeframe and request configuration provided by the user.
type QueryParams struct {
	From     time.Time
	To       time.Time
	Limit    int
	Language string
}
