package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// Entity ID format: "providerType:instanceID:entityID"
// Example: "filesystem:myfs:abc123"

const (
	// EntityIDSeparator is the separator used in entity IDs
	EntityIDSeparator = ":"

	// NumIDParts is the expected number of parts in an entity ID
	NumIDParts = 3
)

// EntityID represents a unique entity identifier with format "providerType:instanceID:entityID".
type EntityID string

// NewEntityID creates an EntityID from its components.
func NewEntityID(providerType, instanceID, entityID string) EntityID {
	return EntityID(providerType + EntityIDSeparator + instanceID + EntityIDSeparator + entityID)
}

// ParseEntityID parses an EntityID from a string.
func ParseEntityID(s string) (EntityID, error) {
	parts := strings.Split(s, EntityIDSeparator)
	if len(parts) != NumIDParts {
		return EntityID(""), fmt.Errorf("invalid entity ID format: expected %d parts separated by %q, got %d", NumIDParts, EntityIDSeparator, len(parts))
	}
	return EntityID(s), nil
}

// MustParseEntityID parses an EntityID and panics on error.
func MustParseEntityID(s string) EntityID {
	id, err := ParseEntityID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string representation of the EntityID.
func (e EntityID) String() string {
	return string(e)
}

// ProviderType returns the provider type part of the ID.
func (e EntityID) ProviderType() string {
	parts := strings.Split(string(e), EntityIDSeparator)
	if len(parts) == NumIDParts {
		return parts[0]
	}
	return ""
}

// InstanceID returns the instance ID part of the ID.
func (e EntityID) InstanceID() string {
	parts := strings.Split(string(e), EntityIDSeparator)
	if len(parts) == NumIDParts {
		return parts[1]
	}
	return ""
}

// ResourceID returns the resource/entity ID part of the ID.
func (e EntityID) ResourceID() string {
	parts := strings.Split(string(e), EntityIDSeparator)
	if len(parts) == NumIDParts {
		return parts[2]
	}
	return ""
}

// Parts returns all three parts of the ID (providerType, instanceID, resourceID).
func (e EntityID) Parts() (string, string, string) {
	parts := strings.Split(string(e), EntityIDSeparator)
	if len(parts) == NumIDParts {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

// IsValid checks if the EntityID has a valid format.
func (e EntityID) IsValid() bool {
	parts := strings.Split(string(e), EntityIDSeparator)
	return len(parts) == NumIDParts &&
		parts[0] != "" &&
		parts[1] != "" &&
		parts[2] != ""
}

// BuildEntityID creates an EntityID from components.
// This is a convenience function equivalent to NewEntityID.
func BuildEntityID(providerType, instanceID, entityID string) EntityID {
	return NewEntityID(providerType, instanceID, entityID)
}

// FilterValuesProvider is an optional interface that providers can implement
// to return pre-obtained filter values (values known from the source without searching).
// Examples: People/Albums in Immich, Genres in Jellyfin, Artists in music libraries.
type FilterValuesProvider interface {
	// FilterValues returns available filter values for the given filter name.
	// Returns an empty slice if the filter is not supported or has no pre-obtained values.
	FilterValues(ctx context.Context, filterName string) ([]FilterOption, error)
}

// FilterCapability describes how a provider supports filtering on a specific attribute.
// This is runtime-discoverable and provider-specific, allowing each provider to declare
// which attributes can be filtered on and how.
//
// This separates the attribute schema (what exists) from filter capability (what can be filtered).
// Core types define what attributes ARE available, while providers declare what CAN be filtered.
type FilterCapability struct {
	// Type is the attribute type
	Type types.AttributeType

	// SupportsEq indicates if equality filtering is supported (e.g., "extension=jpg")
	SupportsEq bool

	// SupportsNeq indicates if inequality filtering is supported (e.g., "extension!=jpg")
	SupportsNeq bool

	// SupportsRange indicates if range filtering is supported (e.g., "size>1000", "width<1920")
	SupportsRange bool

	// SupportsGlob indicates if glob pattern matching is supported (e.g., "path=*.jpg")
	SupportsGlob bool

	// SupportsContains indicates if substring matching is supported (e.g., "title~vacation")
	SupportsContains bool

	// Min is the minimum value for range filters (nil if no minimum)
	Min *float64

	// Max is the maximum value for range filters (nil if no maximum)
	Max *float64

	// Options are the valid options for select/multi-select attributes
	// (nil if not an enumerated type)
	Options []FilterOption

	// Description is a human-readable description of this filter
	Description string
}

// FilterOption represents a single option for enumerated type filters.
type FilterOption struct {
	// Value is the option value
	Value string

	// Label is the human-readable label
	Label string

	// Count is the number of entities with this value (optional, for faceted search)
	Count int
}

// Provider defines the interface that all data source providers must implement.
// Providers are responsible for discovering, searching, and hydrating entities
// from their respective data sources.
//
// Entity ID format: "providerType:instanceID:entityID"
// Example: "filesystem:myfs:abc123"
type Provider interface {
	// Name returns the unique name of this provider (e.g., "filesystem", "immich").
	Name() string

	// InstanceID returns the instance ID for this provider (e.g., "myfs", "photos").
	// This is set during initialization via the "instance_id" config field.
	InstanceID() string

	// Initialize sets up the provider with the given configuration.
	// Called once when the provider is registered.
	// The config must include an "instance_id" field.
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

	// FilterCapabilities returns the filter capabilities for each attribute.
	// This is runtime-discoverable and provider-specific, allowing each provider
	// to declare which attributes can be filtered on and how.
	// Returns a map of attribute name to FilterCapability.
	FilterCapabilities(ctx context.Context) (map[string]FilterCapability, error)

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
	config     map[string]any
	meta       ProviderMetadata
	instanceID string // Provider instance ID (set during initialization)
}

// NewBaseProvider creates a new base provider with the given metadata.
func NewBaseProvider(meta ProviderMetadata) *BaseProvider {
	return &BaseProvider{
		config: make(map[string]any),
		meta:   meta,
	}
}

// SetInstanceID sets the provider instance ID.
// This should be called during Initialize() after reading from config.
func (b *BaseProvider) SetInstanceID(instanceID string) {
	b.instanceID = instanceID
}

// InstanceID returns the provider instance ID.
func (b *BaseProvider) InstanceID() string {
	return b.instanceID
}

// BuildEntityID creates an entity ID using this provider's type and instance ID.
func (b *BaseProvider) BuildEntityID(entityID string) EntityID {
	return NewEntityID(b.meta.Name, b.instanceID, entityID)
}

// StandardConfigFields returns standard configuration fields that all providers should support.
func StandardConfigFields() map[string]ConfigField {
	return map[string]ConfigField{
		"instance_id": {
			Type:        "string",
			Required:    true,
			Description: "Unique ID for this provider instance (e.g., 'myfs', 'photos')",
		},
	}
}

// AddStandardConfigFields adds standard config fields to a config schema.
func AddStandardConfigFields(schema map[string]ConfigField) map[string]ConfigField {
	if schema == nil {
		schema = make(map[string]ConfigField)
	}
	for k, v := range StandardConfigFields() {
		schema[k] = v
	}
	return schema
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

// FilterCapabilities returns an empty map by default.
// Providers that support filtering should override this to declare their capabilities.
func (b *BaseProvider) FilterCapabilities(ctx context.Context) (map[string]FilterCapability, error) {
	return make(map[string]FilterCapability), nil
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
