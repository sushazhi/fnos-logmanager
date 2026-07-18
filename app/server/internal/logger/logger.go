package logger

import (
	"log/slog"
	"os"
	"strings"
)

var baseLogger *slog.Logger
var logLevel slog.Level

func init() {
	Init(os.Getenv("LOG_LEVEL"))
}

// Init initializes the logger with the given log level string.
// Called explicitly in main() to ensure the level from config is used.
func Init(levelStr string) {
	logLevel = slog.LevelInfo
	switch strings.ToLower(levelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Pretty-print in development
	env := os.Getenv("NODE_ENV")
	isDev := env != "production" || os.Getenv("LOG_PRETTY") == "true"
	if isDev {
		baseLogger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	} else {
		baseLogger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
}

// Default returns the base logger.
func Default() *slog.Logger {
	return baseLogger
}

// With returns a logger with the given attributes added.
func With(attrs ...slog.Attr) *slog.Logger {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return baseLogger.With(args...)
}

// GetLogLevel returns the current log level.
func GetLogLevel() string {
	return logLevel.String()
}

// IsDevelopment returns true if running in development mode.
func IsDevelopment() bool {
	return os.Getenv("NODE_ENV") != "production"
}
