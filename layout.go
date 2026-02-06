package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Layout formats log entries for output
type Layout interface {
	Format(entry *Entry) []byte
}

// PatternLayout formats logs using a pattern string
// Supported patterns:
//
//	%d{format} - date/time (Go time format)
//	%p         - level
//	%c         - logger name
//	%m         - message
//	%n         - newline
//	%F         - file name
//	%L         - line number
//	%M         - method/function name
//	%X{key}    - MDC value
//	%marker    - marker
type PatternLayout struct {
	pattern string
	parts   []patternPart
}

type patternPart struct {
	literal  string
	variable string
	param    string
}

var patternRegex = regexp.MustCompile(`%(\w+)(?:\{([^}]+)\})?`)

// NewPatternLayout creates a new pattern layout
// Example: "%d{2006-01-02 15:04:05.000} [%p] %c - %m%n"
func NewPatternLayout(pattern string) *PatternLayout {
	pl := &PatternLayout{pattern: pattern}
	pl.parse()
	return pl
}

func (p *PatternLayout) parse() {
	s := p.pattern
	for {
		loc := patternRegex.FindStringSubmatchIndex(s)
		if loc == nil {
			if len(s) > 0 {
				p.parts = append(p.parts, patternPart{literal: s})
			}
			break
		}

		// Add literal before match
		if loc[0] > 0 {
			p.parts = append(p.parts, patternPart{literal: s[:loc[0]]})
		}

		// Extract variable and optional param
		variable := s[loc[2]:loc[3]]
		param := ""
		if loc[4] >= 0 && loc[5] >= 0 {
			param = s[loc[4]:loc[5]]
		}
		p.parts = append(p.parts, patternPart{variable: variable, param: param})

		s = s[loc[1]:]
	}
}

// Format applies the pattern to an entry
func (p *PatternLayout) Format(entry *Entry) []byte {
	var buf bytes.Buffer

	for _, part := range p.parts {
		if part.literal != "" {
			buf.WriteString(part.literal)
			continue
		}

		switch part.variable {
		case "d":
			format := "2006-01-02 15:04:05.000"
			if part.param != "" {
				format = part.param
			}
			buf.WriteString(entry.Time.Format(format))
		case "p":
			buf.WriteString(entry.Level.String())
		case "c":
			buf.WriteString(entry.Logger)
		case "m":
			buf.WriteString(entry.Message)
		case "n":
			buf.WriteString("\n")
		case "F":
			buf.WriteString(entry.Caller.File)
		case "L":
			buf.WriteString(fmt.Sprintf("%d", entry.Caller.Line))
		case "M":
			buf.WriteString(entry.Caller.Function)
		case "marker":
			buf.WriteString(entry.Marker)
		case "X":
			if part.param != "" {
				if val, ok := entry.Context[part.param]; ok {
					buf.WriteString(fmt.Sprintf("%v", val))
				}
			}
		case "t":
			buf.WriteString(fmt.Sprintf("%d", time.Now().UnixNano()))
		default:
			buf.WriteString("%" + part.variable)
		}
	}

	return buf.Bytes()
}

// JSONLayout formats logs as JSON
type JSONLayout struct {
	Pretty     bool
	TimeFormat string
}

// NewJSONLayout creates a new JSON layout
func NewJSONLayout() *JSONLayout {
	return &JSONLayout{
		Pretty:     false,
		TimeFormat: time.RFC3339Nano,
	}
}

// WithPretty enables pretty printing
func (j *JSONLayout) WithPretty(pretty bool) *JSONLayout {
	j.Pretty = pretty
	return j
}

// WithTimeFormat sets the time format
func (j *JSONLayout) WithTimeFormat(format string) *JSONLayout {
	j.TimeFormat = format
	return j
}

// Format converts entry to JSON
func (j *JSONLayout) Format(entry *Entry) []byte {
	data := map[string]interface{}{
		"timestamp": entry.Time.Format(j.TimeFormat),
		"level":     entry.Level.String(),
		"logger":    entry.Logger,
		"message":   entry.Message,
		"file":      entry.Caller.File,
		"line":      entry.Caller.Line,
	}

	if entry.Marker != "" {
		data["marker"] = entry.Marker
	}

	if len(entry.Context) > 0 {
		data["context"] = entry.Context
	}

	if len(entry.Fields) > 0 {
		for k, v := range entry.Fields {
			data[k] = v
		}
	}

	if entry.Error != nil {
		data["error"] = entry.Error.Error()
	}

	var result []byte
	var err error
	if j.Pretty {
		result, err = json.MarshalIndent(data, "", "  ")
	} else {
		result, err = json.Marshal(data)
	}

	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"marshal failed: %v"}`, err))
	}

	return append(result, '\n')
}

// TextLayout is a simple text formatter
type TextLayout struct {
	TimeFormat string
	ShowCaller bool
	ShowLevel  bool
	LevelWidth int
	Separator  string
}

// NewTextLayout creates a simple text layout
func NewTextLayout() *TextLayout {
	return &TextLayout{
		TimeFormat: "2006/01/02 15:04:05.000",
		ShowCaller: true,
		ShowLevel:  true,
		LevelWidth: 5,
		Separator:  " ",
	}
}

// WithTimeFormat sets the time format
func (t *TextLayout) WithTimeFormat(format string) *TextLayout {
	t.TimeFormat = format
	return t
}

// WithCaller enables/disables caller info
func (t *TextLayout) WithCaller(show bool) *TextLayout {
	t.ShowCaller = show
	return t
}

// Format converts entry to text
func (t *TextLayout) Format(entry *Entry) []byte {
	var parts []string

	// Timestamp
	parts = append(parts, entry.Time.Format(t.TimeFormat))

	// Caller
	if t.ShowCaller {
		parts = append(parts, fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line))
	}

	// Level
	if t.ShowLevel {
		level := entry.Level.String()
		if t.LevelWidth > 0 {
			level = fmt.Sprintf("%-*s", t.LevelWidth, level)
		}
		parts = append(parts, "["+strings.TrimSpace(level)+"]")
	}

	// Marker
	if entry.Marker != "" {
		parts = append(parts, "["+entry.Marker+"]")
	}

	// Message
	parts = append(parts, entry.Message)

	return []byte(strings.Join(parts, t.Separator) + "\n")
}

// ColoredLayout adds ANSI colors to text output
type ColoredLayout struct {
	inner Layout
}

// NewColoredLayout wraps a layout with colors
func NewColoredLayout(inner Layout) *ColoredLayout {
	return &ColoredLayout{inner: inner}
}

var levelColors = map[Level]string{
	TRACE: "\033[90m", // Gray
	DEBUG: "\033[36m", // Cyan
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
	FATAL: "\033[35m", // Magenta
}

const colorReset = "\033[0m"

// Format adds color codes
func (c *ColoredLayout) Format(entry *Entry) []byte {
	result := c.inner.Format(entry)
	color := levelColors[entry.Level]
	if color != "" {
		return []byte(color + string(result) + colorReset)
	}
	return result
}
