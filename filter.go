package logger

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// FilterResult represents the decision of a filter
type FilterResult int

const (
	ACCEPT  FilterResult = iota // Accept the log event
	DENY                        // Deny the log event
	NEUTRAL                     // Neutral, let other filters decide
)

// Filter decides whether a log entry should be processed
type Filter interface {
	Decide(entry *Entry) FilterResult
}

// LevelFilter filters based on log level
type LevelFilter struct {
	minLevel   Level
	maxLevel   Level
	onMatch    FilterResult
	onMismatch FilterResult
}

// NewLevelFilter creates a filter that accepts logs at or above minLevel
func NewLevelFilter(minLevel Level) *LevelFilter {
	return &LevelFilter{
		minLevel:   minLevel,
		maxLevel:   OFF,
		onMatch:    ACCEPT,
		onMismatch: DENY,
	}
}

// WithMaxLevel sets the maximum level
func (f *LevelFilter) WithMaxLevel(maxLevel Level) *LevelFilter {
	f.maxLevel = maxLevel
	return f
}

// WithOnMatch sets the result when filter matches
func (f *LevelFilter) WithOnMatch(result FilterResult) *LevelFilter {
	f.onMatch = result
	return f
}

// WithOnMismatch sets the result when filter doesn't match
func (f *LevelFilter) WithOnMismatch(result FilterResult) *LevelFilter {
	f.onMismatch = result
	return f
}

// Decide implements Filter
func (f *LevelFilter) Decide(entry *Entry) FilterResult {
	if entry.Level >= f.minLevel && (f.maxLevel == OFF || entry.Level <= f.maxLevel) {
		return f.onMatch
	}
	return f.onMismatch
}

// RegexFilter filters based on message pattern
type RegexFilter struct {
	pattern    *regexp.Regexp
	onMatch    FilterResult
	onMismatch FilterResult
}

// NewRegexFilter creates a filter based on a regex pattern
func NewRegexFilter(pattern string) (*RegexFilter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexFilter{
		pattern:    re,
		onMatch:    ACCEPT,
		onMismatch: NEUTRAL,
	}, nil
}

// MustRegexFilter creates a filter, panics on invalid pattern
func MustRegexFilter(pattern string) *RegexFilter {
	f, err := NewRegexFilter(pattern)
	if err != nil {
		panic(err)
	}
	return f
}

// WithOnMatch sets the result when filter matches
func (f *RegexFilter) WithOnMatch(result FilterResult) *RegexFilter {
	f.onMatch = result
	return f
}

// WithOnMismatch sets the result when filter doesn't match
func (f *RegexFilter) WithOnMismatch(result FilterResult) *RegexFilter {
	f.onMismatch = result
	return f
}

// Decide implements Filter
func (f *RegexFilter) Decide(entry *Entry) FilterResult {
	if f.pattern.MatchString(entry.Message) {
		return f.onMatch
	}
	return f.onMismatch
}

// MarkerFilter filters based on marker
type MarkerFilter struct {
	marker     string
	onMatch    FilterResult
	onMismatch FilterResult
}

// NewMarkerFilter creates a filter for a specific marker
func NewMarkerFilter(marker string) *MarkerFilter {
	return &MarkerFilter{
		marker:     marker,
		onMatch:    ACCEPT,
		onMismatch: NEUTRAL,
	}
}

// WithOnMatch sets the result when filter matches
func (f *MarkerFilter) WithOnMatch(result FilterResult) *MarkerFilter {
	f.onMatch = result
	return f
}

// WithOnMismatch sets the result when filter doesn't match
func (f *MarkerFilter) WithOnMismatch(result FilterResult) *MarkerFilter {
	f.onMismatch = result
	return f
}

// Decide implements Filter
func (f *MarkerFilter) Decide(entry *Entry) FilterResult {
	if strings.EqualFold(entry.Marker, f.marker) {
		return f.onMatch
	}
	return f.onMismatch
}

// CompositeFilter combines multiple filters
type CompositeFilter struct {
	filters []Filter
	mode    CompositeMode
}

// CompositeMode defines how multiple filters are combined
type CompositeMode int

const (
	ALL CompositeMode = iota // All filters must accept
	ANY                      // Any filter accepting is enough
)

// NewCompositeFilter creates a composite filter
func NewCompositeFilter(mode CompositeMode, filters ...Filter) *CompositeFilter {
	return &CompositeFilter{
		filters: filters,
		mode:    mode,
	}
}

// Add adds a filter to the composite
func (f *CompositeFilter) Add(filter Filter) *CompositeFilter {
	f.filters = append(f.filters, filter)
	return f
}

// Decide implements Filter
func (f *CompositeFilter) Decide(entry *Entry) FilterResult {
	if len(f.filters) == 0 {
		return NEUTRAL
	}

	switch f.mode {
	case ALL:
		for _, filter := range f.filters {
			result := filter.Decide(entry)
			if result == DENY {
				return DENY
			}
		}
		return ACCEPT
	case ANY:
		for _, filter := range f.filters {
			result := filter.Decide(entry)
			if result == ACCEPT {
				return ACCEPT
			}
		}
		return DENY
	}

	return NEUTRAL
}

// ThresholdFilter is an alias for LevelFilter (log4j2 compatibility)
type ThresholdFilter = LevelFilter

// NewThresholdFilter creates a threshold filter
func NewThresholdFilter(level Level) *ThresholdFilter {
	return NewLevelFilter(level)
}

// DenyAllFilter denies all log events
type DenyAllFilter struct{}

func (f *DenyAllFilter) Decide(entry *Entry) FilterResult {
	return DENY
}

// AcceptAllFilter accepts all log events
type AcceptAllFilter struct{}

func (f *AcceptAllFilter) Decide(entry *Entry) FilterResult {
	return ACCEPT
}

// BurstFilter limits the rate of log events
type BurstFilter struct {
	level      Level
	rate       float64
	maxBurst   int
	onMatch    FilterResult
	onMismatch FilterResult

	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewBurstFilter creates a new burst filter
func NewBurstFilter(level Level, rate float64, maxBurst int) *BurstFilter {
	return &BurstFilter{
		level:      level,
		rate:       rate,
		maxBurst:   maxBurst,
		onMatch:    ACCEPT,
		onMismatch: DENY,
		tokens:     float64(maxBurst),
		lastRefill: time.Now(),
	}
}

// WithOnMatch sets the result when filter matches (allowed)
func (f *BurstFilter) WithOnMatch(result FilterResult) *BurstFilter {
	f.onMatch = result
	return f
}

// WithOnMismatch sets the result when filter doesn't match (rate exhausted)
func (f *BurstFilter) WithOnMismatch(result FilterResult) *BurstFilter {
	f.onMismatch = result
	return f
}

// Decide implements Filter
func (f *BurstFilter) Decide(entry *Entry) FilterResult {
	if entry.Level < f.level {
		// If level is lower than threshold, this filter doesn't apply (passes neutral)
		// Or logic: user said "BurstFilter level: INFO". Usually means limit INFO logs.
		// Log4j2 BurstFilter: "The BurstFilter provides a mechanism to control the rate at which logs are processed by calling appenders."
		// "Events below the level are passed neutrally."
		return NEUTRAL
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(f.lastRefill).Seconds()
	f.tokens += elapsed * f.rate
	if f.tokens > float64(f.maxBurst) {
		f.tokens = float64(f.maxBurst)
	}
	f.lastRefill = now

	if f.tokens >= 1 {
		f.tokens--
		return f.onMatch
	}
	return f.onMismatch
}

// ParseFilter creates a filter from configuration map
func ParseFilter(config map[string]interface{}) Filter {
	if config == nil {
		return nil
	}

	typ, ok := config["type"].(string)
	if !ok {
		return nil
	}

	var onMatch = ACCEPT
	var onMismatch = DENY

	if s, ok := config["on_match"].(string); ok {
		onMatch = parseFilterResult(s)
	}
	if s, ok := config["on_mismatch"].(string); ok {
		onMismatch = parseFilterResult(s)
	}

	switch strings.ToLower(typ) {
	case "marker":
		if marker, ok := config["marker"].(string); ok {
			return NewMarkerFilter(marker).WithOnMatch(onMatch).WithOnMismatch(onMismatch)
		}
	case "level", "threshold":
		if levelStr, ok := config["level"].(string); ok {
			return NewThresholdFilter(ParseLevel(levelStr)).WithOnMatch(onMatch).WithOnMismatch(onMismatch)
		}
	case "burst":
		levelStr, _ := config["level"].(string)
		level := ParseLevel(levelStr)
		rate, _ := config["rate"].(float64)
		if rate == 0 {
			// Try int conversion if float fails (json unmarshal numbers are floats, but manual map might be int)
			if r, ok := config["rate"].(int); ok {
				rate = float64(r)
			}
		}
		maxBurst, _ := config["max_burst"].(int)
		if maxBurst == 0 {
			if m, ok := config["max_burst"].(float64); ok {
				maxBurst = int(m)
			}
		}
		return NewBurstFilter(level, rate, maxBurst).WithOnMatch(onMatch).WithOnMismatch(onMismatch)
	}
	return nil
}

func parseFilterResult(s string) FilterResult {
	switch strings.ToUpper(s) {
	case "ACCEPT":
		return ACCEPT
	case "DENY":
		return DENY
	case "NEUTRAL":
		return NEUTRAL
	}
	return NEUTRAL
}
