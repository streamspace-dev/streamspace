package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/getsentry/sentry-go"
	"github.com/yourusername/streamspace/api/internal/plugins"
)

// SentryPlugin sends errors and performance data to Sentry
type SentryPlugin struct {
	plugins.BasePlugin
	config        SentryConfig
	ignoreRegexps []*regexp.Regexp
}

// SentryConfig holds Sentry configuration
type SentryConfig struct {
	Enabled                bool              `json:"enabled"`
	DSN                    string            `json:"dsn"`
	Environment            string            `json:"environment"`
	Release                string            `json:"release"`
	ServerName             string            `json:"serverName"`
	EnableTracing          bool              `json:"enableTracing"`
	TracesSampleRate       float64           `json:"tracesSampleRate"`
	AttachStacktrace       bool              `json:"attachStacktrace"`
	SendDefaultPii         bool              `json:"sendDefaultPii"`
	CaptureSessionErrors   bool              `json:"captureSessionErrors"`
	CaptureAPIErrors       bool              `json:"captureAPIErrors"`
	CaptureUnhandledErrors bool              `json:"captureUnhandledErrors"`
	IgnoreErrors           []string          `json:"ignoreErrors"`
	BeforeSend             string            `json:"beforeSend"`
	Tags                   map[string]string `json:"tags"`
}

// Initialize sets up the Sentry plugin
func (p *SentryPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Load configuration
	configBytes, err := json.Marshal(ctx.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &p.config); err != nil {
		return fmt.Errorf("failed to unmarshal Sentry config: %w", err)
	}

	if !p.config.Enabled {
		ctx.Logger.Info("Sentry integration is disabled")
		return nil
	}

	if p.config.DSN == "" {
		return fmt.Errorf("Sentry DSN is required")
	}

	// Compile ignore error regexps
	p.ignoreRegexps = make([]*regexp.Regexp, 0, len(p.config.IgnoreErrors))
	for _, pattern := range p.config.IgnoreErrors {
		re, err := regexp.Compile(pattern)
		if err != nil {
			ctx.Logger.Warn("Failed to compile ignore error pattern", "pattern", pattern, "error", err)
			continue
		}
		p.ignoreRegexps = append(p.ignoreRegexps, re)
	}

	// Initialize Sentry SDK
	err = sentry.Init(sentry.ClientOptions{
		Dsn:              p.config.DSN,
		Environment:      p.config.Environment,
		Release:          p.config.Release,
		ServerName:       p.config.ServerName,
		AttachStacktrace: p.config.AttachStacktrace,
		SendDefaultPII:   p.config.SendDefaultPii,
		TracesSampleRate: p.config.TracesSampleRate,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Apply ignore patterns
			if event.Message != "" {
				for _, re := range p.ignoreRegexps {
					if re.MatchString(event.Message) {
						return nil // Ignore this error
					}
				}
			}

			// Add global tags
			for k, v := range p.config.Tags {
				event.Tags[k] = v
			}

			return event
		},
	})

	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Set global tags
	for k, v := range p.config.Tags {
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag(k, v)
		})
	}

	ctx.Logger.Info("Sentry plugin initialized successfully",
		"environment", p.config.Environment,
		"release", p.config.Release,
		"tracing_enabled", p.config.EnableTracing,
		"sample_rate", p.config.TracesSampleRate,
	)

	return nil
}

// OnLoad is called when the plugin is loaded
func (p *SentryPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Sentry error tracking plugin loaded")

	sentry.CaptureMessage("StreamSpace Sentry Plugin Loaded")

	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *SentryPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Sentry error tracking plugin unloading")

	sentry.CaptureMessage("StreamSpace Sentry Plugin Unloaded")

	// Flush any pending events
	sentry.Flush(5000) // 5 second timeout

	return nil
}

// OnSessionCreated tracks session creation in Sentry
func (p *SentryPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])
	templateName := fmt.Sprintf("%v", sessionMap["template_name"])

	// Create breadcrumb
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "info",
		Category: "session",
		Message:  "Session created",
		Data: map[string]interface{}{
			"session_id": sessionID,
			"user_id":    userID,
			"template":   templateName,
		},
		Level: sentry.LevelInfo,
	})

	return nil
}

// OnSessionTerminated tracks session termination
func (p *SentryPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session format")
	}

	sessionID := fmt.Sprintf("%v", sessionMap["id"])
	userID := fmt.Sprintf("%v", sessionMap["user_id"])

	// Create breadcrumb
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "info",
		Category: "session",
		Message:  "Session terminated",
		Data: map[string]interface{}{
			"session_id": sessionID,
			"user_id":    userID,
		},
		Level: sentry.LevelInfo,
	})

	return nil
}

// OnSessionError captures session errors
func (p *SentryPlugin) OnSessionError(ctx *plugins.PluginContext, errorData interface{}) error {
	if !p.config.Enabled || !p.config.CaptureSessionErrors {
		return nil
	}

	errorMap, ok := errorData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid error format")
	}

	errorMsg := fmt.Sprintf("%v", errorMap["error"])
	sessionID := fmt.Sprintf("%v", errorMap["session_id"])
	userID := fmt.Sprintf("%v", errorMap["user_id"])

	// Check if error should be ignored
	for _, re := range p.ignoreRegexps {
		if re.MatchString(errorMsg) {
			return nil
		}
	}

	// Capture exception with context
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("session_id", sessionID)
		scope.SetTag("user_id", userID)
		scope.SetContext("session", map[string]interface{}{
			"session_id": sessionID,
			"user_id":    userID,
			"error":      errorMsg,
		})

		if stack, ok := errorMap["stack"].(string); ok {
			scope.SetExtra("stack_trace", stack)
		}

		sentry.CaptureException(fmt.Errorf("session error: %s", errorMsg))
	})

	return nil
}

// OnUserCreated tracks user creation
func (p *SentryPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled {
		return nil
	}

	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user format")
	}

	userID := fmt.Sprintf("%v", userMap["id"])

	// Create breadcrumb
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "info",
		Category: "user",
		Message:  "User created",
		Data: map[string]interface{}{
			"user_id": userID,
		},
		Level: sentry.LevelInfo,
	})

	return nil
}

// CaptureError is a helper method to capture errors from other parts of StreamSpace
func (p *SentryPlugin) CaptureError(err error, context map[string]interface{}) {
	if !p.config.Enabled {
		return
	}

	// Check if error should be ignored
	errorMsg := err.Error()
	for _, re := range p.ignoreRegexps {
		if re.MatchString(errorMsg) {
			return
		}
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		// Add all context data
		for k, v := range context {
			scope.SetTag(k, fmt.Sprintf("%v", v))
		}

		sentry.CaptureException(err)
	})
}

// CaptureMessage captures a message with level
func (p *SentryPlugin) CaptureMessage(message string, level sentry.Level, context map[string]interface{}) {
	if !p.config.Enabled {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)

		// Add context
		for k, v := range context {
			scope.SetTag(k, fmt.Sprintf("%v", v))
		}

		sentry.CaptureMessage(message)
	})
}

// StartTransaction starts a performance transaction
func (p *SentryPlugin) StartTransaction(name string, operation string) *sentry.Span {
	if !p.config.Enabled || !p.config.EnableTracing {
		return nil
	}

	ctx := sentry.StartTransaction(sentry.Context{}, name)
	ctx.Op = operation

	return ctx
}

// Export the plugin
func init() {
	plugins.Register("streamspace-sentry", &SentryPlugin{})
}
