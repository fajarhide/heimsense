package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fajarhide/heimsense/internal/client"
	"github.com/fajarhide/heimsense/internal/config"
	"github.com/fajarhide/heimsense/internal/handler"
)

func main() {
	// Structured logger.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load config.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded",
		"listen_addr", cfg.ListenAddr,
		"upstream_url", cfg.UpstreamBaseURL,
		"default_model", cfg.DefaultModel,
		"force_model", cfg.ForceModel,
		"request_timeout", cfg.RequestTimeout,
		"max_retries", cfg.MaxRetries,
	)

	// Initialize client and handler.
	oaiClient := client.New(cfg, logger)
	messagesHandler := handler.NewMessagesHandler(oaiClient, cfg, logger)

	// Routes.
	mux := http.NewServeMux()
	mux.Handle("/v1/messages", messagesHandler)
	mux.HandleFunc("/health", handler.HealthHandler)

	// Wrap with logging middleware.
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      loggingMiddleware(logger, mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: cfg.RequestTimeout + 10*time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

// loggingMiddleware logs every incoming HTTP request.
func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for streaming support.
func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
