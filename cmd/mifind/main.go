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
	"github.com/spf13/viper"

	"github.com/yourname/mifind/internal/api"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/provider/mock"
	"github.com/yourname/mifind/internal/search"
	"github.com/yourname/mifind/internal/types"
	"github.com/yourname/mifind/pkg/provider/filesystem"
	"github.com/yourname/mifind/pkg/provider/immich"
)

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	logger := log.With().Str("component", "main").Logger()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	logger.Info().
		Int("http_port", config.HTTPPort).
		Bool("ui_enabled", config.UI.Enabled).
		Msg("Starting mifind API server")

	// Initialize type registry
	typeRegistry := types.NewTypeRegistry()
	registerCoreTypes(typeRegistry, logger)

	// Initialize provider registry
	providerRegistry := provider.NewRegistry()

	// Register mock provider
	if err := providerRegistry.Register(provider.ProviderMetadata{
		Name:        "mock",
		Description: "Mock provider for testing",
		Factory:     func() provider.Provider { return mock.NewMockProvider() },
	}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register mock provider")
	}

	// Register filesystem provider
	if err := providerRegistry.Register(provider.ProviderMetadata{
		Name:        "filesystem",
		Description: "Filesystem provider via filesystem-api",
		Factory:     func() provider.Provider { return filesystem.NewProvider() },
	}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register filesystem provider")
	}

	// Register Immich provider
	if err := providerRegistry.Register(provider.ProviderMetadata{
		Name:        "immich",
		Description: "Immich photo and video server",
		Factory:     func() provider.Provider { return immich.NewProvider() },
	}); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register Immich provider")
	}

	// Initialize provider manager
	providerManager := provider.NewManager(providerRegistry, &logger)

	// Initialize mock provider
	if config.MockEnabled {
		mockConfig := map[string]any{
			"instance_id":  "default",
			"entity_count": config.MockEntityCount,
		}
		if err := providerManager.Initialize(context.Background(), "mock", mockConfig); err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize mock provider")
		} else {
			logger.Info().Int("count", config.MockEntityCount).Msg("Mock provider initialized")
		}
	}

	// Initialize filesystem providers
	for _, fsConfig := range config.FilesystemProviders {
		providerConfig := map[string]any{
			"instance_id": fsConfig.InstanceID,
			"url":         fsConfig.URL,
		}
		if fsConfig.APIKey != "" {
			providerConfig["api_key"] = fsConfig.APIKey
		}
		if err := providerManager.Initialize(context.Background(), "filesystem", providerConfig); err != nil {
			logger.Warn().Err(err).Str("instance", fsConfig.InstanceID).Msg("Failed to initialize filesystem provider")
		} else {
			logger.Info().Str("instance", fsConfig.InstanceID).Str("url", fsConfig.URL).Msg("Filesystem provider initialized")
		}
	}

	// Initialize Immich providers
	for _, immichConfig := range config.ImmichProviders {
		providerConfig := map[string]any{
			"instance_id":          immichConfig.InstanceID,
			"url":                  immichConfig.URL,
			"api_key":              immichConfig.APIKey,
			"insecure_skip_verify": immichConfig.InsecureSkipVerify,
		}
		if err := providerManager.Initialize(context.Background(), "immich", providerConfig); err != nil {
			logger.Warn().Err(err).Str("instance", immichConfig.InstanceID).Msg("Failed to initialize Immich provider")
		} else {
			logger.Info().Str("instance", immichConfig.InstanceID).Str("url", immichConfig.URL).Msg("Immich provider initialized")
		}
	}

	// Initialize search components
	rankingStrategy, err := createRankingStrategy(config.Ranking, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create ranking strategy")
	}
	logger.Info().Str("strategy", rankingStrategy.Name()).Msg("Ranking strategy initialized")

	federator := search.NewFederator(providerManager, rankingStrategy, &logger, 30*time.Second)
	ranker := search.NewRanker()
	filters := search.NewFilters(typeRegistry)
	relationships := search.NewRelationships(providerManager, &logger)

	// Initialize API handlers
	handlers := api.NewHandlers(providerManager, federator, ranker, filters, relationships, typeRegistry, &logger)

	// Setup HTTP server
	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Add middleware
	router.Use(loggingMiddleware(&logger))
	router.Use(corsMiddleware())

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.HTTPPort),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info().Int("port", config.HTTPPort).Msg("HTTP server listening")
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

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
	}

	// Shutdown providers
	if err := providerManager.ShutdownAll(ctx); err != nil {
		logger.Error().Err(err).Msg("Provider shutdown failed")
	}

	logger.Info().Msg("Shutdown complete")
}

// Config holds the application configuration.
type Config struct {
	HTTPPort            int                        `mapstructure:"http_port"`
	UI                  UIConfig                   `mapstructure:"ui"`
	Ranking             search.RankingConfig       `mapstructure:"ranking"`
	MockEnabled         bool                       `mapstructure:"mock_enabled"`
	MockEntityCount     int                        `mapstructure:"mock_entity_count"`
	FilesystemProviders []FilesystemProviderConfig `mapstructure:"filesystem_providers"`
	ImmichProviders     []ImmichProviderConfig     `mapstructure:"immich_providers"`
}

// UIConfig holds configuration for the web UI.
type UIConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	DevProxy  bool   `mapstructure:"dev_proxy"`
	DevURL    string `mapstructure:"dev_url"`
	IndexPath string `mapstructure:"index_path"`
}

// FilesystemProviderConfig holds configuration for a filesystem provider instance.
type FilesystemProviderConfig struct {
	InstanceID string `mapstructure:"instance_id"`
	URL        string `mapstructure:"url"`
	APIKey     string `mapstructure:"api_key"`
}

// ImmichProviderConfig holds configuration for an Immich provider instance.
type ImmichProviderConfig struct {
	InstanceID         string `mapstructure:"instance_id"`
	URL                string `mapstructure:"url"`
	APIKey             string `mapstructure:"api_key"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

// loadConfig loads configuration from file and environment.
func loadConfig() (*Config, error) {
	// Set defaults
	viper.SetDefault("http_port", 8080)
	viper.SetDefault("mock_enabled", true)
	viper.SetDefault("mock_entity_count", 10)

	// Read config file - check config/ dir first, then fallback locations
	viper.SetConfigName("mifind")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/mifind")
	viper.AddConfigPath("$HOME/.mifind")

	// Optional config file
	viper.ReadInConfig()

	// Environment variables
	viper.SetEnvPrefix("MIFIND")
	viper.AutomaticEnv()

	// Unmarshal config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// registerCoreTypes registers core entity types from the central type definitions.
func registerCoreTypes(registry *types.TypeRegistry, logger zerolog.Logger) {
	types.RegisterCoreTypes(registry)
	logger.Info().Int("count", len(registry.List())).Msg("Core types registered")
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, status: 200}

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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// createRankingStrategy creates a ranking strategy based on configuration.
func createRankingStrategy(config search.RankingConfig, logger *zerolog.Logger) (search.RankingStrategy, error) {
	switch config.Strategy {
	case "meilisearch":
		ranker, err := search.NewMeilisearchRanker(config, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to create Meilisearch ranker, falling back to in-memory")
			return search.NewInMemoryRanker(config), nil
		}
		return ranker, nil
	case "in-memory", "":
		return search.NewInMemoryRanker(config), nil
	default:
		logger.Warn().Str("strategy", config.Strategy).Msg("Unknown ranking strategy, using in-memory")
		return search.NewInMemoryRanker(config), nil
	}
}
