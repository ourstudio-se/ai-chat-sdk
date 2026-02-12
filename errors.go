package aichat

import (
	"errors"
	"fmt"
)

// Error codes for SDK errors.
const (
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeRouting       = "ROUTING_ERROR"
	ErrCodeToolExecution = "TOOL_EXECUTION_ERROR"
	ErrCodeLLM           = "LLM_ERROR"
	ErrCodeSchema        = "SCHEMA_ERROR"
	ErrCodeStorage       = "STORAGE_ERROR"
	ErrCodeConfiguration = "CONFIGURATION_ERROR"
	ErrCodeTimeout       = "TIMEOUT_ERROR"
	ErrCodeInternal      = "INTERNAL_ERROR"
)

// Sentinel errors for common error conditions.
var (
	ErrSkillNotFound              = errors.New("skill not found")
	ErrToolNotFound               = errors.New("tool not found")
	ErrConversationNotFound       = errors.New("conversation not found")
	ErrNoSkillMatched             = errors.New("no skill matched the message")
	ErrMissingContext             = errors.New("missing required context")
	ErrInvalidParameter           = errors.New("invalid parameter")
	ErrSchemaValidation           = errors.New("response does not match schema")
	ErrMaxTurnsExceeded           = errors.New("maximum agent turns exceeded")
	ErrActionRequiresConfirmation = errors.New("action requires user confirmation")
)

// SDKError represents an error from the SDK with additional context.
type SDKError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// Cause is the underlying error, if any.
	Cause error `json:"-"`

	// Details contains additional error context.
	Details map[string]any `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *SDKError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *SDKError) Unwrap() error {
	return e.Cause
}

// NewSDKError creates a new SDKError.
func NewSDKError(code, message string, cause error) *SDKError {
	return &SDKError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewSDKErrorWithDetails creates a new SDKError with additional details.
func NewSDKErrorWithDetails(code, message string, cause error, details map[string]any) *SDKError {
	return &SDKError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Details: details,
	}
}

// NewValidationError creates a validation error.
func NewValidationError(message string, cause error) *SDKError {
	return NewSDKError(ErrCodeValidation, message, cause)
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(resource string, identifier string) *SDKError {
	return NewSDKErrorWithDetails(
		ErrCodeNotFound,
		fmt.Sprintf("%s not found: %s", resource, identifier),
		nil,
		map[string]any{
			"resource":   resource,
			"identifier": identifier,
		},
	)
}

// NewRoutingError creates a routing error.
func NewRoutingError(message string, cause error) *SDKError {
	return NewSDKError(ErrCodeRouting, message, cause)
}

// NewToolError creates a tool execution error.
func NewToolError(toolName string, cause error) *SDKError {
	return NewSDKErrorWithDetails(
		ErrCodeToolExecution,
		fmt.Sprintf("tool execution failed: %s", toolName),
		cause,
		map[string]any{
			"tool": toolName,
		},
	)
}

// NewLLMError creates an LLM-related error.
func NewLLMError(message string, cause error) *SDKError {
	return NewSDKError(ErrCodeLLM, message, cause)
}

// NewSchemaError creates a schema validation error.
func NewSchemaError(message string, cause error) *SDKError {
	return NewSDKError(ErrCodeSchema, message, cause)
}

// NewConfigurationError creates a configuration error.
func NewConfigurationError(message string, cause error) *SDKError {
	return NewSDKError(ErrCodeConfiguration, message, cause)
}

// ErrMissingContextValue creates an error for missing required context.
func ErrMissingContextValue(key string) *SDKError {
	return NewSDKErrorWithDetails(
		ErrCodeValidation,
		fmt.Sprintf("missing required context value: %s", key),
		ErrMissingContext,
		map[string]any{
			"key": key,
		},
	)
}

// IsSDKError checks if an error is an SDKError with a specific code.
func IsSDKError(err error, code string) bool {
	var sdkErr *SDKError
	if errors.As(err, &sdkErr) {
		return sdkErr.Code == code
	}
	return false
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return IsSDKError(err, ErrCodeValidation)
}

// IsNotFoundError checks if an error is a not found error.
func IsNotFoundError(err error) bool {
	return IsSDKError(err, ErrCodeNotFound)
}
