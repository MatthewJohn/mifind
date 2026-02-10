package search

import (
	"fmt"

	"github.com/yourname/mifind/internal/types"
)

// Filters handles dynamic filter extraction and application.
type Filters struct {
	// TypeRegistry is used to get valid filters for each type
	TypeRegistry *types.TypeRegistry
}

// NewFilters creates a new filter handler.
func NewFilters(registry *types.TypeRegistry) *Filters {
	return &Filters{
		TypeRegistry: registry,
	}
}

// FilterDefinition defines a dynamic filter with counts.
type FilterDefinition struct {
	// Name is the filter name (attribute key)
	Name string

	// Type is the filter type
	Type types.FilterType

	// Label is a human-readable label
	Label string

	// Options are the available filter values with counts
	Options []FilterOption

	// Min is the minimum value (for range filters)
	Min *float64

	// Max is the maximum value (for range filters)
	Max *float64
}

// FilterOption represents a single filter value option.
type FilterOption struct {
	// Value is the filter value
	Value string `json:"value"`

	// Label is the human-readable label
	Label string `json:"label"`

	// Count is the number of items with this value
	Count int `json:"count"`

	// Selected indicates if this option is currently selected
	Selected bool `json:"selected"`

	// HasMore indicates if more entities exist with this value than shown in Count.
	// Used for provider-based filters to indicate partial results (e.g., "23+").
	HasMore bool `json:"has_more"`
}

// FilterResult contains available filters for a result set.
type FilterResult struct {
	// Filters is the list of available filters
	Filters []FilterDefinition

	// TypeCounts is the count of items per type
	TypeCounts map[string]int

	// TotalCount is the total number of items
	TotalCount int
}

// ExtractFilters extracts available filters from a set of entities.
func (f *Filters) ExtractFilters(entities []types.Entity, typeName string) FilterResult {
	// Get type definition for filter schema
	var typeDef *types.TypeDefinition
	if typeName != "" {
		typeDef = f.TypeRegistry.Get(typeName)
	}

	// Count by type
	typeCounts := make(map[string]int)
	for _, entity := range entities {
		typeCounts[entity.Type]++
	}

	// Extract attribute values for filters
	attrFilters := f.extractAttributeFilters(entities, typeDef)

	return FilterResult{
		Filters:    attrFilters,
		TypeCounts: typeCounts,
		TotalCount: len(entities),
	}
}

// extractAttributeFilters extracts filters from entity attributes.
// typeDef is currently unused but reserved for future type-based filtering.
func (f *Filters) extractAttributeFilters(entities []types.Entity, _ *types.TypeDefinition) []FilterDefinition {
	// Track unique values per attribute
	attrValues := make(map[string]map[string]int)
	attrTypes := make(map[string]types.AttributeType)

	// First pass: collect all unique values and types
	for _, entity := range entities {
		for key, value := range entity.Attributes {
			if attrValues[key] == nil {
				attrValues[key] = make(map[string]int)
			}

			// Determine attribute type
			attrType := f.inferAttributeType(value)
			if existingType, ok := attrTypes[key]; ok {
				// Use the most general type if we've seen this attribute before
				attrTypes[key] = generalizeType(existingType, attrType)
			} else {
				attrTypes[key] = attrType
			}

			// Convert value to string for counting
			valueStr := attributeValueToString(value)
			attrValues[key][valueStr]++
		}
	}

	// Build filter definitions
	var filters []FilterDefinition

	for key, values := range attrValues {
		attrType := attrTypes[key]

		// Determine filter type based on attribute type
		var filterType types.FilterType
		switch attrType {
		case types.AttributeTypeInt, types.AttributeTypeInt64, types.AttributeTypeFloat, types.AttributeTypeFloat64:
			filterType = types.FilterTypeRange
		case types.AttributeTypeBool:
			filterType = types.FilterTypeBool
		case types.AttributeTypeTime:
			filterType = types.FilterTypeDateRange
		case types.AttributeTypeGPS:
			filterType = types.FilterTypeGPS
		case types.AttributeTypeStringSlice:
			filterType = types.FilterTypeMulti
		default:
			// For string, check if it looks like an enum (few unique values)
			if len(values) <= 20 {
				filterType = types.FilterTypeSelect
			} else {
				filterType = types.FilterTypeText
			}
		}

		filterDef := FilterDefinition{
			Name:  key,
			Type:  filterType,
			Label: formatLabel(key),
		}

		// Add options for select/multi filters
		if filterType == types.FilterTypeSelect || filterType == types.FilterTypeMulti {
			for value, count := range values {
				filterDef.Options = append(filterDef.Options, FilterOption{
					Value: value,
					Label: value,
					Count: count,
				})
			}
		}

		// Add min/max for range filters
		if filterType == types.FilterTypeRange {
			minVal, maxVal := f.getMinMax(values)
			filterDef.Min = &minVal
			filterDef.Max = &maxVal
		}

		filters = append(filters, filterDef)
	}

	return filters
}

// inferAttributeType infers the attribute type from a value.
func (f *Filters) inferAttributeType(value any) types.AttributeType {
	switch value.(type) {
	case string:
		return types.AttributeTypeString
	case int, int32, int64:
		return types.AttributeTypeInt64
	case float32, float64:
		return types.AttributeTypeFloat64
	case bool:
		return types.AttributeTypeBool
	case []string:
		return types.AttributeTypeStringSlice
	case types.GPS:
		return types.AttributeTypeGPS
	default:
		// Try to infer from common interfaces
		return types.AttributeTypeString
	}
}

// generalizeType returns the most general type when two types are encountered.
func generalizeType(t1, t2 types.AttributeType) types.AttributeType {
	if t1 == t2 {
		return t1
	}

	// Numeric types generalize to float64
	if (t1 == types.AttributeTypeInt || t1 == types.AttributeTypeInt64) &&
		(t2 == types.AttributeTypeInt || t2 == types.AttributeTypeInt64) {
		return types.AttributeTypeInt64
	}

	if (t1 == types.AttributeTypeFloat || t1 == types.AttributeTypeFloat64 ||
		t1 == types.AttributeTypeInt || t1 == types.AttributeTypeInt64) &&
		(t2 == types.AttributeTypeFloat || t2 == types.AttributeTypeFloat64 ||
			t2 == types.AttributeTypeInt || t2 == types.AttributeTypeInt64) {
		return types.AttributeTypeFloat64
	}

	// Default to string for mixed types
	return types.AttributeTypeString
}

// ApplyFilters filters entities based on the provided filter criteria.
func (f *Filters) ApplyFilters(entities []types.Entity, filters map[string]any) []types.Entity {
	if len(filters) == 0 {
		return entities
	}

	var result []types.Entity

	for _, entity := range entities {
		if f.matchesFilters(entity, filters) {
			result = append(result, entity)
		}
	}

	return result
}

// matchesFilters checks if an entity matches all filter criteria.
func (f *Filters) matchesFilters(entity types.Entity, filters map[string]any) bool {
	for key, filterValue := range filters {
		entityValue, exists := entity.Attributes[key]
		if !exists {
			return false
		}

		if !f.matchValue(entityValue, filterValue) {
			return false
		}
	}
	return true
}

// matchValue checks if an entity value matches a filter value.
func (f *Filters) matchValue(entityValue, filterValue any) bool {
	// Handle slice filters (e.g., for multi-select)
	if filterSlice, ok := filterValue.([]any); ok {
		for _, fv := range filterSlice {
			if f.matchSingleValue(entityValue, fv) {
				return true
			}
		}
		return false
	}

	return f.matchSingleValue(entityValue, filterValue)
}

// matchSingleValue checks if a single entity value matches a single filter value.
func (f *Filters) matchSingleValue(entityValue, filterValue any) bool {
	// Try string comparison
	entityStr := attributeValueToString(entityValue)
	filterStr := attributeValueToString(filterValue)
	return entityStr == filterStr
}

// getMinMax calculates min and max values from a map of string values.
func (f *Filters) getMinMax(values map[string]int) (min, max float64) {
	first := true
	for valStr := range values {
		val := parseNumeric(valStr)
		if first {
			min = val
			max = val
			first = false
		} else {
			if val < min {
				min = val
			}
			if val > max {
				max = val
			}
		}
	}
	return min, max
}

// parseNumeric parses a numeric value from a string.
func parseNumeric(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// attributeValueToString converts an attribute value to string representation.
func attributeValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case []string:
		if len(v) > 0 {
			return v[0]
		}
		return ""
	case types.GPS:
		return fmt.Sprintf("%f,%f", v.Latitude, v.Longitude)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatLabel converts a key to a human-readable label.
func formatLabel(key string) string {
	// Convert snake_case or camelCase to Title Case
	// This is a simple implementation; could be improved
	return key
}
