package logger

import (
	"fmt"
	"sync"
)

// AsyncAppender wraps an Appender to write logs asynchronously
type AsyncAppender struct {
	delegate Appender
	msgChan  chan *Entry
	wg       sync.WaitGroup
	once     sync.Once
}

// NewAsyncAppender creates a new AsyncAppender
func NewAsyncAppender(delegate Appender, bufferSize int) *AsyncAppender {
	if bufferSize <= 0 {
		bufferSize = 4096 // Default buffer size, robust enough for high load
	}

	a := &AsyncAppender{
		delegate: delegate,
		msgChan:  make(chan *Entry, bufferSize),
	}

	a.wg.Add(1)
	go a.worker()

	return a
}

// Name returns the delegate appender's name
func (a *AsyncAppender) Name() string {
	return a.delegate.Name()
}

// Append pushes the entry to the channel
// It will BLOCK if the buffer is full to ensure no log loss (Reliability > Drop)
// For "Strongest", data integrity is usually preferred over dropping.
func (a *AsyncAppender) Append(entry *Entry) error {
	// Send to channel
	// Note: If channel is closed, this will panic. We ensure Close() happens after all Appends
	// or we accept panic as "program is shutting down incorrectly".
	// But to be safe in Go, usually strictly controlled lifecycle.

	// Optimization: We could use a non-blocking select for "Drop" strategy,
	// but user asked for "Strongest" which usually implies "Best", and losing logs is bad.
	// We sticking to blocking to guarantee delivery.
	a.msgChan <- entry
	return nil
}

// Close closes the channel and waits for the worker to finish
func (a *AsyncAppender) Close() error {
	var err error
	a.once.Do(func() {
		close(a.msgChan)
		a.wg.Wait()
		err = a.delegate.Close()
	})
	return err
}

func (a *AsyncAppender) worker() {
	defer a.wg.Done()

	for entry := range a.msgChan {
		// We could implement batching here for even more performance if the delegate supports it.
		// For now, simple forwarding is already huge improvement over sync.
		err := a.delegate.Append(entry)
		if err != nil {
			// Fallback? Print to stderr?
			fmt.Printf("AsyncAppender: failed to write log: %v\n", err)
		}
	}
}
