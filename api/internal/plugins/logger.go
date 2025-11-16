package plugins

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// PluginLogger provides structured logging for plugins
type PluginLogger struct {
	pluginName string
}

// NewPluginLogger creates a new plugin logger
func NewPluginLogger(pluginName string) *PluginLogger {
	return &PluginLogger{
		pluginName: pluginName,
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Plugin    string                 `json:"plugin"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// log writes a structured log entry
func (pl *PluginLogger) log(level string, message string, data map[string]interface{}) {
	entry := LogEntry{
		Plugin:    pl.pluginName,
		Level:     level,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Marshal to JSON for structured logging
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[Plugin:%s] Failed to marshal log entry: %v", pl.pluginName, err)
		return
	}

	// Output structured log
	log.Println(string(jsonBytes))
}

// Debug logs a debug message
func (pl *PluginLogger) Debug(message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	pl.log("DEBUG", message, d)
}

// Info logs an info message
func (pl *PluginLogger) Info(message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	pl.log("INFO", message, d)
}

// Warn logs a warning message
func (pl *PluginLogger) Warn(message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	pl.log("WARN", message, d)
}

// Error logs an error message
func (pl *PluginLogger) Error(message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	pl.log("ERROR", message, d)
}

// Fatal logs a fatal message (does NOT exit)
func (pl *PluginLogger) Fatal(message string, data ...map[string]interface{}) {
	var d map[string]interface{}
	if len(data) > 0 {
		d = data[0]
	}
	pl.log("FATAL", message, d)
}

// WithField returns a logger with a default field
func (pl *PluginLogger) WithField(key string, value interface{}) *PluginLoggerWithFields {
	return &PluginLoggerWithFields{
		logger: pl,
		fields: map[string]interface{}{key: value},
	}
}

// WithFields returns a logger with default fields
func (pl *PluginLogger) WithFields(fields map[string]interface{}) *PluginLoggerWithFields {
	return &PluginLoggerWithFields{
		logger: pl,
		fields: fields,
	}
}

// PluginLoggerWithFields is a logger with pre-set fields
type PluginLoggerWithFields struct {
	logger *PluginLogger
	fields map[string]interface{}
}

// mergeData merges pre-set fields with provided data
func (plwf *PluginLoggerWithFields) mergeData(data ...map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy pre-set fields
	for k, v := range plwf.fields {
		merged[k] = v
	}

	// Merge with provided data
	if len(data) > 0 {
		for k, v := range data[0] {
			merged[k] = v
		}
	}

	return merged
}

// Debug logs a debug message with pre-set fields
func (plwf *PluginLoggerWithFields) Debug(message string, data ...map[string]interface{}) {
	plwf.logger.log("DEBUG", message, plwf.mergeData(data...))
}

// Info logs an info message with pre-set fields
func (plwf *PluginLoggerWithFields) Info(message string, data ...map[string]interface{}) {
	plwf.logger.log("INFO", message, plwf.mergeData(data...))
}

// Warn logs a warning message with pre-set fields
func (plwf *PluginLoggerWithFields) Warn(message string, data ...map[string]interface{}) {
	plwf.logger.log("WARN", message, plwf.mergeData(data...))
}

// Error logs an error message with pre-set fields
func (plwf *PluginLoggerWithFields) Error(message string, data ...map[string]interface{}) {
	plwf.logger.log("ERROR", message, plwf.mergeData(data...))
}

// Fatal logs a fatal message with pre-set fields
func (plwf *PluginLoggerWithFields) Fatal(message string, data ...map[string]interface{}) {
	plwf.logger.log("FATAL", message, plwf.mergeData(data...))
}
