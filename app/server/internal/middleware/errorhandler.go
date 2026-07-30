package middleware

import (
	"crypto/rand"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/sushazhi/fnos-logmanager/internal/errors"
)

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *APIMeta    `json:"meta,omitempty"`
}

// APIError represents an error in the API response.
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// APIMeta represents metadata in the API response.
type APIMeta struct {
	Timestamp int64  `json:"timestamp"`
	RequestID string `json:"requestId"`
}

func generateRequestID() string {
	return time.Now().Format("150405.000") + "-" + randomString(9)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			b[i] = letters[i%len(letters)] // fallback, should never happen
			continue
		}
		b[i] = letters[idx.Int64()]
	}
	return string(b)
}

// NotFoundHandler handles 404 routes.
func NotFoundHandler(c *gin.Context) {
	requestID := generateRequestID()
	c.JSON(http.StatusNotFound, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "NOT_FOUND",
			Message: "资源不存在",
			Details: map[string]string{"path": c.Request.URL.Path},
		},
		Meta: &APIMeta{
			Timestamp: time.Now().UnixMilli(),
			RequestID: requestID,
		},
	})
}

// ErrorHandler is the global error handler middleware.
func ErrorHandler(c *gin.Context) {
	c.Next()

	// Only handle errors that occurred during request processing
	if len(c.Errors) == 0 {
		return
	}

	err := c.Errors.Last().Err
	requestID := generateRequestID()

	// Log the error
	slog.Error("request error",
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"requestId", requestID,
		"error", err.Error(),
	)

	switch e := err.(type) {
	case *apperrors.AppError:
		c.JSON(e.StatusCode, APIResponse{
			Success: false,
			Error: &APIError{
				Code:    e.Code,
				Message: e.Message,
				Details: e.Details,
			},
			Meta: &APIMeta{
				Timestamp: time.Now().UnixMilli(),
				RequestID: requestID,
			},
		})
		return
	case *apperrors.ValidationError:
		c.JSON(400, APIResponse{
			Success: false,
			Error: &APIError{
				Code:    "VALIDATION_ERROR",
				Message: e.Message,
				Details: e.ValidationErrors,
			},
			Meta: &APIMeta{
				Timestamp: time.Now().UnixMilli(),
				RequestID: requestID,
			},
		})
		return
	}

	// Unknown error — always return a generic message to prevent information
	// leakage. The full error is logged server-side (see slog.Error above)
	// and can be correlated via the requestId field.
	c.JSON(http.StatusInternalServerError, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "INTERNAL_ERROR",
			Message: "服务器内部错误",
		},
		Meta: &APIMeta{
			Timestamp: time.Now().UnixMilli(),
			RequestID: requestID,
		},
	})
}
