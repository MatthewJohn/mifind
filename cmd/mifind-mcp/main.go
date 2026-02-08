package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	logger := log.With().Str("component", "mcp").Logger()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	logger.Info().Msg("Starting mifind MCP server")

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
			"instance_id":  "default",
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

	// Initialize MCP server
	mcpServer := api.NewMCPServer(providerManager, handlers, &logger)

	// Start MCP server in stdio mode
	logger.Info().Msg("MCP server running in stdio mode")
	runMCPServer(mcpServer, &logger)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown providers
	if err := providerManager.ShutdownAll(ctx); err != nil {
		logger.Error().Err(err).Msg("Provider shutdown failed")
	}

	logger.Info().Msg("Shutdown complete")
}

// Config holds the application configuration.
type Config struct {
	MockEnabled     bool `mapstructure:"mock_enabled"`
	MockEntityCount int  `mapstructure:"mock_entity_count"`
}

// loadConfig loads configuration from file and environment.
func loadConfig() (*Config, error) {
	// Set defaults
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

// registerCoreTypes registers core entity types from the central type definitions.
func registerCoreTypes(registry *types.TypeRegistry, logger zerolog.Logger) {
	types.RegisterCoreTypes(registry)

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

// runMCPServer runs the MCP server using stdio transport.
// This is a simplified implementation. A full implementation would use
// the actual MCP protocol SDK when available.
func runMCPServer(server *api.MCPServer, logger *zerolog.Logger) {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		// Read request from stdin
		var request MCPRequest
		if err := decoder.Decode(&request); err != nil {
			logger.Error().Err(err).Msg("Failed to decode request")
			continue
		}

		// Handle request
		response := handleMCPRequest(server, request, logger)

		// Write response to stdout
		if err := encoder.Encode(response); err != nil {
			logger.Error().Err(err).Msg("Failed to encode response")
		}
	}
}

// MCPRequest represents an MCP protocol request.
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP protocol response.
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handleMCPRequest handles an MCP request.
func handleMCPRequest(server *api.MCPServer, request MCPRequest, logger *zerolog.Logger) MCPResponse {
	ctx := context.Background()

	switch request.Method {
	case "tools/list":
		tools := server.ListTools()
		return MCPResponse{
			ID: request.ID,
			Result: map[string]interface{}{
				"tools": tools,
			},
		}

	case "tools/call":
		params, ok := request.Params["name"].(string)
		if !ok {
			return MCPResponse{
				ID: request.ID,
				Error: &MCPError{
					Code:    -32602,
					Message: "Invalid params: name is required",
				},
			}
		}

		toolName := params
		var args map[string]interface{}
		if argsMap, ok := request.Params["arguments"].(map[string]interface{}); ok {
			args = argsMap
		}

		toolResponse := server.CallToolWithResponse(ctx, toolName, args)
		if toolResponse.Error != nil {
			return MCPResponse{
				ID: request.ID,
				Error: &MCPError{
					Code:    toolResponse.Error.Code,
					Message: toolResponse.Error.Message,
				},
			}
		}

		return MCPResponse{
			ID:     request.ID,
			Result: toolResponse.Result,
		}

	case "initialize":
		return MCPResponse{
			ID: request.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]bool{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "mifind",
					"version": "0.1.0",
				},
			},
		}

	default:
		return MCPResponse{
			ID: request.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", request.Method),
			},
		}
	}
}
