package provider

import (
	"fmt"
	"sync"
)

// Registry manages provider type registration and instantiation.
type Registry struct {
	mu    sync.RWMutex
	types map[string]ProviderMetadata
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		types: make(map[string]ProviderMetadata),
	}
}

// Register registers a new provider type.
// Returns an error if a provider with the same name is already registered.
func (r *Registry) Register(meta ProviderMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if meta.Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if meta.Factory == nil {
		return fmt.Errorf("provider %q: factory cannot be nil", meta.Name)
	}

	if _, exists := r.types[meta.Name]; exists {
		return fmt.Errorf("provider %q already registered", meta.Name)
	}

	r.types[meta.Name] = meta
	return nil
}

// Unregister removes a provider type from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[name]; !exists {
		return fmt.Errorf("provider %q not registered", name)
	}

	delete(r.types, name)
	return nil
}

// Create creates a new provider instance by name.
// Returns an error if the provider type is not registered.
func (r *Registry) Create(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, exists := r.types[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not registered", name)
	}

	return meta.Factory(), nil
}

// Get retrieves metadata for a registered provider type.
// Returns nil if the provider is not registered.
func (r *Registry) Get(name string) *ProviderMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, exists := r.types[name]
	if !exists {
		return nil
	}
	return &meta
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	return names
}

// ListMetadata returns metadata for all registered providers.
func (r *Registry) ListMetadata() []ProviderMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metas := make([]ProviderMetadata, 0, len(r.types))
	for _, meta := range r.types {
		metas = append(metas, meta)
	}
	return metas
}

// Exists checks if a provider type is registered.
func (r *Registry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.types[name]
	return exists
}

// ValidateConfig validates configuration against a provider's config schema.
func (r *Registry) ValidateConfig(providerName string, config map[string]any) error {
	meta := r.Get(providerName)
	if meta == nil {
		return fmt.Errorf("provider %q not registered", providerName)
	}

	// Check required fields
	for key, field := range meta.ConfigSchema {
		if field.Required {
			if _, exists := config[key]; !exists {
				return fmt.Errorf("provider %q: required config field %q is missing", providerName, key)
			}
		}
	}

	return nil
}

// Count returns the number of registered provider types.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.types)
}

// Clear removes all registered providers.
// Primarily used for testing.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.types = make(map[string]ProviderMetadata)
}
