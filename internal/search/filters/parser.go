package filters

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// Parser converts HTTP request filter data into typed FilterValue objects.
// It handles the frontend's filter format and validates against attribute definitions.
type Parser struct {
	registry *types.TypeRegistry
}

// NewParser creates a new filter parser.
func NewParser(registry *types.TypeRegistry) *Parser {
	return &Parser{
		registry: registry,
	}
}

// ParseFilters parses filter data from an HTTP request into typed FilterValue objects.
// The input format is a map of attribute names to filter specifications:
//
//	{
//	  "extension": {"eq": "jpg"},
//	  "size": {"gte": 1000, "lte": 100000},
//	  "person": {"in": ["id1", "id2"]},
//	  "created": {"min": 1234567890, "max": 1234567899}
//	}
//
// Returns a map of attribute names to FilterValue objects, or a MultiValidationError
// if any filters fail to parse or validate.
func (p *Parser) ParseFilters(filterData map[string]any) (map[string]FilterValue, error) {
	if len(filterData) == 0 {
		return nil, nil
	}

	// Get all attribute definitions for validation
	allAttrs := p.registry.GetAllAttributes()

	result := make(map[string]FilterValue)
	var validationErrors *MultiValidationError

	for attrName, filterSpec := range filterData {
		// Look up attribute definition
		attrDef, exists := allAttrs[attrName]
		if !exists {
			validationErrors = p.addError(validationErrors, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("unknown attribute: %q", attrName),
			})
			continue
		}

		// Check if attribute is filterable
		if !attrDef.Filterable {
			validationErrors = p.addError(validationErrors, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("attribute %q is not filterable", attrName),
			})
			continue
		}

		// Parse the filter specification based on its format
		filterValue, err := p.parseFilterSpec(attrName, filterSpec, attrDef)
		if err != nil {
			validationErrors = p.addError(validationErrors, err)
			continue
		}

		// Validate the filter against the attribute definition
		if err := filterValue.Validate(attrDef); err != nil {
			validationErrors = p.addError(validationErrors, err)
			continue
		}

		result[attrName] = filterValue
	}

	if validationErrors != nil && validationErrors.HasErrors() {
		return result, validationErrors
	}

	return result, nil
}

// parseFilterSpec parses a single filter specification into a FilterValue.
// The filter specification can be in several formats:
//
// 1. Simple value (string, number, bool): treated as implicit "eq" operation
//    "value" -> StringFilter with eq
// 2. Simple operation: {"eq": "value"} -> StringFilter
// 3. Range: {"gte": 100, "lte": 1000} -> IntFilter (will be combined by caller)
// 4. Min/max range: {"min": 100, "max": 1000} -> RangeFilter
// 5. Array membership: {"in": ["a", "b"]} -> StringSliceFilter
func (p *Parser) parseFilterSpec(attrName string, filterSpec any, attrDef types.AttributeDef) (FilterValue, error) {
	// Handle simple (non-object) values as implicit "eq" operations
	if _, isMap := filterSpec.(map[string]any); !isMap {
		// Simple value - treat as implicit equality filter
		return p.parseOperationFilter(attrName, OpEq, filterSpec, attrDef)
	}

	// Convert to map if it's JSON
	specMap, ok := filterSpec.(map[string]any)
	if !ok {
		// Try to unmarshal as JSON (in case it came from JSON decoder)
		jsonBytes, err := json.Marshal(filterSpec)
		if err != nil {
			return nil, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("invalid filter format: not an object"),
			}
		}
		if err := json.Unmarshal(jsonBytes, &specMap); err != nil {
			return nil, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("invalid filter format: %v", err),
			}
		}
	}

	// Check for range format (min/max keys) - takes precedence
	if _, hasMin := specMap["min"]; hasMin {
		return p.parseRangeFilter(attrName, specMap, attrDef)
	}

	// Check for multiple comparison operations (e.g., both gte and lte)
	// In this case, we return the first one and let the caller handle combining
	for opKey, opValue := range specMap {
		op := FilterOperation(opKey)
		if !isValidOperation(op) {
			continue
		}

		// Parse based on attribute type and operation
		return p.parseOperationFilter(attrName, op, opValue, attrDef)
	}

	// Check for "in" operation (array membership)
	if inValue, hasIn := specMap["in"]; hasIn {
		return p.parseInFilter(attrName, inValue, attrDef)
	}

	return nil, &ValidationError{
		FilterName: attrName,
		Reason:     "no valid filter operation found (expected eq, neq, gt, gte, lt, lte, contains, in, min, or max)",
	}
}

// parseRangeFilter parses a range filter with min/max keys.
func (p *Parser) parseRangeFilter(attrName string, specMap map[string]any, attrDef types.AttributeDef) (FilterValue, error) {
	minValue, hasMin := specMap["min"]
	maxValue, hasMax := specMap["max"]

	if !hasMin && !hasMax {
		return nil, &ValidationError{
			FilterName: attrName,
			Reason:     "range filter must have min or max key",
		}
	}

	// Handle time attributes
	if attrDef.Type == types.AttributeTypeTime {
		return p.parseDateRangeFilter(attrName, minValue, maxValue)
	}

	// Handle numeric attributes
	var minPtr, maxPtr *float64

	if hasMin {
		min, err := parseFloat64(attrName, "min", minValue)
		if err != nil {
			return nil, err
		}
		minPtr = &min
	}

	if hasMax {
		max, err := parseFloat64(attrName, "max", maxValue)
		if err != nil {
			return nil, err
		}
		maxPtr = &max
	}

	return NewRangeFilter(minPtr, maxPtr), nil
}

// parseDateRangeFilter parses a date range filter.
func (p *Parser) parseDateRangeFilter(attrName string, minValue, maxValue any) (FilterValue, error) {
	var minPtr, maxPtr *time.Time

	if minValue != nil {
		min, err := parseTime(attrName, "min", minValue)
		if err != nil {
			return nil, err
		}
		minPtr = &min
	}

	if maxValue != nil {
		max, err := parseTime(attrName, "max", maxValue)
		if err != nil {
			return nil, err
		}
		maxPtr = &max
	}

	return NewDateRangeFilter(minPtr, maxPtr), nil
}

// parseOperationFilter parses a single operation filter (eq, gt, contains, etc.)
func (p *Parser) parseOperationFilter(attrName string, op FilterOperation, value any, attrDef types.AttributeDef) (FilterValue, error) {
	switch attrDef.Type {
	case types.AttributeTypeString:
		strVal, ok := value.(string)
		if !ok {
			return nil, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("expected string value for %s operation, got %T", op, value),
				Operation:  op,
			}
		}
		return NewStringFilter(op, strVal), nil

	case types.AttributeTypeInt, types.AttributeTypeInt64:
		intVal, err := parseInt64(attrName, string(op), value)
		if err != nil {
			return nil, err
		}
		return NewIntFilter(op, intVal), nil

	case types.AttributeTypeFloat, types.AttributeTypeFloat64:
		floatVal, err := parseFloat64(attrName, string(op), value)
		if err != nil {
			return nil, err
		}
		return NewFloatFilter(op, floatVal), nil

	case types.AttributeTypeBool:
		boolVal, ok := value.(bool)
		if !ok {
			return nil, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("expected bool value for %s operation, got %T", op, value),
				Operation:  op,
			}
		}
		return NewBoolFilter(op, boolVal), nil

	case types.AttributeTypeTime:
		timeVal, err := parseTime(attrName, string(op), value)
		if err != nil {
			return nil, err
		}
		return NewTimeFilter(op, timeVal), nil

	case types.AttributeTypeStringSlice:
		// String slice with "eq" or "neq" - treat as array membership
		if op == OpEq || op == OpNeq {
			return p.parseInFilter(attrName, value, attrDef)
		}
		// For other operations, parse as string slice
		sliceVal, err := parseStringSlice(attrName, value)
		if err != nil {
			return nil, err
		}
		return NewStringSliceFilter(op, sliceVal), nil

	default:
		return nil, &ValidationError{
			FilterName: attrName,
			Reason:     fmt.Sprintf("unsupported attribute type for filtering: %s", attrDef.Type),
		}
	}
}

// parseInFilter parses an "in" filter (array membership).
// Handles both array values and single string values (which are converted to single-element arrays).
func (p *Parser) parseInFilter(attrName string, value any, attrDef types.AttributeDef) (FilterValue, error) {
	// Handle single string value by converting to single-element array
	if strVal, ok := value.(string); ok {
		return NewStringSliceFilter(OpIn, []string{strVal}), nil
	}

	sliceVal, err := parseStringSlice(attrName, value)
	if err != nil {
		return nil, err
	}
	return NewStringSliceFilter(OpIn, sliceVal), nil
}

// Helper functions for parsing specific types

func parseInt64(attrName, op string, value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("invalid int value for %s: %v", op, err),
			}
		}
		return i, nil
	default:
		return 0, &ValidationError{
			FilterName: attrName,
			Reason:     fmt.Sprintf("expected numeric value for %s, got %T", op, value),
		}
	}
}

func parseFloat64(attrName, op string, value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("invalid float value for %s: %v", op, err),
			}
		}
		return f, nil
	default:
		return 0, &ValidationError{
			FilterName: attrName,
			Reason:     fmt.Sprintf("expected numeric value for %s, got %T", op, value),
		}
	}
}

func parseTime(attrName, op string, value any) (time.Time, error) {
	var timestamp int64

	switch v := value.(type) {
	case float64:
		timestamp = int64(v)
	case int:
		timestamp = int64(v)
	case int64:
		timestamp = v
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return time.Time{}, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("invalid time value for %s: %v", op, err),
			}
		}
		timestamp = i
	default:
		return time.Time{}, &ValidationError{
			FilterName: attrName,
			Reason:     fmt.Sprintf("expected numeric timestamp for %s, got %T", op, value),
		}
	}

	return time.Unix(timestamp, 0), nil
}

func parseStringSlice(attrName string, value any) ([]string, error) {
	slice, ok := value.([]any)
	if !ok {
		return nil, &ValidationError{
			FilterName: attrName,
			Reason:     fmt.Sprintf("expected array for in filter, got %T", value),
		}
	}

	result := make([]string, 0, len(slice))
	for i, item := range slice {
		str, ok := item.(string)
		if !ok {
			return nil, &ValidationError{
				FilterName: attrName,
				Reason:     fmt.Sprintf("expected string at index %d, got %T", i, item),
			}
		}
		result = append(result, str)
	}

	if len(result) == 0 {
		return nil, &ValidationError{
			FilterName: attrName,
			Reason:     "in filter array cannot be empty",
		}
	}

	return result, nil
}

func isValidOperation(op FilterOperation) bool {
	switch op {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpContains, OpIn:
		return true
	default:
		return false
	}
}

func (p *Parser) addError(errors *MultiValidationError, err error) *MultiValidationError {
	if errors == nil {
		return NewMultiValidationError([]error{err})
	}
	errors.AddError(err)
	return errors
}
