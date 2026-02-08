package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourname/mifind/internal/types"
)

// Manager manages the lifecycle of multiple provider instances.
type Manager struct {
	mu        sync.RWMutex
	providers map[string]*ProviderInstance
	registry  *Registry
	logger    *zerolog.Logger
}

// ProviderInstance represents an active provider instance with its state.
type ProviderInstance struct {
	Provider        Provider
	Config          map[string]any
	Status          ProviderStatus
	lastDiscovery   time.Time
	discoveryMutex  sync.Mutex
}

// NewManager creates a new provider manager.
func NewManager(registry *Registry, logger *zerolog.Logger) *Manager {
	return &Manager{
		providers: make(map[string]*ProviderInstance),
		registry:  registry,
		logger:    logger,
	}
}

// Initialize initializes a provider from the registry with the given configuration.
// The provider instance is stored and managed by the manager.
func (m *Manager) Initialize(ctx context.Context, name string, config map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already initialized
	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider %q already initialized", name)
	}

	// Validate config against schema
	if err := m.registry.ValidateConfig(name, config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Create provider instance
	prov, err := m.registry.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Initialize the provider
	if err := prov.Initialize(ctx, config); err != nil {
		return fmt.Errorf("provider initialization failed: %w", err)
	}

	// Store the provider instance
	m.providers[name] = &ProviderInstance{
		Provider: prov,
		Config:   config,
		Status: ProviderStatus{
			Name:                name,
			Connected:           true,
			EntityCount:         0,
			SupportsIncremental: prov.SupportsIncremental(),
		},
	}

	m.logger.Info().
		Str("provider", name).
		Msg("Provider initialized")

	return nil
}

// Shutdown shuts down a provider and removes it from the manager.
func (m *Manager) Shutdown(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, exists := m.providers[name]
	if !exists {
		return fmt.Errorf("provider %q not initialized", name)
	}

	// Shutdown the provider
	if err := inst.Provider.Shutdown(ctx); err != nil {
		m.logger.Error().
			Str("provider", name).
			Err(err).
			Msg("Provider shutdown failed")
		return err
	}

	delete(m.providers, name)

	m.logger.Info().
		Str("provider", name).
		Msg("Provider shut down")

	return nil
}

// ShutdownAll shuts down all managed providers.
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for name, inst := range m.providers {
		if err := inst.Provider.Shutdown(ctx); err != nil {
			m.logger.Error().
				Str("provider", name).
				Err(err).
				Msg("Provider shutdown failed")
			errs = append(errs, fmt.Errorf("%q: %w", name, err))
		}
	}

	m.providers = make(map[string]*ProviderInstance)

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// Get retrieves a managed provider instance by name.
func (m *Manager) Get(name string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.providers[name]
	if !exists {
		return nil, false
	}
	return inst.Provider, true
}

// List returns the names of all managed providers.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// Status returns the status of all managed providers.
func (m *Manager) Status() []ProviderStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ProviderStatus, 0, len(m.providers))
	for _, inst := range m.providers {
		status := inst.Status
		status.LastDiscovery = inst.lastDiscovery
		statuses = append(statuses, status)
	}
	return statuses
}

// GetStatus returns the status of a specific provider.
func (m *Manager) GetStatus(name string) (ProviderStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.providers[name]
	if !exists {
		return ProviderStatus{}, false
	}

	status := inst.Status
	status.LastDiscovery = inst.lastDiscovery
	return status, true
}

// DiscoverAll runs discovery on all managed providers concurrently.
// Returns all discovered entities aggregated together.
func (m *Manager) DiscoverAll(ctx context.Context) ([]types.Entity, error) {
	m.mu.RLock()
	providerNames := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providerNames = append(providerNames, name)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(chan []types.Entity, len(providerNames))
	errors := make(chan error, len(providerNames))

	for _, name := range providerNames {
		wg.Add(1)
		go func(providerName string) {
			defer wg.Done()

			entities, err := m.Discover(ctx, providerName)
			if err != nil {
				errors <- fmt.Errorf("%q: %w", providerName, err)
				return
			}
			results <- entities
		}(name)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Collect results
	var allEntities []types.Entity
	for entities := range results {
		allEntities = append(allEntities, entities...)
	}

	// Check for errors (non-fatal, some providers may fail)
	var errs []error
	for err := range errors {
		errs = append(errs, err)
		m.logger.Warn().Err(err).Msg("Provider discovery failed")
	}

	return allEntities, nil
}

// Discover runs discovery on a specific provider.
func (m *Manager) Discover(ctx context.Context, name string) ([]types.Entity, error) {
	m.mu.RLock()
	inst, exists := m.providers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider %q not initialized", name)
	}

	inst.discoveryMutex.Lock()
	defer inst.discoveryMutex.Unlock()

	m.logger.Info().
		Str("provider", name).
		Msg("Starting discovery")

	start := time.Now()
	entities, err := inst.Provider.Discover(ctx)
	duration := time.Since(start)

	if err != nil {
		// Update status with error
		m.mu.Lock()
		inst.Status.LastError = err.Error()
		inst.Status.Connected = false
		m.providers[name] = inst
		m.mu.Unlock()

		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Update status
	m.mu.Lock()
	inst.lastDiscovery = time.Now()
	inst.Status.EntityCount = len(entities)
	inst.Status.LastError = ""
	inst.Status.Connected = true
	m.providers[name] = inst
	m.mu.Unlock()

	m.logger.Info().
		Str("provider", name).
		Int("count", len(entities)).
		Dur("duration", duration).
		Msg("Discovery completed")

	return entities, nil
}

// DiscoverSince runs incremental discovery on a specific provider.
// Returns an error if the provider doesn't support incremental updates.
func (m *Manager) DiscoverSince(ctx context.Context, name string, since time.Time) ([]types.Entity, error) {
	m.mu.RLock()
	inst, exists := m.providers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider %q not initialized", name)
	}

	if !inst.Provider.SupportsIncremental() {
		return nil, fmt.Errorf("provider %q does not support incremental discovery", name)
	}

	inst.discoveryMutex.Lock()
	defer inst.discoveryMutex.Unlock()

	m.logger.Info().
		Str("provider", name).
		Time("since", since).
		Msg("Starting incremental discovery")

	entities, err := inst.Provider.DiscoverSince(ctx, since)
	if err != nil {
		// Update status with error
		m.mu.Lock()
		inst.Status.LastError = err.Error()
		inst.Status.Connected = false
		m.providers[name] = inst
		m.mu.Unlock()

		return nil, fmt.Errorf("incremental discovery failed: %w", err)
	}

	// Update status
	m.mu.Lock()
	inst.lastDiscovery = time.Now()
	inst.Status.EntityCount += len(entities)
	inst.Status.LastError = ""
	inst.Status.Connected = true
	m.providers[name] = inst
	m.mu.Unlock()

	m.logger.Info().
		Str("provider", name).
		Int("new_count", len(entities)).
		Msg("Incremental discovery completed")

	return entities, nil
}

// SearchAll runs a search query across all managed providers concurrently.
func (m *Manager) SearchAll(ctx context.Context, query SearchQuery) map[string][]types.Entity {
	m.mu.RLock()
	providerList := make(map[string]Provider)
	for name, inst := range m.providers {
		if inst.Status.Connected {
			providerList[name] = inst.Provider
		}
	}
	m.mu.RUnlock()

	results := make(map[string][]types.Entity)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for name, prov := range providerList {
		wg.Add(1)
		go func(providerName string, provider Provider) {
			defer wg.Done()

			entities, err := provider.Search(ctx, query)
			if err != nil {
				m.logger.Warn().
					Str("provider", providerName).
					Err(err).
					Msg("Provider search failed")
				return
			}

			mu.Lock()
			results[providerName] = entities
			mu.Unlock()
		}(name, prov)
	}

	wg.Wait()
	return results
}

// Hydrate retrieves full entity details by ID from the appropriate provider.
// The provider name is inferred from the entity ID prefix or looked up.
func (m *Manager) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	// Parse provider from ID (format: "provider:rest-of-id")
	// For now, we'll iterate through providers
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, inst := range m.providers {
		if !inst.Status.Connected {
			continue
		}

		entity, err := inst.Provider.Hydrate(ctx, id)
		if err == nil {
			return entity, nil
		}
		if err != ErrNotFound {
			m.logger.Warn().
				Str("provider", name).
				Str("id", id).
				Err(err).
				Msg("Provider hydrate failed")
		}
	}

	return types.Entity{}, ErrNotFound
}

// GetRelated retrieves related entities from the appropriate provider.
func (m *Manager) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	// Find the provider that owns this entity
	// For now, iterate through providers
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, inst := range m.providers {
		if !inst.Status.Connected {
			continue
		}

		related, err := inst.Provider.GetRelated(ctx, id, relType)
		if err == nil {
			return related, nil
		}
		if err != ErrNotFound {
			m.logger.Warn().
				Str("provider", name).
				Str("id", id).
				Err(err).
				Msg("Provider getRelated failed")
		}
	}

	return nil, ErrNotFound
}

// Count returns the number of managed providers.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.providers)
}

// IsConnected checks if a specific provider is connected.
func (m *Manager) IsConnected(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.providers[name]
	return exists && inst.Status.Connected
}
