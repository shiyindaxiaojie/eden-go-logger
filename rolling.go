package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RollingPolicy determines when to roll the log file
type RollingPolicy interface {
	ShouldRoll(entry *Entry, fileInfo os.FileInfo) bool
	GetNextFileName(baseName string, index int) string
}

// TriggeringPolicy determines when to trigger a rollover
type TriggeringPolicy interface {
	ShouldTrigger(entry *Entry, file *os.File) bool
}

// SizeBasedPolicy triggers rollover based on file size
type SizeBasedPolicy struct {
	maxSize int64 // in bytes
}

// NewSizeBasedPolicy creates a size-based rolling policy
// size is in bytes
func NewSizeBasedPolicy(maxBytes int64) *SizeBasedPolicy {
	return &SizeBasedPolicy{maxSize: maxBytes}
}

// ShouldRoll implements RollingPolicy
func (p *SizeBasedPolicy) ShouldRoll(entry *Entry, fileInfo os.FileInfo) bool {
	if fileInfo == nil {
		return false
	}
	return fileInfo.Size() >= p.maxSize
}

// GetNextFileName implements RollingPolicy
func (p *SizeBasedPolicy) GetNextFileName(baseName string, index int) string {
	ext := filepath.Ext(baseName)
	name := baseName[:len(baseName)-len(ext)]
	return fmt.Sprintf("%s.%d%s", name, index, ext)
}

// TimeBasedPolicy triggers rollover based on time
type TimeBasedPolicy struct {
	interval time.Duration
	pattern  string // date pattern for file naming
	lastRoll time.Time
}

// NewTimeBasedPolicy creates a time-based rolling policy
// interval examples: "hourly", "daily", "weekly"
func NewTimeBasedPolicy(interval string) *TimeBasedPolicy {
	var d time.Duration
	var pattern string

	switch interval {
	case "hourly":
		d = time.Hour
		pattern = "2006-01-02-15"
	case "daily":
		d = 24 * time.Hour
		pattern = "2006-01-02"
	case "weekly":
		d = 7 * 24 * time.Hour
		pattern = "2006-01-02"
	default:
		d = 24 * time.Hour
		pattern = "2006-01-02"
	}

	return &TimeBasedPolicy{
		interval: d,
		pattern:  pattern,
		lastRoll: time.Now(),
	}
}

// ShouldRoll implements RollingPolicy
func (p *TimeBasedPolicy) ShouldRoll(entry *Entry, fileInfo os.FileInfo) bool {
	return time.Since(p.lastRoll) >= p.interval
}

// GetNextFileName implements RollingPolicy
func (p *TimeBasedPolicy) GetNextFileName(baseName string, index int) string {
	ext := filepath.Ext(baseName)
	name := baseName[:len(baseName)-len(ext)]
	timestamp := time.Now().Format(p.pattern)
	p.lastRoll = time.Now()
	return fmt.Sprintf("%s.%s%s", name, timestamp, ext)
}

// CronBasedPolicy triggers rollover based on a simplified cron schedule
// Supports "0 0 H * * ?" format (daily at hour H)
type CronBasedPolicy struct {
	schedule string
	hour     int // Hour to trigger (parsed from schedule)
	lastRoll time.Time
}

// NewCronBasedPolicy creates a cron-based rolling policy
// schedule format: "0 0 H * * ?" where H is the hour (0-23)
func NewCronBasedPolicy(schedule string) *CronBasedPolicy {
	hour := parseCronHour(schedule)
	return &CronBasedPolicy{
		schedule: schedule,
		hour:     hour,
		lastRoll: time.Now(),
	}
}

// parseCronHour extracts the hour from cron expression "0 0 H * * ?"
func parseCronHour(schedule string) int {
	parts := strings.Fields(schedule)
	if len(parts) >= 3 {
		var h int
		fmt.Sscanf(parts[2], "%d", &h)
		return h
	}
	return 4 // Default to 4 AM
}

// ShouldRoll implements RollingPolicy
func (p *CronBasedPolicy) ShouldRoll(entry *Entry, fileInfo os.FileInfo) bool {
	now := time.Now()
	// Check if we've crossed the target hour since last roll
	// Roll if: current hour matches target AND we haven't rolled today
	if now.Hour() == p.hour {
		// Check if last roll was before today's target time
		targetToday := time.Date(now.Year(), now.Month(), now.Day(), p.hour, 0, 0, 0, now.Location())
		if p.lastRoll.Before(targetToday) {
			p.lastRoll = now
			return true
		}
	}
	return false
}

// GetNextFileName implements RollingPolicy
func (p *CronBasedPolicy) GetNextFileName(baseName string, index int) string {
	ext := filepath.Ext(baseName)
	name := baseName[:len(baseName)-len(ext)]
	timestamp := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s.%s%s", name, timestamp, ext)
}

// CompositeTriggeringPolicy combines multiple policies (any triggers = roll)
type CompositeTriggeringPolicy struct {
	policies []RollingPolicy
}

// NewCompositeTriggeringPolicy creates a composite policy
func NewCompositeTriggeringPolicy(policies ...RollingPolicy) *CompositeTriggeringPolicy {
	return &CompositeTriggeringPolicy{policies: policies}
}

// ShouldRoll returns true if any policy triggers
func (p *CompositeTriggeringPolicy) ShouldRoll(entry *Entry, fileInfo os.FileInfo) bool {
	for _, policy := range p.policies {
		if policy.ShouldRoll(entry, fileInfo) {
			return true
		}
	}
	return false
}

// RollingFileAppender writes logs with automatic file rotation
type RollingFileAppender struct {
	BaseAppender
	filename     string
	file         *os.File
	policies     []RollingPolicy
	maxBackups   int           // max number of backup files to keep
	maxAge       time.Duration // max age of backup files
	totalMaxSize int64         // max total size of all log files
	currentIndex int
}

// NewRollingFileAppender creates a rolling file appender
func NewRollingFileAppender(filename string) *RollingFileAppender {
	return &RollingFileAppender{
		BaseAppender: BaseAppender{
			name:   "RollingFile",
			layout: NewTextLayout(),
		},
		filename:   filename,
		maxBackups: 7,
		policies:   make([]RollingPolicy, 0),
	}
}

// WithName sets the appender name
func (r *RollingFileAppender) WithName(name string) *RollingFileAppender {
	r.name = name
	return r
}

// WithLayout sets the layout
func (r *RollingFileAppender) WithLayout(layout Layout) *RollingFileAppender {
	r.layout = layout
	return r
}

// WithFilter sets the filter
func (r *RollingFileAppender) WithFilter(filter Filter) *RollingFileAppender {
	r.filter = filter
	return r
}

// WithPolicy adds a rolling policy
func (r *RollingFileAppender) WithPolicy(policy RollingPolicy) *RollingFileAppender {
	r.policies = append(r.policies, policy)
	return r
}

// WithMaxBackups sets max number of backup files
func (r *RollingFileAppender) WithMaxBackups(max int) *RollingFileAppender {
	r.maxBackups = max
	return r
}

// WithMaxAge sets max age of backup files
func (r *RollingFileAppender) WithMaxAge(age time.Duration) *RollingFileAppender {
	r.maxAge = age
	return r
}

// WithTotalMaxSize sets max total size of all log files
func (r *RollingFileAppender) WithTotalMaxSize(maxBytes int64) *RollingFileAppender {
	r.totalMaxSize = maxBytes
	return r
}

// Retention sets max age of backup files using string duration (e.g., "7d")
func (r *RollingFileAppender) Retention(durationStr string) *RollingFileAppender {
	r.maxAge = parseDuration(durationStr)
	return r
}

// MaxBackups sets max number of backup files
func (r *RollingFileAppender) MaxBackups(max int) *RollingFileAppender {
	r.maxBackups = max
	return r
}

// SizePolicy adds a size-based triggering policy
func (r *RollingFileAppender) SizePolicy(sizeStr string) *RollingFileAppender {
	size := parseSize(sizeStr)
	return r.WithPolicy(NewSizeBasedPolicy(size))
}

// CronPolicy adds a cron-based triggering policy
func (r *RollingFileAppender) CronPolicy(schedule string) *RollingFileAppender {
	return r.WithPolicy(NewCronBasedPolicy(schedule))
}

// FilterLevel sets a threshold filter for this appender
func (r *RollingFileAppender) FilterLevel(level string) *RollingFileAppender {
	return r.WithFilter(NewThresholdFilter(ParseLevel(level)))
}

// FilterMap sets the filter from a map configuration
func (r *RollingFileAppender) FilterMap(config map[string]interface{}) *RollingFileAppender {
	return r.WithFilter(ParseFilter(config))
}

// Pattern sets the layout pattern
func (r *RollingFileAppender) Pattern(pattern string) *RollingFileAppender {
	return r.WithLayout(NewPatternLayout(pattern))
}

// Name returns the appender name
func (r *RollingFileAppender) Name() string {
	return r.name
}

// open opens the file if not already open
func (r *RollingFileAppender) open() error {
	if r.file != nil {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(r.filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(r.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	r.file = file
	return nil
}

// shouldRoll checks if any policy triggers a rollover
func (r *RollingFileAppender) shouldRoll(entry *Entry) bool {
	if r.file == nil {
		return false
	}
	if len(r.policies) == 0 {
		return false
	}

	fileInfo, err := r.file.Stat()
	if err != nil {
		return false
	}

	for _, policy := range r.policies {
		if policy.ShouldRoll(entry, fileInfo) {
			return true
		}
	}
	return false
}

// rollover performs the file rotation
func (r *RollingFileAppender) rollover() error {
	if r.file == nil {
		return nil
	}

	// Close current file
	r.file.Close()
	r.file = nil

	// Determine new file name
	r.currentIndex++
	var newName string
	if len(r.policies) > 0 {
		newName = r.policies[0].GetNextFileName(r.filename, r.currentIndex)
	} else {
		newName = fmt.Sprintf("%s.%d", r.filename, r.currentIndex)
	}

	// Rename current to backup
	if err := os.Rename(r.filename, newName); err != nil {
		// If rename fails, try to reopen original
		r.open()
		return err
	}

	// Clean up old backups
	r.cleanup()

	// Open new file
	return r.open()
}

// cleanup removes old backup files
func (r *RollingFileAppender) cleanup() {
	if r.maxBackups <= 0 && r.totalMaxSize <= 0 {
		return
	}

	dir := filepath.Dir(r.filename)
	base := filepath.Base(r.filename)

	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Find matching backup files
	type backupFile struct {
		name    string
		path    string
		modTime time.Time
		size    int64
	}
	var backups []backupFile

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		// Check if it's a backup of our log file
		if len(name) > len(base) && name[:len(base)] == base {
			info, err := f.Info()
			if err != nil {
				continue
			}
			backups = append(backups, backupFile{
				name:    name,
				path:    filepath.Join(dir, name),
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
	}

	// Sort by modification time (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.Before(backups[j].modTime)
	})

	// Remove excess files by count
	for len(backups) > r.maxBackups && r.maxBackups > 0 {
		os.Remove(backups[0].path)
		backups = backups[1:]
	}

	// Remove files by age
	if r.maxAge > 0 {
		expirationTime := time.Now().Add(-r.maxAge)
		var validBackups []backupFile
		for _, b := range backups {
			if b.modTime.Before(expirationTime) {
				os.Remove(b.path)
			} else {
				validBackups = append(validBackups, b)
			}
		}
		backups = validBackups
	}

	// Remove files to stay under total size limit
	if r.totalMaxSize > 0 {
		var totalSize int64
		for _, b := range backups {
			totalSize += b.size
		}
		for totalSize > r.totalMaxSize && len(backups) > 0 {
			totalSize -= backups[0].size
			os.Remove(backups[0].path)
			backups = backups[1:]
		}
	}
}

// Append writes a log entry
func (r *RollingFileAppender) Append(entry *Entry) error {
	if !r.applyFilter(entry) {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.open(); err != nil {
		return err
	}

	// Check if we need to roll
	if r.shouldRoll(entry) {
		if err := r.rollover(); err != nil {
			return err
		}
	}

	data := r.layout.Format(entry)
	_, err := r.file.Write(data)
	return err
}

// Close closes the file
func (r *RollingFileAppender) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}
