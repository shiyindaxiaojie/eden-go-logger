package logger

import (
	"os"
	"testing"
)

// BenchmarkSyncLogger benchmarks synchronous file logging
func BenchmarkSyncLogger(b *testing.B) {
	// Create a temporary file
	file, err := os.CreateTemp("", "bench-sync-*.log")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(file.Name()) // Clean up

	// Sync Appender
	appender := NewFileAppender(file.Name())

	// Logger
	log := NewLogger("SyncBench")
	log.AddAppender(appender)
	log.SetLevel(INFO)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("This is a benchmark log message %d", i)
	}
}

// BenchmarkAsyncLogger benchmarks asynchronous logging
func BenchmarkAsyncLogger(b *testing.B) {
	// Create a temporary file
	file, err := os.CreateTemp("", "bench-async-*.log")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(file.Name()) // Clean up

	// Async Appender (Buffer 4096)
	fileAppender := NewFileAppender(file.Name())
	appender := NewAsyncAppender(fileAppender, 4096)
	defer appender.Close() // Ensure flush

	// Logger
	log := NewLogger("AsyncBench")
	log.AddAppender(appender)
	log.SetLevel(INFO)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("This is a benchmark log message %d", i)
	}
}

// BenchmarkDiscard benchmarks pure logger overhead (no I/O)
func BenchmarkDiscard(b *testing.B) {
	appender := NewNullAppender()
	log := NewLogger("DiscardBench")
	log.AddAppender(appender)
	log.SetLevel(INFO)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("This is a benchmark log message %d", i)
	}
}
