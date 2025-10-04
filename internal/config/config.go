package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config captures runtime configuration for the RADAR service.
type Config struct {
	ListenAddr       string
	StaticDataPath   string
	DefaultWindow    time.Duration
	TopK             int
	VibeRouterAPIKey string
	VibeRouterModel  string
	LLMTemperature   float64
	LLMMaxTokens     int
	LLMMaxItems      int
}

// FromEnv creates a configuration instance sourced from environment variables.
func FromEnv() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		ListenAddr:       getEnv("RADAR_LISTEN_ADDR", ":8080"),
		StaticDataPath:   getEnv("RADAR_STATIC_DATA", "data/sample_news.json"),
		TopK:             5,
		DefaultWindow:    24 * time.Hour,
		VibeRouterAPIKey: getEnv("RADAR_VIBEROUTER_API_KEY", ""),
		VibeRouterModel:  getEnv("RADAR_VIBEROUTER_MODEL", "gemini-2.5-flash"),
		LLMTemperature:   0.2,
		LLMMaxTokens:     1024,
		LLMMaxItems:      40,
	}

	if topK := os.Getenv("RADAR_TOP_K"); topK != "" {
		if _, err := fmt.Sscanf(topK, "%d", &cfg.TopK); err != nil {
			return Config{}, fmt.Errorf("parse RADAR_TOP_K: %w", err)
		}
	}

	if window := os.Getenv("RADAR_DEFAULT_WINDOW_H"); window != "" {
		var hours int
		if _, err := fmt.Sscanf(window, "%d", &hours); err != nil {
			return Config{}, fmt.Errorf("parse RADAR_DEFAULT_WINDOW_H: %w", err)
		}
		cfg.DefaultWindow = time.Duration(hours) * time.Hour
	}

	if temp := os.Getenv("RADAR_LLM_TEMPERATURE"); temp != "" {
		if _, err := fmt.Sscanf(temp, "%f", &cfg.LLMTemperature); err != nil {
			return Config{}, fmt.Errorf("parse RADAR_LLM_TEMPERATURE: %w", err)
		}
	}

	if tokens := os.Getenv("RADAR_LLM_MAX_TOKENS"); tokens != "" {
		if _, err := fmt.Sscanf(tokens, "%d", &cfg.LLMMaxTokens); err != nil {
			return Config{}, fmt.Errorf("parse RADAR_LLM_MAX_TOKENS: %w", err)
		}
	}

	if maxItems := os.Getenv("RADAR_LLM_MAX_ITEMS"); maxItems != "" {
		if _, err := fmt.Sscanf(maxItems, "%d", &cfg.LLMMaxItems); err != nil {
			return Config{}, fmt.Errorf("parse RADAR_LLM_MAX_ITEMS: %w", err)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
