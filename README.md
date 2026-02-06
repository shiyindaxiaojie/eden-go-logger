# Eden Go Logger

[![Go Report Card](https://goreportcard.com/badge/github.com/shiyindaxiaojie/eden-go-logger)](https://goreportcard.com/report/github.com/shiyindaxiaojie/eden-go-logger) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

English | [中文](README_zh-CN.md)

A high-performance, zero-dependency logging library for Go. Inspired by Log4j2, it is designed for enterprise-grade applications, supporting async writing, log rotation, and advanced filtering.

## 🌟 Key Features

- **High Performance Async IO**: Built-in `AsyncAppender` uses native Channels + Workers for lock-free async writing, ensuring minimal impact on business performance.
- **Enterprise Log Rotation**: Auto-rotation based on size (`20MB`) or time (`0 0 4 * * ?` Cron expression), with gzip compression.
- **Powerful Filters**:
    - **Level**: Standard severity filtering.
    - **Marker**: Business tagging (e.g., `AccessLog`, `SQL`) for log routing.
    - **Burst**: Rate limiting to prevent disk flooding during error storms.
- **Elegant Configuration**:
    - **Fluent Builder API**: Type-safe method chaining.
    - **YAML/JSON Config**: Hot-loadable from standard config files.
- **Context Aware**: Supports MDC (Mapped Diagnostic Context) for distributed tracing.

## 📦 Installation

```bash
go get github.com/shiyindaxiaojie/eden-go-logger
```

## 🚀 Quick Start

### 1. Basic Usage

```go
package main

import "github.com/shiyindaxiaojie/eden-go-logger"

func main() {
    // Default outputs to Console
    logger.Info("Hello, Eden Logger!")
    logger.Error("Something went wrong: %v", "timeout")
}
```

### 2. Advanced Configuration (Fluent API)

Using the Builder pattern for production setups:

```go
func initLogger() {
    logger.NewBuilder().
        Level("INFO").
        // 1. Console for Dev
        Console(func(c *logger.ConsoleAppender) {
            c.Pattern("%d{2006-01-02 15:04:05} [%p] %c - %m%n")
        }).
        // 2. App Log (Async + Rolling + Compress)
        RollingFile("logs/app.log", func(f *logger.RollingFileAppender) {
            f.WithName("AppLog")
            f.SizePolicy("100MB")
            f.CronPolicy("0 0 0 * * ?")  // Daily rotation
            f.MaxBackups(30)
            f.Retention("30d")
            f.Compress(true)
        }).
        // 3. Enable Global Async Mode (Critical for perf)
        Async(4096).
        Init()
}
```

### 3. Config File Driver (YAML)

Ideal for Cloud-Native environments (ConfigMap):

```yaml
log:
    level: INFO
    appenders:
        - name: AppLog
          type: RollingFile
          file_name: "logs/app.log"
          file_pattern: "app-%i.log.gz"
          async: true # Enable Async
          rollover:
              max_file: 30
              retention: 30d
          filter: # Anti-Flood Protection
              type: burst
              level: ERROR
              rate: 10
              max_burst: 100
```

## 📖 Guide

### 1. Async Logging

In high-concurrency production, you **MUST** enable async logging. Eden Go Logger uses buffered channels to decouple I/O.

- **Config**: Use `.Async(bufferSize)` in Builder or `async: true` in YAML.
- **Benefit**: Reduces I/O latency from milliseconds to microseconds.

### 2. Log Routing (Marker)

Separate `Access Logs` from `App Logs` using **Markers**.

1.  **Configure Appender**:

    ```yaml
    - name: AccessLog
      file_name: "logs/access.log"
      filter:
          type: marker
          marker: API
          on_match: ACCEPT
          on_mismatch: DENY
    ```

2.  **Log with Marker**:
    ```go
    logger.WithMarker("API").Info("GET /users/1 200 OK")
    ```

### 3. Context Tracing (MDC)

Trace Request IDs across microservices:

```go
// Middleware
l := logger.WithContext("request_id", "req-123456")

// Business Logic
l.Info("Processing order") // Automatically includes request_id
```

Pattern Config:

```go
c.Pattern("%d [%p] [%X{request_id}] %m%n")
```

## 🧩 Performance Checks

Benchmark results on i5-1135G7 @ 2.40GHz:

- **Log Overload (Discard)**: ~1,500,000 ops/sec (692 ns/op) - Pure processing power.
- **Sync File IO**: ~120,000 ops/sec (8016 ns/op) - Dependent on disk speed.
- **Async File IO**: Provides microsecond-level write latency until buffer saturation.

> **Best Practice**: Always enable `async: true` in production with a sufficient buffer size (e.g., 4096) to handle traffic spikes and prevent unexpected I/O blocking.

## 📄 License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
