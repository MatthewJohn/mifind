package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/yourname/mifind/internal/filesystem"
)

const version = "0.1.0"

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	logger := log.With().Str("component", "main").Logger()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Set log level from config
	setLogLevel(config.Logging.Level, &logger)

	logger.Info().
		Int("port", config.Server.Port).
		Str("version", version).
		Msg("Starting filesystem-api server")

	// Initialize service
	svc, err := filesystem.NewService(config, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize service")
	}

	// Initialize API handlers
	handlers := filesystem.NewHandlers(svc, &logger, version)

	// Setup HTTP server
	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Add middleware
	router.Use(loggingMiddleware(&logger))
	if config.API.CORSEnabled {
		router.Use(corsMiddleware())
	}
	if config.API.APIKey != "" {
		router.Use(authMiddleware(config.API.APIKey, &logger))
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Server.Port),
		Handler:      router,
		ReadTimeout:  config.GetReadTimeout(),
		WriteTimeout: config.GetWriteTimeout(),
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info().Int("port", config.Server.Port).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown service
	if err := svc.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Service shutdown failed")
	}

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
	}

	logger.Info().Msg("Shutdown complete")
}

// loadConfig loads configuration from file and environment.
func loadConfig() (*filesystem.Config, error) {
	return filesystem.LoadConfig()
}

// setLogLevel sets the global log level.
func setLogLevel(level string, logger *zerolog.Logger) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.status).
				Dur("duration", time.Since(start)).
				Msg("HTTP request")
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// corsMiddleware adds CORS headers.
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// authMiddleware validates API key if configured.
func authMiddleware(apiKey string, logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health check
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Check API key
			receivedKey := r.Header.Get("X-API-Key")
			if receivedKey != apiKey {
				logger.Warn().Str("path", r.URL.Path).Msg("Invalid API key")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
