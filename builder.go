package logger

import (
	"fmt"
	"strings"
	"time"
)

// Fields type alias for compatibility
type Fields map[string]interface{}

// Builder provides a fluent API for configuring a Logger
type Builder struct {
	name            string
	level           Level
	includeLocation bool
	appenders       []Appender
}

// NewBuilder creates a new logger builder
func NewBuilder() *Builder {
	return &Builder{
		name:            "root",
		level:           INFO,
		includeLocation: false,
		appenders:       make([]Appender, 0),
	}
}

// SetName sets the logger name
func (b *Builder) SetName(name string) *Builder {
	b.name = name
	return b
}

// SetLevel sets the log level
func (b *Builder) SetLevel(level Level) *Builder {
	b.level = level
	return b
}

// SetLevelString sets the log level from string
func (b *Builder) SetLevelString(level string) *Builder {
	b.level = ParseLevel(level)
	return b
}

// IncludeLocation sets whether to include caller location
func (b *Builder) IncludeLocation(include bool) *Builder {
	b.includeLocation = include
	return b
}

// AddAppender adds an appender
func (b *Builder) AddAppender(appender Appender) *Builder {
	b.appenders = append(b.appenders, appender)
	return b
}

// AddConsole adds a console appender with default settings
func (b *Builder) AddConsole() *Builder {
	return b.AddAppender(NewConsoleAppender())
}

// AddFile adds a file appender
func (b *Builder) AddFile(filename string) *Builder {
	return b.AddAppender(NewFileAppender(filename))
}

// Level sets the log level from string (Alias for SetLevelString)
func (b *Builder) Level(level string) *Builder {
	return b.SetLevelString(level)
}

// AddRollingFile adds a rolling file appender with default settings
func (b *Builder) AddRollingFile(filename string, maxSize int64, maxBackups int) *Builder {
	appender := NewRollingFileAppender(filename).
		WithPolicy(NewSizeBasedPolicy(maxSize)).
		WithMaxBackups(maxBackups)
	return b.AddAppender(appender)
}

// Console adds a console appender with functional options
func (b *Builder) Console(opts ...func(*ConsoleAppender)) *Builder {
	c := NewConsoleAppender()
	for _, opt := range opts {
		opt(c)
	}
	return b.AddAppender(c)
}

// RollingFile adds a rolling file appender with functional options
func (b *Builder) RollingFile(filename string, opts ...func(*RollingFileAppender)) *Builder {
	rf := NewRollingFileAppender(filename)
	for _, opt := range opts {
		opt(rf)
	}
	return b.AddAppender(rf)
}

// Init builds the logger and sets it as the global logger
func (b *Builder) Init() {
	globalLogger = b.Build()
}

// Build constructs the Logger
func (b *Builder) Build() *Logger {
	logger := NewLogger(b.name)
	logger.SetLevel(b.level)
	logger.SetIncludeLocation(b.includeLocation)

	for _, appender := range b.appenders {
		logger.AddAppender(appender)
	}

	// If no appenders configured, add console as default
	if len(b.appenders) == 0 {
		logger.AddAppender(NewConsoleAppender())
	}

	return logger
}

// Global logger instance
var globalLogger *Logger

// ============================================================================
// Configuration Structs (User-Defined Custom Format)
// ============================================================================

// Configuration defines the log configuration
type Configuration struct {
	Level           string           `yaml:"level" json:"level"`                       // DEBUG, INFO, WARN, ERROR, FATAL
	Format          string           `yaml:"format" json:"format"`                     // text, json
	Pattern         string           `yaml:"pattern" json:"pattern"`                   // Global pattern
	Policies        *PoliciesConfig  `yaml:"policies" json:"policies"`                 // Global triggering policies
	Rollover        *RolloverConfig  `yaml:"rollover" json:"rollover"`                 // Global rollover strategy
	IncludeLocation bool             `yaml:"include_location" json:"include_location"` // Whether to include caller location
	Appenders       []AppenderConfig `yaml:"appenders" json:"appenders"`               // List of appenders
}

// PoliciesConfig defines triggering policies
type PoliciesConfig struct {
	CronTriggeringPolicy      *CronPolicyConfig `yaml:"cron_triggering_policy" json:"cron_triggering_policy"`
	SizeBasedTriggeringPolicy *SizePolicyConfig `yaml:"size_based_triggering_policy" json:"size_based_triggering_policy"`
}

// CronPolicyConfig for cron-based triggering
type CronPolicyConfig struct {
	Schedule string `yaml:"schedule" json:"schedule"` // e.g. "0 0 4 * * ?"
}

// SizePolicyConfig for size-based triggering
type SizePolicyConfig struct {
	Size string `yaml:"size" json:"size"` // e.g. "20MB"
}

// RolloverConfig defines rollover strategy
type RolloverConfig struct {
	MaxFile   int    `yaml:"max_file" json:"max_file"`   // Max backup files (0 = no limit)
	Retention string `yaml:"retention" json:"retention"` // e.g. "30d"
}

// AppenderConfig defines configuration for an appender
type AppenderConfig struct {
	Name        string                 `yaml:"name" json:"name"`
	Type        string                 `yaml:"type" json:"type"` // Console, RollingFile
	Level       string                 `yaml:"level" json:"level"`
	Pattern     string                 `yaml:"pattern" json:"pattern"`
	FileName    string                 `yaml:"file_name" json:"file_name"`
	FilePattern string                 `yaml:"file_pattern" json:"file_pattern"` // e.g. access-%i.log.gz
	Filter      map[string]interface{} `yaml:"filter" json:"filter"`
	Async       bool                   `yaml:"async" json:"async"`       // Whether to use async appender
	Rollover    *RolloverConfig        `yaml:"rollover" json:"rollover"` // Per-appender override
}

// ============================================================================
// Init Function
// ============================================================================

// Init initializes the global logger with the configuration
func Init(cfg Configuration) error {
	builder := NewBuilder()

	// Set global level
	if cfg.Level != "" {
		builder.SetLevel(ParseLevel(cfg.Level))
	}

	// Set include location
	if cfg.IncludeLocation {
		builder.IncludeLocation(true)
	}

	// Determine global layout
	var globalLayout Layout
	if cfg.Pattern != "" {
		globalLayout = NewPatternLayout(cfg.Pattern)
	} else if strings.ToLower(cfg.Format) == "json" {
		globalLayout = NewJSONLayout()
	} else {
		globalLayout = NewTextLayout()
	}

	// Parse global rollover config
	globalMaxFile := 0
	var globalRetention time.Duration
	if cfg.Rollover != nil {
		globalMaxFile = cfg.Rollover.MaxFile
		globalRetention = parseDuration(cfg.Rollover.Retention)
	}

	// Parse global policies
	var globalSizeBytes int64
	var globalCronSchedule string
	if cfg.Policies != nil {
		if cfg.Policies.SizeBasedTriggeringPolicy != nil {
			globalSizeBytes = parseSize(cfg.Policies.SizeBasedTriggeringPolicy.Size)
		}
		if cfg.Policies.CronTriggeringPolicy != nil {
			globalCronSchedule = cfg.Policies.CronTriggeringPolicy.Schedule
		}
	}

	// Build appenders
	if len(cfg.Appenders) == 0 {
		// Default to console
		builder.AddConsole()
	} else {
		for _, appCfg := range cfg.Appenders {
			var appender Appender

			switch strings.ToLower(appCfg.Type) {
			case "console":
				c := NewConsoleAppender()
				if appCfg.Pattern != "" {
					c.WithLayout(NewPatternLayout(appCfg.Pattern))
				} else {
					c.WithLayout(globalLayout)
				}
				if appCfg.Name != "" {
					c.WithName(appCfg.Name)
				}
				// Construct filter
				var filter Filter
				if appCfg.Level != "" {
					filter = NewThresholdFilter(ParseLevel(appCfg.Level))
				}

				if len(appCfg.Filter) > 0 {
					if customFilter := ParseFilter(appCfg.Filter); customFilter != nil {
						if filter != nil {
							filter = NewCompositeFilter(ALL, filter, customFilter)
						} else {
							filter = customFilter
						}
					}
				}

				if filter != nil {
					c.WithFilter(filter)
				}
				appender = c

			case "rollingfile", "file":
				filename := appCfg.FileName
				if filename == "" {
					filename = "app.log"
				}

				rf := NewRollingFileAppender(filename)

				// Layout
				if appCfg.Pattern != "" {
					rf.WithLayout(NewPatternLayout(appCfg.Pattern))
				} else {
					rf.WithLayout(globalLayout)
				}

				// Name
				if appCfg.Name != "" {
					rf.WithName(appCfg.Name)
				}

				// Construct filter
				var filter Filter
				if appCfg.Level != "" {
					filter = NewThresholdFilter(ParseLevel(appCfg.Level))
				}

				if len(appCfg.Filter) > 0 {
					if customFilter := ParseFilter(appCfg.Filter); customFilter != nil {
						if filter != nil {
							// If both level and custom filter are present, require BOTH to accept (AND logic)
							filter = NewCompositeFilter(ALL, filter, customFilter)
						} else {
							filter = customFilter
						}
					}
				}

				if filter != nil {
					rf.WithFilter(filter)
				}

				// Policies (use global if not overridden)
				if globalSizeBytes > 0 {
					rf.WithPolicy(NewSizeBasedPolicy(globalSizeBytes))
				}
				if globalCronSchedule != "" {
					rf.WithPolicy(NewCronBasedPolicy(globalCronSchedule))
				}

				// Rollover strategy (per-appender overrides global)
				maxFile := globalMaxFile
				retention := globalRetention
				if appCfg.Rollover != nil {
					if appCfg.Rollover.MaxFile > 0 {
						maxFile = appCfg.Rollover.MaxFile
					}
					if appCfg.Rollover.Retention != "" {
						retention = parseDuration(appCfg.Rollover.Retention)
					}
				}
				if maxFile > 0 {
					rf.WithMaxBackups(maxFile)
				}
				if retention > 0 {
					rf.WithMaxAge(retention)
				}

				appender = rf

			default:
				// Unknown type, skip
				continue
			}

			// Wrap in AsyncAppender if configured
			if appCfg.Async {
				// Default buffer size 4096 is hardcoded in NewAsyncAppender for now
				// We can expose it in config later if needed
				appender = NewAsyncAppender(appender, 0)
			}

			builder.AddAppender(appender)
		}
	}

	globalLogger = builder.Build()
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// parseSize parses size string like "20MB" to int64 bytes
func parseSize(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	var val int64
	if strings.HasSuffix(s, "KB") {
		fmt.Sscanf(s, "%dKB", &val)
		return val * 1024
	}
	if strings.HasSuffix(s, "MB") {
		fmt.Sscanf(s, "%dMB", &val)
		return val * 1024 * 1024
	}
	if strings.HasSuffix(s, "GB") {
		fmt.Sscanf(s, "%dGB", &val)
		return val * 1024 * 1024 * 1024
	}
	fmt.Sscanf(s, "%d", &val)
	return val
}

// parseDuration parses duration like "7d", "30d"
func parseDuration(s string) time.Duration {
	s = strings.ToUpper(strings.TrimSpace(s))
	if strings.HasSuffix(s, "D") {
		daysStr := strings.TrimSuffix(s, "D")
		var days int
		fmt.Sscanf(daysStr, "%d", &days)
		return time.Duration(days) * 24 * time.Hour
	}
	d, _ := time.ParseDuration(s)
	return d
}

// ============================================================================
// Package-level logging functions
// ============================================================================

func GetLogger() interface{} {
	return globalLogger
}

func Trace(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Trace(format, args...)
	}
}

func Debug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

func Fatal(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatal(format, args...)
	}
}

func WithMarker(marker string) *MarkerLogger {
	if globalLogger != nil {
		return globalLogger.WithMarker(marker)
	}
	return nil
}

func WithContext(key string, value interface{}) *Logger {
	if globalLogger != nil {
		return globalLogger.WithContext(key, value)
	}
	return nil
}

func SQL(sql string, duration time.Duration, rows int64) {
	if globalLogger != nil {
		globalLogger.WithMarker("SQL").Debug("[%dms] [rows:%d] %s", duration.Milliseconds(), rows, sql)
	}
}

func SQLWithError(sql string, duration time.Duration, rows int64, isError bool) {
	if globalLogger != nil {
		if isError {
			globalLogger.WithMarker("SQL").Error("[%dms] [rows:%d] %s", duration.Milliseconds(), rows, sql)
		} else {
			globalLogger.WithMarker("SQL").Debug("[%dms] [rows:%d] %s", duration.Milliseconds(), rows, sql)
		}
	}
}

func API(method, path, clientIP string, statusCode int, duration time.Duration) {
	if globalLogger != nil {
		globalLogger.WithMarker("API").Info("[%dms] [%d] %s %s %s", duration.Milliseconds(), statusCode, clientIP, method, path)
	}
}

func LogHTTPRequest(statusCode int, method, path string, latency time.Duration, clientIP string) {
	API(method, path, clientIP, statusCode, latency)
}

// WithFields adds fields to the global logger
func WithFields(fields map[string]interface{}) *FieldLogger {
	if globalLogger != nil {
		return globalLogger.WithFields(fields)
	}
	// Return a dummy/safe logger if globalLogger is nil?
	// Or panic/return nil. Existing methods return nil.
	return nil
}

// WithField adds a single field
func WithField(key string, value interface{}) *FieldLogger {
	if globalLogger != nil {
		return globalLogger.WithFields(map[string]interface{}{key: value})
	}
	return nil
}

// WithError adds an error field
func WithError(err error) *FieldLogger {
	if globalLogger != nil {
		return globalLogger.WithFields(map[string]interface{}{"error": err})
	}
	return nil
}
