package logger

import (
	"io"
	"os"
	"sync"
)

// Appender writes log entries to a destination
type Appender interface {
	Name() string
	Append(entry *Entry) error
	Close() error
}

// BaseAppender provides common functionality for appenders
type BaseAppender struct {
	name   string
	layout Layout
	filter Filter
	mu     sync.Mutex
}

// applyFilter checks if entry should be logged
func (b *BaseAppender) applyFilter(entry *Entry) bool {
	if b.filter == nil {
		return true
	}
	result := b.filter.Decide(entry)
	return result != DENY
}

// ConsoleAppender writes to stdout or stderr
type ConsoleAppender struct {
	BaseAppender
	writer io.Writer
	target string // "stdout" or "stderr"
}

// NewConsoleAppender creates a console appender writing to stdout
func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{
		BaseAppender: BaseAppender{
			name:   "Console",
			layout: NewTextLayout(),
		},
		writer: os.Stdout,
		target: "stdout",
	}
}

// WithName sets the appender name
func (c *ConsoleAppender) WithName(name string) *ConsoleAppender {
	c.name = name
	return c
}

// WithLayout sets the layout
func (c *ConsoleAppender) WithLayout(layout Layout) *ConsoleAppender {
	c.layout = layout
	return c
}

// WithFilter sets the filter
func (c *ConsoleAppender) WithFilter(filter Filter) *ConsoleAppender {
	c.filter = filter
	return c
}

// WithTarget sets output target (stdout/stderr)
func (c *ConsoleAppender) WithTarget(target string) *ConsoleAppender {
	c.target = target
	if target == "stderr" {
		c.writer = os.Stderr
	} else {
		c.writer = os.Stdout
	}
	return c
}

// FilterLevel sets a threshold filter for this appender
func (c *ConsoleAppender) FilterLevel(level string) *ConsoleAppender {
	return c.WithFilter(NewThresholdFilter(ParseLevel(level)))
}

// Pattern sets the layout pattern
func (c *ConsoleAppender) Pattern(pattern string) *ConsoleAppender {
	return c.WithLayout(NewPatternLayout(pattern))
}

// FilterMap sets the filter from a map configuration
func (c *ConsoleAppender) FilterMap(config map[string]interface{}) *ConsoleAppender {
	return c.WithFilter(ParseFilter(config))
}

// Name returns the appender name
func (c *ConsoleAppender) Name() string {
	return c.name
}

// Append writes a log entry
func (c *ConsoleAppender) Append(entry *Entry) error {
	if !c.applyFilter(entry) {
		return nil
	}

	data := c.layout.Format(entry)

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.writer.Write(data)
	return err
}

// Close does nothing for console
func (c *ConsoleAppender) Close() error {
	return nil
}

// FileAppender writes to a file
type FileAppender struct {
	BaseAppender
	file     *os.File
	filename string
	append   bool
}

// NewFileAppender creates a file appender
func NewFileAppender(filename string) *FileAppender {
	return &FileAppender{
		BaseAppender: BaseAppender{
			name:   "File",
			layout: NewTextLayout(),
		},
		filename: filename,
		append:   true,
	}
}

// WithName sets the appender name
func (f *FileAppender) WithName(name string) *FileAppender {
	f.name = name
	return f
}

// WithLayout sets the layout
func (f *FileAppender) WithLayout(layout Layout) *FileAppender {
	f.layout = layout
	return f
}

// WithFilter sets the filter
func (f *FileAppender) WithFilter(filter Filter) *FileAppender {
	f.filter = filter
	return f
}

// WithAppend sets whether to append to existing file
func (f *FileAppender) WithAppend(append bool) *FileAppender {
	f.append = append
	return f
}

// open opens the file if not already open
func (f *FileAppender) open() error {
	if f.file != nil {
		return nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	if f.append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(f.filename, flags, 0644)
	if err != nil {
		return err
	}
	f.file = file
	return nil
}

// Name returns the appender name
func (f *FileAppender) Name() string {
	return f.name
}

// Append writes a log entry
func (f *FileAppender) Append(entry *Entry) error {
	if !f.applyFilter(entry) {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.open(); err != nil {
		return err
	}

	data := f.layout.Format(entry)
	_, err := f.file.Write(data)
	return err
}

// Close closes the file
func (f *FileAppender) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file != nil {
		err := f.file.Close()
		f.file = nil
		return err
	}
	return nil
}

// WriterAppender writes to any io.Writer
type WriterAppender struct {
	BaseAppender
	writer io.Writer
}

// NewWriterAppender creates an appender for any io.Writer
func NewWriterAppender(name string, writer io.Writer) *WriterAppender {
	return &WriterAppender{
		BaseAppender: BaseAppender{
			name:   name,
			layout: NewTextLayout(),
		},
		writer: writer,
	}
}

// WithLayout sets the layout
func (w *WriterAppender) WithLayout(layout Layout) *WriterAppender {
	w.layout = layout
	return w
}

// WithFilter sets the filter
func (w *WriterAppender) WithFilter(filter Filter) *WriterAppender {
	w.filter = filter
	return w
}

// Name returns the appender name
func (w *WriterAppender) Name() string {
	return w.name
}

// Append writes a log entry
func (w *WriterAppender) Append(entry *Entry) error {
	if !w.applyFilter(entry) {
		return nil
	}

	data := w.layout.Format(entry)

	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := w.writer.Write(data)
	return err
}

// Close does nothing for generic writer
func (w *WriterAppender) Close() error {
	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// NullAppender discards all output (useful for testing)
type NullAppender struct {
	name string
}

func NewNullAppender() *NullAppender {
	return &NullAppender{name: "Null"}
}

func (n *NullAppender) Name() string {
	return n.name
}

func (n *NullAppender) Append(entry *Entry) error {
	return nil
}

func (n *NullAppender) Close() error {
	return nil
}
