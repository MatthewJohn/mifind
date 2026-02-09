package filters

import (
	"fmt"

	"github.com/yourname/mifind/internal/types"
)

// ValidationError represents an error that occurs during filter validation.
// It provides detailed information about what went wrong.
type ValidationError struct {
	// FilterName is the name of the filter that failed validation
	FilterName string

	// Reason is a human-readable explanation of the validation failure
	Reason string

	// Operation is the filter operation that failed (eq, gt, contains, etc.)
	Operation FilterOperation

	// ExpectedType is the attribute type that was expected
	ExpectedType types.AttributeType

	// ActualType is the attribute type that was provided
	ActualType types.AttributeType
}

// Error returns a human-readable error message.
func (e *ValidationError) Error() string {
	if e.FilterName != "" {
		return fmt.Sprintf("filter %q: %s", e.FilterName, e.Reason)
	}
	return e.Reason
}

// IsTypeMismatch returns true if the error is due to a type mismatch.
func (e *ValidationError) IsTypeMismatch() bool {
	return e.ExpectedType != "" && e.ActualType != "" && e.ExpectedType != e.ActualType
}

// IsUnsupportedOperation returns true if the error is due to an unsupported operation.
func (e *ValidationError) IsUnsupportedOperation() bool {
	return e.Operation != ""
}

// ValidateType checks if a value matches the expected attribute type.
// This is a helper for type-checking raw filter values.
func ValidateType(value any, attrDef types.AttributeDef) error {
	switch attrDef.Type {
	case types.AttributeTypeString:
		if _, ok := value.(string); !ok {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected string, got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeInt, types.AttributeTypeInt64:
		if !isInt(value) {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected int, got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeFloat, types.AttributeTypeFloat64:
		if !isFloat(value) {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected float, got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeBool:
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected bool, got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeTime:
		// Time can be int (Unix timestamp) or float (Unix timestamp with decimals)
		if !isInt(value) && !isFloat(value) {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected time (numeric timestamp), got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeStringSlice:
		if !isStringSlice(value) {
			return &ValidationError{
				FilterName:   attrDef.Name,
				Reason:       fmt.Sprintf("expected []string, got %T", value),
				ExpectedType: attrDef.Type,
				ActualType:   inferType(value),
			}
		}
	case types.AttributeTypeGPS:
		// GPS is a special type - handled separately
		// For now, just allow it
	default:
		return &ValidationError{
			FilterName: attrDef.Name,
			Reason:     fmt.Sprintf("unknown attribute type: %s", attrDef.Type),
		}
	}
	return nil
}

// isInt checks if a value is an integer type (int, int64, etc.).
func isInt(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

// isFloat checks if a value is a float type.
func isFloat(value any) bool {
	_, ok := value.(float64)
	return ok
}

// isStringSlice checks if a value is a []string.
func isStringSlice(value any) bool {
	slice, ok := value.([]any)
	if !ok {
		return false
	}
	for _, v := range slice {
		if _, ok := v.(string); !ok {
			return false
		}
	}
	return true
}

// inferType attempts to infer the attribute type from a value.
// This is used for error reporting when types don't match.
func inferType(value any) types.AttributeType {
	switch value.(type) {
	case string:
		return types.AttributeTypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return types.AttributeTypeInt64
	case float64:
		return types.AttributeTypeFloat64
	case bool:
		return types.AttributeTypeBool
	case []any:
		return types.AttributeTypeStringSlice
	default:
		return "unknown"
	}
}

// MultiValidationError represents multiple validation errors.
// This is useful when validating multiple filters at once.
type MultiValidationError struct {
	Errors []error
}

// Error returns a human-readable error message.
func (e *MultiValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%d validation errors: %s", len(e.Errors), e.Errors[0].Error())
}

// AllErrors returns all validation errors.
func (e *MultiValidationError) AllErrors() []error {
	return e.Errors
}

// HasErrors returns true if there are any validation errors.
func (e *MultiValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// NewMultiValidationError creates a new MultiValidationError from a slice of errors.
// Nil errors are filtered out.
func NewMultiValidationError(errors []error) *MultiValidationError {
	var filtered []error
	for _, err := range errors {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return &MultiValidationError{Errors: filtered}
}

// AddError adds an error to the multi-error if it's non-nil.
func (e *MultiValidationError) AddError(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}
