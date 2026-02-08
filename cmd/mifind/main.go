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
)

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

	logger.Info().
		Int("http_port", config.HTTPPort).
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

	// Initialize provider manager
	providerManager := provider.NewManager(providerRegistry, &logger)

	// Initialize mock provider
	if config.MockEnabled {
		mockConfig := map[string]any{
			"entity_count": config.MockEntityCount,
		}
		if err := providerManager.Initialize(context.Background(), "mock", mockConfig); err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize mock provider")
		} else {
			logger.Info().Int("count", config.MockEntityCount).Msg("Mock provider initialized")
		}
	}

	// Initialize search components
	federator := search.NewFederator(providerManager, &logger, 30*time.Second)
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
	HTTPPort        int  `mapstructure:"http_port"`
	MockEnabled     bool `mapstructure:"mock_enabled"`
	MockEntityCount int  `mapstructure:"mock_entity_count"`
}

// loadConfig loads configuration from file and environment.
func loadConfig() (*Config, error) {
	// Set defaults
	viper.SetDefault("http_port", 8080)
	viper.SetDefault("mock_enabled", true)
	viper.SetDefault("mock_entity_count", 10)

	// Read config file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
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

// registerCoreTypes registers core entity types.
func registerCoreTypes(registry *types.TypeRegistry, logger zerolog.Logger) {
	// Register root types
	registry.Register(types.TypeDefinition{
		Name:        "item",
		Description: "Base type for all items",
		Attributes:  make(map[string]types.AttributeDef),
		Filters:     []types.FilterDefinition{},
	})

	registry.Register(types.TypeDefinition{
		Name:        "media",
		Description: "Base type for media items",
		Parent:      "item",
		Attributes:  make(map[string]types.AttributeDef),
		Filters:     []types.FilterDefinition{},
	})

	registry.Register(types.TypeDefinition{
		Name:        "collection",
		Description: "Base type for collections",
		Parent:      "item",
		Attributes:  make(map[string]types.AttributeDef),
		Filters:     []types.FilterDefinition{},
	})

	// Register file types
	registry.Register(types.TypeDefinition{
		Name:   "file",
		Parent: "item",
		Attributes: map[string]types.AttributeDef{
			types.AttrPath:      {Name: types.AttrPath, Type: types.AttributeTypeString, Filterable: true},
			types.AttrSize:      {Name: types.AttrSize, Type: types.AttributeTypeInt64, Filterable: true},
			types.AttrExtension: {Name: types.AttrExtension, Type: types.AttributeTypeString, Filterable: true},
			types.AttrMimeType:  {Name: types.AttrMimeType, Type: types.AttributeTypeString, Filterable: true},
			types.AttrModified:  {Name: types.AttrModified, Type: types.AttributeTypeTime, Filterable: true},
		},
		Filters: []types.FilterDefinition{
			{Name: types.AttrExtension, Type: types.FilterTypeSelect, Label: "Extension"},
			{Name: types.AttrMimeType, Type: types.FilterTypeSelect, Label: "MIME Type"},
		},
	})

	registry.Register(types.TypeDefinition{
		Name:        "file.media",
		Parent:      "file",
		Description: "Media file",
	})

	registry.Register(types.TypeDefinition{
		Name:        "file.media.video",
		Parent:      "file.media",
		Description: "Video file",
		Attributes: map[string]types.AttributeDef{
			types.AttrDuration: {Name: types.AttrDuration, Type: types.AttributeTypeInt64, Filterable: true},
			types.AttrWidth:    {Name: types.AttrWidth, Type: types.AttributeTypeInt, Filterable: true},
			types.AttrHeight:   {Name: types.AttrHeight, Type: types.AttributeTypeInt, Filterable: true},
		},
	})

	registry.Register(types.TypeDefinition{
		Name:        "file.media.image",
		Parent:      "file.media",
		Description: "Image file",
		Attributes: map[string]types.AttributeDef{
			types.AttrWidth:  {Name: types.AttrWidth, Type: types.AttributeTypeInt, Filterable: true},
			types.AttrHeight: {Name: types.AttrHeight, Type: types.AttributeTypeInt, Filterable: true},
			types.AttrCamera: {Name: types.AttrCamera, Type: types.AttributeTypeString, Filterable: true},
			types.AttrGPS:    {Name: types.AttrGPS, Type: types.AttributeTypeGPS, Filterable: true},
		},
	})

	registry.Register(types.TypeDefinition{
		Name:        "file.document",
		Parent:      "file",
		Description: "Document file",
	})

	// Register media asset types
	registry.Register(types.TypeDefinition{
		Name:        "media.asset",
		Parent:      "media",
		Description: "Media asset",
	})

	registry.Register(types.TypeDefinition{
		Name:        "media.asset.photo",
		Parent:      "media.asset",
		Description: "Photo asset",
		Attributes: map[string]types.AttributeDef{
			types.AttrCamera: {Name: types.AttrCamera, Type: types.AttributeTypeString, Filterable: true},
			types.AttrGPS:    {Name: types.AttrGPS, Type: types.AttributeTypeGPS, Filterable: true},
		},
	})

	registry.Register(types.TypeDefinition{
		Name:        "media.asset.video",
		Parent:      "media.asset",
		Description: "Video asset",
		Attributes: map[string]types.AttributeDef{
			types.AttrDuration: {Name: types.AttrDuration, Type: types.AttributeTypeInt64, Filterable: true},
		},
	})

	// Register collection types
	registry.Register(types.TypeDefinition{
		Name:        "collection.album",
		Parent:      "collection",
		Description: "Album collection",
		Attributes: map[string]types.AttributeDef{
			types.AttrAlbum: {Name: types.AttrAlbum, Type: types.AttributeTypeString, Filterable: true},
		},
	})

	registry.Register(types.TypeDefinition{
		Name:        "collection.folder",
		Parent:      "collection",
		Description: "Folder collection",
	})

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
