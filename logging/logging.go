package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-load-balancer/middleware"
)

// Logger provides structured JSON logging with thread-safe output.
type Logger struct {
	mu  sync.Mutex
	out io.Writer
}

// NewLogger creates a logger that writes to stdout.
func NewLogger() *Logger {
	return &Logger{out: os.Stdout}
}

// Info logs a message at info level.
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.write("info", msg, fields)
}

// Error logs a message at error level.
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.write("error", msg, fields)
}

// WithRequestID logs a message with request ID from context.
func (l *Logger) WithRequestID(ctx context.Context, level, msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	
	// Extract request ID if context is provided
	if ctx != nil {
		if requestID := middleware.GetRequestID(ctx); requestID != "" {
			fields["request_id"] = requestID
		}
	}
	
	l.write(level, msg, fields)
}

func (l *Logger) write(level, msg string, fields map[string]interface{}) {
	entry := make(map[string]interface{}, len(fields)+3)
	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["level"] = level
	entry["message"] = msg
	entry["service"] = "go-load-balancer"
	for k, v := range fields {
		entry[k] = v
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	json.NewEncoder(l.out).Encode(entry)
}
