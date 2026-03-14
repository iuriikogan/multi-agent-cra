// Package logger provides a structured logging wrapper using slog.
package logger

import (
	"log/slog"
	"os"
)

// Setup initializes the global default logger with the given level.
// Supports DEBUG, INFO (default), WARN, and ERROR.
func Setup(level string) {
	var logLevel slog.Level
	switch level {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: true,
	}

	// Use JSON handler for structured logs suitable for Google Cloud Logging.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}
