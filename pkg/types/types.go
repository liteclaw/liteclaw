// Package types provides shared types used across packages.
package types

// Error codes
const (
	ErrCodeInvalidInput       = "INVALID_INPUT"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeRateLimited        = "RATE_LIMITED"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// APIError represents an API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// Pagination holds pagination parameters.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// PaginatedResponse is a generic paginated response.
type PaginatedResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Response is a generic API response.
type Response[T any] struct {
	Success bool      `json:"success"`
	Data    T         `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// OK creates a successful response.
func OK[T any](data T) *Response[T] {
	return &Response[T]{Success: true, Data: data}
}

// Err creates an error response.
func Err[T any](code, message string) *Response[T] {
	return &Response[T]{
		Success: false,
		Error:   &APIError{Code: code, Message: message},
	}
}
