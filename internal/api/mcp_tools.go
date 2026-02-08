package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/search"
	"github.com/yourname/mifind/internal/types"
)

// MCPServer provides MCP (Model Context Protocol) tools for AI agent integration.
// Note: This is a placeholder implementation. The actual MCP protocol support
// would use github.com/modelcontextprotocol/sdk-go when available.
type MCPServer struct {
	manager  *provider.Manager
	handlers *Handlers
	logger   *zerolog.Logger
}

// NewMCPServer creates a new MCP server instance.
func NewMCPServer(manager *provider.Manager, handlers *Handlers, logger *zerolog.Logger) *MCPServer {
	return &MCPServer{
		manager:  manager,
		handlers: handlers,
		logger:   logger,
	}
}

// MCPTool represents an MCP tool definition.
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ListTools returns all available MCP tools.
func (m *MCPServer) ListTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "search_entities",
			Description: "Search across all personal data sources for entities matching the query",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query string",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by entity type (optional)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (optional)",
					},
					"filters": map[string]interface{}{
						"type":        "object",
						"description": "Attribute filters to apply (optional)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "describe_entity",
			Description: "Get full details of a specific entity by its ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The entity ID",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "expand_entity",
			Description: "Get an entity with all its related entities populated",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The entity ID",
					},
					"depth": map[string]interface{}{
						"type":        "integer",
						"description": "How deep to follow relationships (default: 1)",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "list_types",
			Description: "List all available entity types in the system",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_type",
			Description: "Get details about a specific entity type",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The type name",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "get_related",
			Description: "Get entities related to a specific entity",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The entity ID",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by relationship type (optional)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results (optional)",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "get_filters",
			Description: "Get available filters for a search query",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"search": map[string]interface{}{
						"type":        "string",
						"description": "The search query to get filters for",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by entity type (optional)",
					},
				},
				"required": []string{"search"},
			},
		},
	}
}

// CallTool executes an MCP tool call.
func (m *MCPServer) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "search_entities":
		return m.searchEntities(ctx, args)
	case "describe_entity":
		return m.describeEntity(ctx, args)
	case "expand_entity":
		return m.expandEntity(ctx, args)
	case "list_types":
		return m.listTypes(ctx, args)
	case "get_type":
		return m.getType(ctx, args)
	case "get_related":
		return m.getRelated(ctx, args)
	case "get_filters":
		return m.getFilters(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// search_entities implementation
func (m *MCPServer) searchEntities(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Build search query
	searchQuery := search.NewSearchQuery(query)

	if typeName, ok := args["type"].(string); ok {
		searchQuery.Type = typeName
	}

	if limit, ok := args["limit"].(float64); ok {
		searchQuery.Limit = int(limit)
	}

	if filters, ok := args["filters"].(map[string]interface{}); ok {
		searchQuery.Filters = filters
	}

	// Execute search
	response := m.handlers.federator.Search(ctx, searchQuery)
	result := m.handlers.ranker.Rank(response, searchQuery)

	// Return simplified format for AI consumption
	entities := make([]map[string]interface{}, 0, len(result.Entities))
	for _, ranked := range result.Entities {
		entities = append(entities, map[string]interface{}{
			"id":          ranked.Entity.ID,
			"type":        ranked.Entity.Type,
			"title":       ranked.Entity.Title,
			"description": ranked.Entity.Description,
			"provider":    ranked.Provider,
			"attributes":  ranked.Entity.Attributes,
			"score":       ranked.Score,
		})
	}

	return map[string]interface{}{
		"entities":    entities,
		"total_count": result.TotalCount,
		"type_counts": result.TypeCounts,
	}, nil
}

// describe_entity implementation
func (m *MCPServer) describeEntity(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	// Get entity from provider manager
	entity, err := m.manager.Hydrate(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	return map[string]interface{}{
		"id":            entity.ID,
		"type":          entity.Type,
		"title":         entity.Title,
		"description":   entity.Description,
		"provider":      entity.Provider,
		"attributes":    entity.Attributes,
		"relationships": entity.Relationships,
		"search_tokens": entity.SearchTokens,
		"timestamp":     entity.Timestamp,
	}, nil
}

// expand_entity implementation
func (m *MCPServer) expandEntity(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	maxDepth := 1
	if depth, ok := args["depth"].(float64); ok {
		maxDepth = int(depth)
	}

	expanded, err := m.handlers.relationships.Expand(ctx, id, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to expand entity: %w", err)
	}

	// Convert to AI-friendly format
	result := map[string]interface{}{
		"id":                  expanded.Entity.ID,
		"type":                expanded.Entity.Type,
		"title":               expanded.Entity.Title,
		"description":         expanded.Entity.Description,
		"provider":            expanded.Entity.Provider,
		"attributes":          expanded.Entity.Attributes,
		"related":             expanded.Related,
		"relationship_counts": expanded.RelationshipCount(),
	}

	return result, nil
}

// list_types implementation
func (m *MCPServer) listTypes(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	allTypes := m.handlers.typeRegistry.GetAll()

	typeList := make([]map[string]interface{}, 0, len(allTypes))
	for _, t := range allTypes {
		typeList = append(typeList, map[string]interface{}{
			"name":        t.Name,
			"parent":      t.Parent,
			"description": t.Description,
		})
	}

	return map[string]interface{}{
		"types": typeList,
		"count": len(typeList),
	}, nil
}

// get_type implementation
func (m *MCPServer) getType(_ context.Context, args map[string]interface{}) (interface{}, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name is required")
	}

	typeDef := m.handlers.typeRegistry.Get(name)
	if typeDef == nil {
		return nil, fmt.Errorf("type not found: %s", name)
	}

	// Get ancestors
	ancestors := m.handlers.typeRegistry.GetAncestors(name)
	ancestorNames := make([]string, len(ancestors))
	for i, a := range ancestors {
		ancestorNames[i] = a.Name
	}

	return map[string]interface{}{
		"name":        typeDef.Name,
		"parent":      typeDef.Parent,
		"ancestors":   ancestorNames,
		"description": typeDef.Description,
		"attributes":  typeDef.Attributes,
		"filters":     typeDef.Filters,
	}, nil
}

// get_related implementation
func (m *MCPServer) getRelated(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	relType := ""
	if rt, ok := args["type"].(string); ok {
		relType = rt
	}

	limit := 0
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	entities, err := m.handlers.relationships.GetRelated(ctx, id, relType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get related: %w", err)
	}

	// Convert to AI-friendly format
	related := make([]map[string]interface{}, 0, len(entities))
	for _, e := range entities {
		related = append(related, map[string]interface{}{
			"id":          e.ID,
			"type":        e.Type,
			"title":       e.Title,
			"description": e.Description,
			"provider":    e.Provider,
			"attributes":  e.Attributes,
		})
	}

	return map[string]interface{}{
		"entities": related,
		"count":    len(related),
	}, nil
}

// get_filters implementation
func (m *MCPServer) getFilters(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	searchQuery, ok := args["search"].(string)
	if !ok || searchQuery == "" {
		return nil, fmt.Errorf("search is required")
	}

	typeName := ""
	if tn, ok := args["type"].(string); ok {
		typeName = tn
	}

	// Execute search to get entities
	query := search.NewSearchQuery(searchQuery)
	query.Limit = 100

	response := m.handlers.federator.Search(ctx, query)
	result := m.handlers.ranker.Rank(response, query)

	// Extract entities
	entities := make([]types.Entity, len(result.Entities))
	for i, ranked := range result.Entities {
		entities[i] = ranked.Entity
	}

	// Extract filters
	filterResult := m.handlers.filters.ExtractFilters(entities, typeName)

	return filterResult, nil
}

// MCPError represents an MCP tool error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// NewMCPError creates a new MCP error.
func NewMCPError(code int, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
	}
}

// MCPResponse represents a standardized MCP tool response.
type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// ToJSON converts the MCP response to JSON.
func (r *MCPResponse) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// NewMCPResponse creates a new successful MCP response.
func NewMCPResponse(result interface{}) *MCPResponse {
	return &MCPResponse{
		Result: result,
	}
}

// NewMCPErrorResponse creates a new error MCP response.
func NewMCPErrorResponse(code int, message string) *MCPResponse {
	return &MCPResponse{
		Error: NewMCPError(code, message),
	}
}

// CallToolWithResponse executes an MCP tool and returns a standardized response.
func (m *MCPServer) CallToolWithResponse(ctx context.Context, name string, args map[string]interface{}) *MCPResponse {
	result, err := m.CallTool(ctx, name, args)
	if err != nil {
		if err == provider.ErrNotFound {
			return NewMCPErrorResponse(404, err.Error())
		}
		return NewMCPErrorResponse(500, err.Error())
	}
	return NewMCPResponse(result)
}
