package provider

import (
	"context"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// Provider defines the interface that all data source providers must implement.
// Providers are responsible for discovering, searching, and hydrating entities
// from their respective data sources.
type Provider interface {
	// Name returns the unique name of this provider (e.g., "filesystem", "immich").
	Name() string

	// Initialize sets up the provider with the given configuration.
	// Called once when the provider is registered.
	Initialize(ctx context.Context, config map[string]any) error

	// Discover performs a full discovery of all entities in this provider.
	// Returns a slice of lightweight entity summaries.
	// For large providers, this should return paginated results or use
	// a cursor mechanism (to be added as needed).
	Discover(ctx context.Context) ([]types.Entity, error)

	// Hydrate retrieves the full details of a specific entity by ID.
	// Returns the complete entity with all attributes and relationships.
	Hydrate(ctx context.Context, id string) (types.Entity, error)

	// GetRelated retrieves entities related to the given entity ID.
	// relType specifies which relationship type to follow (empty for all).
	// Returns a slice of related entities.
	GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error)

	// Search performs a search query on this provider.
	// Returns matching entities filtered according to the query.
	Search(ctx context.Context, query SearchQuery) ([]types.Entity, error)

	// SupportsIncremental returns true if this provider supports incremental updates.
	// If true, the provider may implement DiscoverSince for more efficient syncing.
	SupportsIncremental() bool

	// DiscoverSince performs an incremental discovery since the given timestamp.
	// Only called if SupportsIncremental returns true.
	// Returns entities that were created or modified since the cutoff time.
	DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error)

	// Shutdown gracefully shuts down the provider.
	// Called when the provider is being unregistered or the service is stopping.
	Shutdown(ctx context.Context) error
}

// SearchQuery defines a search query to be executed against a provider.
type SearchQuery struct {
	// Query is the search string (may be empty for match-all queries)
	Query string

	// Filters specifies attribute filters to apply
	Filters map[string]any

	// Type filters by entity type (empty for all types)
	Type string

	// RelationshipType filters by relationship type (optional)
	RelationshipType string

	// Limit specifies the maximum number of results to return
	// 0 means no limit (provider default)
	Limit int

	// Offset specifies the number of results to skip
	Offset int
}

// SearchResult contains search results with metadata.
type SearchResult struct {
	// Entities is the list of matching entities
	Entities []types.Entity

	// TotalCount is the total number of matches (may be greater than len(Entities))
	TotalCount int

	// TypeCounts breaks down the result count by entity type
	TypeCounts map[string]int

	// HasMore indicates whether more results are available
	HasMore bool
}

// ProviderStatus represents the current status of a provider.
type ProviderStatus struct {
	// Name is the provider name
	Name string

	// Connected indicates whether the provider is connected
	Connected bool

	// LastDiscovery is the timestamp of the last successful discovery
	LastDiscovery time.Time

	// LastError is the last error encountered (if any)
	LastError string

	// EntityCount is the number of entities known from this provider
	EntityCount int

	// SupportsIncremental indicates if provider supports incremental updates
	SupportsIncremental bool
}

// ProviderFactory creates new provider instances.
type ProviderFactory func() Provider

// ProviderMetadata describes a provider type for registration.
type ProviderMetadata struct {
	// Name is the unique provider name
	Name string

	// Description describes what this provider does
	Description string

	// ConfigSchema defines the expected configuration structure
	// (used for validation and documentation)
	ConfigSchema map[string]ConfigField

	// Factory creates new provider instances
	Factory ProviderFactory
}

// ConfigField describes a configuration field for a provider.
type ConfigField struct {
	// Type is the field type (string, int, bool, etc.)
	Type string

	// Required indicates if the field is required
	Required bool

	// Description describes the field's purpose
	Description string

	// Default is the default value (optional)
	Default any
}

// ProviderOption is a functional option for configuring provider behavior.
type ProviderOption func(*ProviderConfig)

// ProviderConfig contains common provider configuration.
type ProviderConfig struct {
	// Timeout is the timeout for provider operations
	Timeout time.Duration

	// RetryCount is the number of retries for failed operations
	RetryCount int

	// RateLimit is the maximum requests per second (0 = no limit)
	RateLimit float64

	// EnableCaching enables caching for this provider
	EnableCaching bool

	// CacheTTL is the cache TTL for this provider
	CacheTTL time.Duration

	// Custom contains provider-specific custom configuration
	Custom map[string]any
}

// NewProviderConfig creates a default provider config.
func NewProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		Timeout:       30 * time.Second,
		RetryCount:    3,
		RateLimit:     0,
		EnableCaching: true,
		CacheTTL:      5 * time.Minute,
		Custom:        make(map[string]any),
	}
}

// WithTimeout sets the timeout for provider operations.
func WithTimeout(timeout time.Duration) ProviderOption {
	return func(c *ProviderConfig) {
		c.Timeout = timeout
	}
}

// WithRetryCount sets the retry count for failed operations.
func WithRetryCount(count int) ProviderOption {
	return func(c *ProviderConfig) {
		c.RetryCount = count
	}
}

// WithRateLimit sets the rate limit (requests per second).
func WithRateLimit(rps float64) ProviderOption {
	return func(c *ProviderConfig) {
		c.RateLimit = rps
	}
}

// WithCaching enables or disables caching.
func WithCaching(enabled bool) ProviderOption {
	return func(c *ProviderConfig) {
		c.EnableCaching = enabled
	}
}

// WithCacheTTL sets the cache TTL.
func WithCacheTTL(ttl time.Duration) ProviderOption {
	return func(c *ProviderConfig) {
		c.CacheTTL = ttl
	}
}

// WithCustom sets a custom configuration value.
func WithCustom(key string, value any) ProviderOption {
	return func(c *ProviderConfig) {
		if c.Custom == nil {
			c.Custom = make(map[string]any)
		}
		c.Custom[key] = value
	}
}

// BaseProvider provides a base implementation that providers can embed.
// It handles common functionality like config storage.
type BaseProvider struct {
	config map[string]any
	meta   ProviderMetadata
}

// NewBaseProvider creates a new base provider with the given metadata.
func NewBaseProvider(meta ProviderMetadata) *BaseProvider {
	return &BaseProvider{
		config: make(map[string]any),
		meta:   meta,
	}
}

// GetConfig retrieves a configuration value by key.
func (b *BaseProvider) GetConfig(key string) (any, bool) {
	val, ok := b.config[key]
	return val, ok
}

// GetConfigString retrieves a string configuration value.
func (b *BaseProvider) GetConfigString(key string) string {
	if val, ok := b.GetConfig(key); ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetConfigInt retrieves an int configuration value.
func (b *BaseProvider) GetConfigInt(key string) int {
	if val, ok := b.GetConfig(key); ok {
		// Try various int types
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// GetConfigBool retrieves a bool configuration value.
func (b *BaseProvider) GetConfigBool(key string) bool {
	if val, ok := b.GetConfig(key); ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// SetConfig sets a configuration value.
func (b *BaseProvider) SetConfig(key string, value any) {
	b.config[key] = value
}

// Metadata returns the provider's metadata.
func (b *BaseProvider) Metadata() ProviderMetadata {
	return b.meta
}

// SupportsIncremental returns false by default.
// Providers that support incremental updates should override this.
func (b *BaseProvider) SupportsIncremental() bool {
	return false
}

// DiscoverSince returns an error by default.
// Providers that support incremental updates should override this.
func (b *BaseProvider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	return nil, ErrIncrementalNotSupported
}

// Shutdown is a no-op by default.
// Providers with cleanup needs should override this.
func (b *BaseProvider) Shutdown(ctx context.Context) error {
	return nil
}

// Common errors returned by providers.
var (
	// ErrNotConfigured is returned when a provider is not properly configured.
	ErrNotConfigured = &ProviderError{Type: ErrorTypeConfig, Message: "provider not configured"}

	// ErrAuthenticationFailed is returned when authentication fails.
	ErrAuthenticationFailed = &ProviderError{Type: ErrorTypeAuth, Message: "authentication failed"}

	// ErrNotFound is returned when an entity is not found.
	ErrNotFound = &ProviderError{Type: ErrorTypeNotFound, Message: "entity not found"}

	// ErrIncrementalNotSupported is returned when DiscoverSince is called on a provider
	// that doesn't support incremental updates.
	ErrIncrementalNotSupported = &ProviderError{Type: ErrorTypeNotSupported, Message: "incremental discovery not supported"}

	// ErrRateLimited is returned when rate limit is exceeded.
	ErrRateLimited = &ProviderError{Type: ErrorTypeRateLimit, Message: "rate limit exceeded"}

	// ErrTemporary is returned for temporary failures that may be retried.
	ErrTemporary = &ProviderError{Type: ErrorTypeTemporary, Message: "temporary failure"}
)

// ErrorType represents the category of a provider error.
type ErrorType string

const (
	ErrorTypeConfig       ErrorType = "config"
	ErrorTypeAuth         ErrorType = "auth"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeNotSupported ErrorType = "not_supported"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeTemporary    ErrorType = "temporary"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// ProviderError represents an error from a provider.
type ProviderError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error returns the error message.
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error.
func NewProviderError(errType ErrorType, message string, err error) *ProviderError {
	return &ProviderError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
