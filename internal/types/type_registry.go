package types

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// TypeRegistry manages the hierarchical type system for entities.
type TypeRegistry struct {
	mu    sync.RWMutex
	types map[string]*TypeDefinition
}

// TypeDefinition defines a type in the hierarchy with its attributes and filters.
type TypeDefinition struct {
	// Name is the type name (e.g., "file.media.video")
	Name string

	// Parent is the parent type name for inheritance (empty for root types)
	Parent string

	// Attributes defines valid attributes for this type
	Attributes map[string]AttributeDef

	// Filters defines available filters for this type
	Filters []FilterDefinition

	// Description is a human-readable description
	Description string
}

// UIConfig describes how to display an attribute in the UI.
// This enables generic frontend rendering without hardcoded attribute knowledge.
type UIConfig struct {
	// Widget specifies the UI widget type for this attribute
	// Values: "select", "multiselect", "input", "checkbox-group", "date-range", "range", "bool"
	Widget string

	// Icon is the icon name for frontend (e.g., lucide-react icon names)
	Icon string

	// Group is the display group for this attribute (e.g., "media", "file", "metadata", "core")
	Group string

	// Label is the display label (overrides Name)
	Label string

	// Priority is the display order (lower = first)
	Priority int
}

// FilterConfig describes how filtering works for an attribute.
// This enables generic filter handling without hardcoded attribute knowledge.
type FilterConfig struct {
	// SupportsEq indicates if equality filtering is supported
	SupportsEq bool

	// SupportsNeq indicates if inequality filtering is supported
	SupportsNeq bool

	// SupportsRange indicates if range filtering is supported
	SupportsRange bool

	// SupportsContains indicates if substring matching is supported
	SupportsContains bool

	// Cacheable indicates if filter values should be cached
	Cacheable bool

	// CacheTTL is the cache duration for filter values
	CacheTTL time.Duration
}

// AttributeDef defines an attribute's type and constraints.
type AttributeDef struct {
	// Name is the attribute key name
	Name string

	// Type is the attribute value type
	Type AttributeType

	// Required indicates if this attribute must be present
	Required bool

	// Filterable indicates if this attribute can be used for filtering
	Filterable bool

	// Description is a human-readable description
	Description string

	// AlwaysVisible indicates if this filter should always be shown (even without results)
	AlwaysVisible bool

	// UI describes how to display this attribute in the frontend
	UI UIConfig

	// Filter describes how filtering works for this attribute
	Filter FilterConfig
}

// AttributeType represents the type of an attribute value.
type AttributeType string

const (
	AttributeTypeString      AttributeType = "string"
	AttributeTypeInt         AttributeType = "int"
	AttributeTypeInt64       AttributeType = "int64"
	AttributeTypeFloat       AttributeType = "float"
	AttributeTypeFloat64     AttributeType = "float64"
	AttributeTypeBool        AttributeType = "bool"
	AttributeTypeTime        AttributeType = "time"
	AttributeTypeStringSlice AttributeType = "[]string"
	AttributeTypeGPS         AttributeType = "gps" // Special type for coordinates
)

// FilterDefinition defines a filter that can be applied to a type.
type FilterDefinition struct {
	// Name is the filter name (typically matches an attribute name)
	Name string

	// Type is the filter type
	Type FilterType

	// Label is a human-readable label for the filter
	Label string

	// Description describes what this filter does
	Description string

	// Options provides predefined choices for enum-like filters
	Options []FilterOption
}

// FilterType represents the type of filter.
type FilterType string

const (
	FilterTypeText      FilterType = "text"      // Text search input
	FilterTypeSelect    FilterType = "select"    // Single choice dropdown
	FilterTypeMulti     FilterType = "multi"     // Multi-select
	FilterTypeRange     FilterType = "range"     // Min/max range
	FilterTypeDateRange FilterType = "dateRange" // Date range picker
	FilterTypeBool      FilterType = "bool"      // Boolean toggle
	FilterTypeGPS       FilterType = "gps"       // GPS coordinates
)

// FilterOption represents a single option in a select/multi filter.
type FilterOption struct {
	// Value is the option value
	Value string

	// Label is the human-readable label
	Label string

	// Count is the number of items with this value (for faceted search)
	Count int
}

// NewTypeRegistry creates a new empty TypeRegistry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]*TypeDefinition),
	}
}

// Register registers a new type definition.
// Returns an error if the type already exists or parent doesn't exist.
func (r *TypeRegistry) Register(def TypeDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[def.Name]; exists {
		return fmt.Errorf("type %q already registered", def.Name)
	}

	// Validate parent exists (unless it's a root type)
	if def.Parent != "" {
		if _, exists := r.types[def.Parent]; !exists {
			return fmt.Errorf("parent type %q not found for type %q", def.Parent, def.Name)
		}
	}

	// Ensure attributes map is initialized
	if def.Attributes == nil {
		def.Attributes = make(map[string]AttributeDef)
	}

	r.types[def.Name] = &def
	return nil
}

// Get retrieves a type definition by name.
// Returns nil if the type doesn't exist.
func (r *TypeRegistry) Get(name string) *TypeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.types[name]
}

// GetParent returns the parent type definition, if any.
func (r *TypeRegistry) GetParent(name string) *TypeDefinition {
	def := r.Get(name)
	if def == nil || def.Parent == "" {
		return nil
	}
	return r.Get(def.Parent)
}

// GetAncestors returns all ancestor types from parent to root.
// The first element is the immediate parent, the last is the root.
func (r *TypeRegistry) GetAncestors(name string) []*TypeDefinition {
	var ancestors []*TypeDefinition
	current := r.GetParent(name)
	for current != nil {
		ancestors = append(ancestors, current)
		current = r.GetParent(current.Name)
	}
	return ancestors
}

// GetDescendants returns all descendant types (children, grandchildren, etc.).
func (r *TypeRegistry) GetDescendants(name string) []*TypeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var descendants []*TypeDefinition
	for _, def := range r.types {
		if def.Parent == name {
			descendants = append(descendants, def)
			// Recursively get children's descendants
			childDesc := r.getDescendantsLocked(def.Name)
			descendants = append(descendants, childDesc...)
		}
	}
	return descendants
}

func (r *TypeRegistry) getDescendantsLocked(name string) []*TypeDefinition {
	var descendants []*TypeDefinition
	for _, def := range r.types {
		if def.Parent == name {
			descendants = append(descendants, def)
			childDesc := r.getDescendantsLocked(def.Name)
			descendants = append(descendants, childDesc...)
		}
	}
	return descendants
}

// GetAll returns all registered type definitions.
func (r *TypeRegistry) GetAll() map[string]*TypeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid external modification
	result := make(map[string]*TypeDefinition, len(r.types))
	for k, v := range r.types {
		result[k] = v
	}
	return result
}

// GetAllAttributes returns all attribute definitions from all registered types.
// This is useful for building generic filter capabilities without hardcoded attribute names.
func (r *TypeRegistry) GetAllAttributes() map[string]AttributeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all unique attributes from all types
	attrs := make(map[string]AttributeDef)
	for _, typeDef := range r.types {
		for name, attr := range typeDef.Attributes {
			attrs[name] = attr
		}
	}
	return attrs
}

// List returns a slice of all type names.
func (r *TypeRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	return names
}

// IsTypeOf checks if a type is a descendant of the given parent type.
// Returns true if typeName equals parentName or is a descendant.
func (r *TypeRegistry) IsTypeOf(typeName, parentName string) bool {
	if typeName == parentName {
		return true
	}

	current := r.Get(typeName)
	for current != nil && current.Parent != "" {
		if current.Parent == parentName {
			return true
		}
		current = r.Get(current.Parent)
	}
	return false
}

// GetAttributes returns all attributes for a type, including inherited ones.
// Child attributes override parent attributes with the same name.
func (r *TypeRegistry) GetAttributes(typeName string) map[string]AttributeDef {
	result := make(map[string]AttributeDef)

	// Add ancestor attributes first (so children can override)
	ancestors := r.GetAncestors(typeName)
	for i := len(ancestors) - 1; i >= 0; i-- {
		for name, attr := range ancestors[i].Attributes {
			result[name] = attr
		}
	}

	// Add type's own attributes
	def := r.Get(typeName)
	if def != nil {
		for name, attr := range def.Attributes {
			result[name] = attr
		}
	}

	return result
}

// GetFilters returns all filters for a type, including inherited ones.
func (r *TypeRegistry) GetFilters(typeName string) []FilterDefinition {
	var filters []FilterDefinition

	// Add ancestor filters first
	ancestors := r.GetAncestors(typeName)
	for i := len(ancestors) - 1; i >= 0; i-- {
		filters = append(filters, ancestors[i].Filters...)
	}

	// Add type's own filters
	def := r.Get(typeName)
	if def != nil {
		filters = append(filters, def.Filters...)
	}

	return filters
}

// GetRootTypes returns all root types (types with no parent).
func (r *TypeRegistry) GetRootTypes() []*TypeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var roots []*TypeDefinition
	for _, def := range r.types {
		if def.Parent == "" {
			roots = append(roots, def)
		}
	}
	return roots
}

// ParseType parses a dotted type string and validates it exists.
// Returns the type name and any validation error.
func (r *TypeRegistry) ParseType(typeStr string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		return "", fmt.Errorf("type cannot be empty")
	}

	if _, exists := r.types[typeStr]; !exists {
		return "", fmt.Errorf("unknown type: %q", typeStr)
	}

	return typeStr, nil
}

// ValidateEntity checks if an entity's attributes match its type definition.
func (r *TypeRegistry) ValidateEntity(entity Entity) error {
	def := r.Get(entity.Type)
	if def == nil {
		return fmt.Errorf("unknown entity type: %q", entity.Type)
	}

	attrs := r.GetAttributes(entity.Type)

	// Check required attributes
	for name, attrDef := range attrs {
		if attrDef.Required {
			if _, exists := entity.Attributes[name]; !exists {
				return fmt.Errorf("required attribute %q missing for type %q", name, entity.Type)
			}
		}
	}

	return nil
}
