package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"finamhackbackend/internal/config"
	"finamhackbackend/internal/llm"
	"finamhackbackend/internal/radar"
	transporthttp "finamhackbackend/internal/transport/http"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	staticSource, err := radar.NewStaticFileSource("sample", cfg.StaticDataPath)
	if err != nil {
		log.Fatalf("init static source: %v", err)
	}

	sources, err := radar.NewSourceRegistry(staticSource)
	if err != nil {
		log.Fatalf("init source registry: %v", err)
	}

	clusterer := radar.DefaultClusterer()
	fmt.Println(cfg.VibeRouterAPIKey)
	if cfg.VibeRouterAPIKey != "" {
		llmClient := llm.NewClient(cfg.VibeRouterAPIKey)
		clusterer = radar.LLMClusterer{
			Client:      llmClient,
			Model:       cfg.VibeRouterModel,
			Temperature: cfg.LLMTemperature,
			MaxTokens:   cfg.LLMMaxTokens,
			MaxItems:    cfg.LLMMaxItems,
			Fallback:    radar.NewHeuristicClusterer(6*time.Hour, 0.45),
		}
		log.Printf("LLM clustering enabled with model %s", cfg.VibeRouterModel)
	}

	pipeline, err := radar.NewPipeline(sources, clusterer, radar.DefaultScorer())
	if err != nil {
		log.Fatalf("init pipeline: %v", err)
	}

	server := transporthttp.NewServer(pipeline, cfg)

	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      withLogging(server.Routes()),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("RADAR API listening on %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("signal received: %s, shutting down", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("%s %s %s", r.Method, r.URL.Path, duration)
	})
}
