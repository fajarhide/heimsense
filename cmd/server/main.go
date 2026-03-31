package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/fajarhide/heimsense/internal/client"
	"github.com/fajarhide/heimsense/internal/config"
	"github.com/fajarhide/heimsense/internal/handler"
	"github.com/fajarhide/heimsense/internal/setup"
)

// Version is the current version of the Heimsense binary.
// Can be overridden via -ldflags="-X main.Version=v0.1.x"
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	// Handle subcommands.
	switch command {
	case "setup":
		if err := setup.RunWizard(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Setup failed: %v\n", err)
			os.Exit(1)
		}
		// Set command to run so it continues to start server
		command = "run"
	case "run":
		// Check first-run config
		if setup.NeedsSetup() {
			fmt.Println("  ℹ First run detected. Let's configure your setup.")
			if err := setup.RunWizard(); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Setup failed: %v\n", err)
				os.Exit(1)
			}
		}
		// Continues outside the switch to start the server.
	case "sync":
		if err := setup.SyncToClaude(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Sync failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "version", "-v", "--version":
		fmt.Printf("heimsense version %s\n", getVersion())
		os.Exit(0)
	case "help", "-h", "--help":
		printHelp()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}

	if command != "run" {
		return // Should not be accessible due to os.Exit in other cases, but just to be safe.
	}

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

func printHelp() {
	fmt.Println()
	fmt.Printf("  %sHEIM·SENSE%s\n", "\033[1;36m", "\033[0m")
	fmt.Printf("  %sUnlock Your Claude Code for Any LLM%s\n", "\033[3;36m", "\033[0m")
	fmt.Println()
	fmt.Println("  Usage:")
	fmt.Println("    heimsense [command]")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    setup    Launch the interactive setup wizard to configure provider, model, and port.")
	fmt.Println("    run      Start the Heimsense server (also runs setup if config is missing).")
	fmt.Println("    sync     Read ~/.heimsense/.env and sync its values to ~/.claude/settings.json.")
	fmt.Println("    version  Show current version.")
	fmt.Println("    help     Show this help message.")
	fmt.Println()
	fmt.Println("  Examples:")
	fmt.Println("    heimsense setup    (Run interactive configuration)")
	fmt.Println("    heimsense run      (Start background daemon/server)")
	fmt.Println("    heimsense sync     (Sync manual .env changes to Claude Code)")
	fmt.Println()
}

// getVersion returns the current version, optionally falling back to vcs info.
func getVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "(devel)" && info.Main.Version != "" {
			return info.Main.Version
		}
		var revision, modified string
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				modified = "-dirty"
			}
		}
		if revision != "" {
			if len(revision) > 7 {
				revision = revision[:7]
			}
			return fmt.Sprintf("dev-%s%s", revision, modified)
		}
	}
	return Version
}
