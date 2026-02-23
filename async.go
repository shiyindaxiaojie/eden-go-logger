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
		bufferSize = 4096
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
func (a *AsyncAppender) Append(entry *Entry) error {
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
		err := a.delegate.Append(entry)
		if err != nil {
			fmt.Printf("AsyncAppender: failed to write log: %v\n", err)
		}
	}
}
