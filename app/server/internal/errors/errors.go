package errors

import "fmt"

// AppError is the base application error.
type AppError struct {
	StatusCode int    `json:"statusCode"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewAppError creates a new AppError.
func NewAppError(message string, statusCode int, code string) *AppError {
	return &AppError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	}
}

// ValidationError represents a 400 validation error.
type ValidationError struct {
	AppError
	ValidationErrors []ValidationErrorDetail `json:"errors,omitempty"`
}

// ValidationErrorDetail represents a single field validation error.
type ValidationErrorDetail struct {
	Msg      string      `json:"msg"`
	Param    string      `json:"param,omitempty"`
	Location string      `json:"location,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string, details []ValidationErrorDetail) *ValidationError {
	return &ValidationError{
		AppError: AppError{
			StatusCode: 400,
			Code:       "VALIDATION_ERROR",
			Message:    message,
		},
		ValidationErrors: details,
	}
}

// AuthenticationError represents a 401 error.
type AuthenticationError struct {
	AppError
}

// NewAuthenticationError creates a new AuthenticationError.
func NewAuthenticationError(message string) *AuthenticationError {
	if message == "" {
		message = "需要认证"
	}
	return &AuthenticationError{
		AppError: AppError{
			StatusCode: 401,
			Code:       "AUTHENTICATION_ERROR",
			Message:    message,
		},
	}
}

// RateLimitError represents a 429 error.
type RateLimitError struct {
	AppError
}

// NewRateLimitError creates a new RateLimitError.
func NewRateLimitError(message string) *RateLimitError {
	if message == "" {
		message = "请求过于频繁"
	}
	return &RateLimitError{
		AppError: AppError{
			StatusCode: 429,
			Code:       "RATE_LIMIT_ERROR",
			Message:    message,
		},
	}
}

// CSRFError represents a 403 CSRF error.
type CSRFError struct {
	AppError
}

// NewCSRFError creates a new CSRFError.
func NewCSRFError(message string) *CSRFError {
	if message == "" {
		message = "CSRF验证失败"
	}
	return &CSRFError{
		AppError: AppError{
			StatusCode: 403,
			Code:       "CSRF_ERROR",
			Message:    message,
		},
	}
}
