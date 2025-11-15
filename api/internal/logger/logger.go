package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Global logger instance
var (
	Log zerolog.Logger
)

// Initialize sets up the global logger with configuration
func Initialize(level string, pretty bool) {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Configure output format
	if pretty {
		// Pretty console output for development
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON output for production
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}

	// Set global logger
	Log = log.With().
		Str("service", "streamspace-api").
		Logger()

	Log.Info().
		Str("level", logLevel.String()).
		Bool("pretty", pretty).
		Msg("Logger initialized")
}

// GetLogger returns the global logger instance
func GetLogger() *zerolog.Logger {
	return &Log
}

// Security creates a logger for security events
func Security() *zerolog.Logger {
	l := Log.With().Str("component", "security").Logger()
	return &l
}

// WebSocket creates a logger for WebSocket events
func WebSocket() *zerolog.Logger {
	l := Log.With().Str("component", "websocket").Logger()
	return &l
}

// Webhook creates a logger for webhook events
func Webhook() *zerolog.Logger {
	l := Log.With().Str("component", "webhook").Logger()
	return &l
}

// Integration creates a logger for integration events
func Integration() *zerolog.Logger {
	l := Log.With().Str("component", "integration").Logger()
	return &l
}

// Database creates a logger for database events
func Database() *zerolog.Logger {
	l := Log.With().Str("component", "database").Logger()
	return &l
}

// HTTP creates a logger for HTTP request events
func HTTP() *zerolog.Logger {
	l := Log.With().Str("component", "http").Logger()
	return &l
}
