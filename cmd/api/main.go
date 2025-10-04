package main

import (
	"context"
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

	ingestSource := radar.NewIngestSource("ingest")

	sources, err := radar.NewSourceRegistry(staticSource, ingestSource)
	if err != nil {
		log.Fatalf("init source registry: %v", err)
	}

	clusterer := radar.DefaultClusterer()
	if cfg.VibeRouterAPIKey != "" {
		llmClient := llm.NewClient(cfg.VibeRouterAPIKey)
		clusterer = &radar.LLMClusterer{
			Client:      llmClient,
			Model:       cfg.VibeRouterModel,
			Temperature: cfg.LLMTemperature,
			MaxTokens:   cfg.LLMMaxTokens,
			MaxItems:    cfg.LLMMaxItems,
			Fallback:    radar.NewHeuristicClusterer(6*time.Hour, 0.45),
			CacheTTL:    2 * time.Minute,
		}
		log.Printf("LLM clustering enabled with model %s", cfg.VibeRouterModel)
	}

	pipeline, err := radar.NewPipeline(sources, clusterer, radar.DefaultScorer())
	if err != nil {
		log.Fatalf("init pipeline: %v", err)
	}

	server := transporthttp.NewServer(pipeline, cfg, ingestSource)

	// добавляем CORS и логирование
	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      withLogging(withCORS(server.Routes())),
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

	// Graceful shutdown
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

// Middleware: логирование запросов
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		// Отдельно подсвечиваем preflight (OPTIONS)
		if r.Method == http.MethodOptions {
			log.Printf("[CORS preflight] %s %s %s", r.Method, r.URL.Path, duration)
		} else {
			log.Printf("%s %s %s", r.Method, r.URL.Path, duration)
		}
	})
}

// Middleware: разрешаем CORS
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем фронт получать ответы
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Если это preflight-запрос, сразу отвечаем
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
