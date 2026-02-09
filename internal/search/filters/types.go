package filters

import (
	"fmt"
	"time"

	"github.com/yourname/mifind/internal/types"
)

// FilterOperation represents a comparison operation for filters.
type FilterOperation string

const (
	OpEq       FilterOperation = "eq"       // Equality
	OpNeq      FilterOperation = "neq"      // Inequality
	OpGt       FilterOperation = "gt"       // Greater than
	OpGte      FilterOperation = "gte"      // Greater than or equal
	OpLt       FilterOperation = "lt"       // Less than
	OpLte      FilterOperation = "lte"      // Less than or equal
	OpContains FilterOperation = "contains" // Substring match
	OpIn       FilterOperation = "in"       // In array
)

// FilterValue is the interface for all typed filter values.
// Each filter type implements this interface to provide type-safe operations.
type FilterValue interface {
	// Operation returns the filter operation (eq, gt, contains, etc.)
	Operation() FilterOperation

	// Validate checks if the filter value is valid for the given attribute definition.
	// Returns an error if the value is invalid or the operation is not supported.
	Validate(attrDef types.AttributeDef) error

	// Value returns the underlying value for provider consumption.
	// This provides backward compatibility with providers that use map[string]any.
	Value() any
}

// StringFilter represents a filter on string attributes.
type StringFilter struct {
	Op    FilterOperation
	value string
}

// NewStringFilter creates a new StringFilter.
func NewStringFilter(op FilterOperation, value string) *StringFilter {
	return &StringFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *StringFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the string filter is valid.
func (f *StringFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is string
	if attrDef.Type != types.AttributeTypeString {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected %s, got %s", types.AttributeTypeString, attrDef.Type),
			Operation:    f.Op,
			ExpectedType: types.AttributeTypeString,
			ActualType:   attrDef.Type,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	// Validate value is not empty
	if f.value == "" {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "string value cannot be empty",
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the string value.
func (f *StringFilter) Value() any {
	return f.value
}

// IntFilter represents a filter on integer attributes.
type IntFilter struct {
	Op    FilterOperation
	value int64
}

// NewIntFilter creates a new IntFilter.
func NewIntFilter(op FilterOperation, value int64) *IntFilter {
	return &IntFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *IntFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the int filter is valid.
func (f *IntFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is int or int64
	if attrDef.Type != types.AttributeTypeInt && attrDef.Type != types.AttributeTypeInt64 {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected int or int64, got %s", attrDef.Type),
			Operation:    f.Op,
			ExpectedType: attrDef.Type,
			ActualType:   attrDef.Type,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the int value.
func (f *IntFilter) Value() any {
	return f.value
}

// FloatFilter represents a filter on float attributes.
type FloatFilter struct {
	Op    FilterOperation
	value float64
}

// NewFloatFilter creates a new FloatFilter.
func NewFloatFilter(op FilterOperation, value float64) *FloatFilter {
	return &FloatFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *FloatFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the float filter is valid.
func (f *FloatFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is float or float64
	if attrDef.Type != types.AttributeTypeFloat && attrDef.Type != types.AttributeTypeFloat64 {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected float or float64, got %s", attrDef.Type),
			Operation:    f.Op,
			ExpectedType: attrDef.Type,
			ActualType:   attrDef.Type,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the float value.
func (f *FloatFilter) Value() any {
	return f.value
}

// BoolFilter represents a filter on boolean attributes.
type BoolFilter struct {
	Op    FilterOperation
	value bool
}

// NewBoolFilter creates a new BoolFilter.
func NewBoolFilter(op FilterOperation, value bool) *BoolFilter {
	return &BoolFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *BoolFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the bool filter is valid.
func (f *BoolFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is bool
	if attrDef.Type != types.AttributeTypeBool {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected bool, got %s", attrDef.Type),
			Operation:    f.Op,
			ExpectedType: types.AttributeTypeBool,
			ActualType:   attrDef.Type,
		}
	}

	// Bool only supports eq and neq
	if f.Op != OpEq && f.Op != OpNeq {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     fmt.Sprintf("bool filters only support eq and neq operations, got %s", f.Op),
			Operation:  f.Op,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the bool value.
func (f *BoolFilter) Value() any {
	return f.value
}

// TimeFilter represents a filter on time attributes.
type TimeFilter struct {
	Op    FilterOperation
	value time.Time
}

// NewTimeFilter creates a new TimeFilter.
func NewTimeFilter(op FilterOperation, value time.Time) *TimeFilter {
	return &TimeFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *TimeFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the time filter is valid.
func (f *TimeFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is time
	if attrDef.Type != types.AttributeTypeTime {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected time, got %s", attrDef.Type),
			Operation:    f.Op,
			ExpectedType: types.AttributeTypeTime,
			ActualType:   attrDef.Type,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	// Validate time is not zero
	if f.value.IsZero() {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "time value cannot be zero",
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the time as Unix timestamp.
func (f *TimeFilter) Value() any {
	return f.value.Unix()
}

// StringSliceFilter represents a filter on string slice attributes (e.g., labels, tags).
type StringSliceFilter struct {
	Op    FilterOperation
	value []string
}

// NewStringSliceFilter creates a new StringSliceFilter.
func NewStringSliceFilter(op FilterOperation, value []string) *StringSliceFilter {
	return &StringSliceFilter{Op: op, value: value}
}

// Operation returns the filter operation.
func (f *StringSliceFilter) Operation() FilterOperation {
	return f.Op
}

// Validate checks if the string slice filter is valid.
func (f *StringSliceFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is []string
	if attrDef.Type != types.AttributeTypeStringSlice {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: expected []string, got %s", attrDef.Type),
			Operation:    f.Op,
			ExpectedType: types.AttributeTypeStringSlice,
			ActualType:   attrDef.Type,
		}
	}

	// StringSliceFilter only supports eq (contains all) and in (contains any)
	if f.Op != OpEq && f.Op != OpIn {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     fmt.Sprintf("[]string filters only support eq and in operations, got %s", f.Op),
			Operation:  f.Op,
		}
	}

	// Validate operation is supported
	if err := validateOperation(f.Op, attrDef.Filter); err != nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     err.Error(),
			Operation:  f.Op,
		}
	}

	// Validate non-empty
	if len(f.value) == 0 {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "string slice value cannot be empty",
			Operation:  f.Op,
		}
	}

	return nil
}

// Value returns the string slice.
func (f *StringSliceFilter) Value() any {
	return f.value
}

// RangeFilter represents a range filter on numeric attributes.
type RangeFilter struct {
	Min *float64 // Minimum value (nil = no minimum)
	Max *float64 // Maximum value (nil = no maximum)
}

// NewRangeFilter creates a new RangeFilter.
func NewRangeFilter(min, max *float64) *RangeFilter {
	return &RangeFilter{Min: min, Max: max}
}

// Operation returns the filter operation (always "range" for range filters).
func (f *RangeFilter) Operation() FilterOperation {
	return "range"
}

// Validate checks if the range filter is valid.
func (f *RangeFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is numeric
	if attrDef.Type != types.AttributeTypeInt && attrDef.Type != types.AttributeTypeInt64 &&
		attrDef.Type != types.AttributeTypeFloat && attrDef.Type != types.AttributeTypeFloat64 {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: range filters require numeric type, got %s", attrDef.Type),
			Operation:    f.Operation(),
			ExpectedType: attrDef.Type,
			ActualType:   attrDef.Type,
		}
	}

	// Check that range filtering is supported
	if !attrDef.Filter.SupportsRange {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "range filtering is not supported for this attribute",
			Operation:  f.Operation(),
		}
	}

	// Validate at least one bound is set
	if f.Min == nil && f.Max == nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "range filter must have at least min or max bound",
			Operation:  f.Operation(),
		}
	}

	// Validate min <= max
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "range min cannot be greater than max",
			Operation:  f.Operation(),
		}
	}

	return nil
}

// Value returns the range as a map with min/max keys.
func (f *RangeFilter) Value() any {
	result := make(map[string]any)
	if f.Min != nil {
		result["min"] = *f.Min
	}
	if f.Max != nil {
		result["max"] = *f.Max
	}
	return result
}

// DateRangeFilter represents a date range filter on time attributes.
type DateRangeFilter struct {
	Min *time.Time // Minimum time (nil = no minimum)
	Max *time.Time // Maximum time (nil = no maximum)
}

// NewDateRangeFilter creates a new DateRangeFilter.
func NewDateRangeFilter(min, max *time.Time) *DateRangeFilter {
	return &DateRangeFilter{Min: min, Max: max}
}

// Operation returns the filter operation (always "date-range" for date range filters).
func (f *DateRangeFilter) Operation() FilterOperation {
	return "date-range"
}

// Validate checks if the date range filter is valid.
func (f *DateRangeFilter) Validate(attrDef types.AttributeDef) error {
	// Check attribute type is time
	if attrDef.Type != types.AttributeTypeTime {
		return &ValidationError{
			FilterName:   attrDef.Name,
			Reason:       fmt.Sprintf("invalid type: date range filters require time type, got %s", attrDef.Type),
			Operation:    f.Operation(),
			ExpectedType: types.AttributeTypeTime,
			ActualType:   attrDef.Type,
		}
	}

	// Check that range filtering is supported
	if !attrDef.Filter.SupportsRange {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "range filtering is not supported for this attribute",
			Operation:  f.Operation(),
		}
	}

	// Validate at least one bound is set
	if f.Min == nil && f.Max == nil {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "date range filter must have at least min or max bound",
			Operation:  f.Operation(),
		}
	}

	// Validate min <= max
	if f.Min != nil && f.Max != nil && f.Min.After(*f.Max) {
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     "date range min cannot be after max",
			Operation:  f.Operation(),
		}
	}

	return nil
}

// Value returns the date range as a map with min/max keys (Unix timestamps).
func (f *DateRangeFilter) Value() any {
	result := make(map[string]any)
	if f.Min != nil {
		result["min"] = f.Min.Unix()
	}
	if f.Max != nil {
		result["max"] = f.Max.Unix()
	}
	return result
}

// validateOperation checks if the operation is supported by the attribute's filter config.
func validateOperation(op FilterOperation, filterConfig types.FilterConfig) error {
	switch op {
	case OpEq:
		if !filterConfig.SupportsEq {
			return fmt.Errorf("equality filtering is not supported for this attribute")
		}
	case OpNeq:
		if !filterConfig.SupportsNeq {
			return fmt.Errorf("inequality filtering is not supported for this attribute")
		}
	case OpGt, OpGte, OpLt, OpLte:
		// Range operations require SupportsRange
		if !filterConfig.SupportsRange {
			return fmt.Errorf("range filtering is not supported for this attribute")
		}
	case OpContains:
		if !filterConfig.SupportsContains {
			return fmt.Errorf("contains filtering is not supported for this attribute")
		}
	case OpIn:
		// "in" operation is for array membership - check if eq is supported as fallback
		// Many attributes support "in" even if they don't explicitly declare it
		if !filterConfig.SupportsEq && !filterConfig.SupportsRange {
			return fmt.Errorf("in filtering is not supported for this attribute")
		}
	default:
		return fmt.Errorf("unknown operation: %s", op)
	}
	return nil
}
