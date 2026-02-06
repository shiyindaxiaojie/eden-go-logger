package logger

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
	OFF
)

var levelNames = map[Level]string{
	TRACE: "TRACE",
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
	OFF:   "OFF",
}

var levelValues = map[string]Level{
	"TRACE": TRACE,
	"DEBUG": DEBUG,
	"INFO":  INFO,
	"WARN":  WARN,
	"ERROR": ERROR,
	"FATAL": FATAL,
	"OFF":   OFF,
	"trace": TRACE,
	"debug": DEBUG,
	"info":  INFO,
	"warn":  WARN,
	"error": ERROR,
	"fatal": FATAL,
}

func (l Level) String() string {
	if name, ok := levelNames[l]; ok {
		return name
	}
	return "UNKNOWN"
}

// ParseLevel converts string to Level
func ParseLevel(s string) Level {
	if level, ok := levelValues[s]; ok {
		return level
	}
	return INFO
}

// Entry represents a single log event
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Logger  string
	Marker  string
	Context map[string]interface{}
	Caller  CallerInfo
	Error   error
	Fields  map[string]interface{}
}

// CallerInfo holds source code location
type CallerInfo struct {
	File     string
	Line     int
	Function string
}

// MDC (Mapped Diagnostic Context) for context propagation
type MDC struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func NewMDC() *MDC {
	return &MDC{data: make(map[string]interface{})}
}

func (m *MDC) Put(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *MDC) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *MDC) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *MDC) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
}

func (m *MDC) Clone() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clone := make(map[string]interface{}, len(m.data))
	for k, v := range m.data {
		clone[k] = v
	}
	return clone
}

// Logger is the main logging interface
type Logger struct {
	name            string
	level           Level
	includeLocation bool
	appenders       []Appender
	mdc             *MDC
	mu              sync.RWMutex
}

// NewLogger creates a new logger instance
func NewLogger(name string) *Logger {
	return &Logger{
		name:            name,
		level:           INFO,
		includeLocation: false,
		appenders:       make([]Appender, 0),
		mdc:             NewMDC(),
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetIncludeLocation sets whether to include caller location
func (l *Logger) SetIncludeLocation(include bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.includeLocation = include
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// AddAppender adds an appender to the logger
func (l *Logger) AddAppender(appender Appender) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.appenders = append(l.appenders, appender)
}

// MDC returns the MDC for context propagation
func (l *Logger) MDC() *MDC {
	return l.mdc
}

// IsEnabled checks if a level is enabled
func (l *Logger) IsEnabled(level Level) bool {
	return level >= l.GetLevel()
}

// log is the internal logging method
func (l *Logger) log(level Level, marker string, format string, args ...interface{}) {
	if !l.IsEnabled(level) {
		return
	}

	l.mu.RLock()
	includeLocation := l.includeLocation
	appenders := l.appenders
	l.mu.RUnlock()

	var caller CallerInfo
	if includeLocation {
		caller = getCaller(4)
	}

	entry := &Entry{
		Time:    time.Now(),
		Level:   level,
		Message: fmt.Sprintf(format, args...),
		Logger:  l.name,
		Marker:  marker,
		Context: l.mdc.Clone(),
		Caller:  caller,
		Fields:  make(map[string]interface{}),
	}

	for _, appender := range appenders {
		_ = appender.Append(entry)
	}
}

// Trace logs at TRACE level
func (l *Logger) Trace(format string, args ...interface{}) {
	l.log(TRACE, "", format, args...)
}

// Debug logs at DEBUG level
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, "", format, args...)
}

// Info logs at INFO level
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, "", format, args...)
}

// Warn logs at WARN level
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, "", format, args...)
}

// Error logs at ERROR level
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, "", format, args...)
}

// Fatal logs at FATAL level
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, "", format, args...)
}

// WithMarker returns a MarkerLogger for categorized logging
func (l *Logger) WithMarker(marker string) *MarkerLogger {
	return &MarkerLogger{logger: l, marker: marker}
}

// WithContext adds context and returns the logger for chaining
func (l *Logger) WithContext(key string, value interface{}) *Logger {
	l.mdc.Put(key, value)
	return l
}

// WithFields logs with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{logger: l, fields: fields}
}

// Close closes all appenders
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, appender := range l.appenders {
		_ = appender.Close()
	}
	return nil
}

// MarkerLogger wraps logger with a marker
type MarkerLogger struct {
	logger *Logger
	marker string
}

func (m *MarkerLogger) Trace(format string, args ...interface{}) {
	m.logger.log(TRACE, m.marker, format, args...)
}

func (m *MarkerLogger) Debug(format string, args ...interface{}) {
	m.logger.log(DEBUG, m.marker, format, args...)
}

func (m *MarkerLogger) Info(format string, args ...interface{}) {
	m.logger.log(INFO, m.marker, format, args...)
}

func (m *MarkerLogger) Warn(format string, args ...interface{}) {
	m.logger.log(WARN, m.marker, format, args...)
}

func (m *MarkerLogger) Error(format string, args ...interface{}) {
	m.logger.log(ERROR, m.marker, format, args...)
}

// FieldLogger wraps logger with additional fields
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

func (f *FieldLogger) log(level Level, format string, args ...interface{}) {
	if !f.logger.IsEnabled(level) {
		return
	}

	entry := &Entry{
		Time:    time.Now(),
		Level:   level,
		Message: fmt.Sprintf(format, args...),
		Logger:  f.logger.name,
		Context: f.logger.mdc.Clone(),
		Caller:  getCaller(4),
		Fields:  f.fields,
	}

	f.logger.mu.RLock()
	appenders := f.logger.appenders
	f.logger.mu.RUnlock()

	for _, appender := range appenders {
		_ = appender.Append(entry)
	}
}

func (f *FieldLogger) Info(format string, args ...interface{}) {
	f.log(INFO, format, args...)
}

func (f *FieldLogger) Error(format string, args ...interface{}) {
	f.log(ERROR, format, args...)
}

// getCaller retrieves caller information
func getCaller(skip int) CallerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return CallerInfo{}
	}
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
	}
	// Extract just the file name
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			file = file[i+1:]
			break
		}
	}
	return CallerInfo{
		File:     file,
		Line:     line,
		Function: funcName,
	}
}

// Context-aware logging
type ContextLogger struct {
	logger *Logger
	ctx    context.Context
}

func (l *Logger) WithCtx(ctx context.Context) *ContextLogger {
	return &ContextLogger{logger: l, ctx: ctx}
}

func (c *ContextLogger) Info(format string, args ...interface{}) {
	c.logger.Info(format, args...)
}

func (c *ContextLogger) Error(format string, args ...interface{}) {
	c.logger.Error(format, args...)
}
