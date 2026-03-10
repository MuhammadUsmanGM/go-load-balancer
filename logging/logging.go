package logging

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
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

func (l *Logger) write(level, msg string, fields map[string]interface{}) {
	entry := make(map[string]interface{}, len(fields)+3)
	entry["time"] = time.Now().UTC().Format(time.RFC3339)
	entry["level"] = level
	entry["msg"] = msg
	for k, v := range fields {
		entry[k] = v
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	json.NewEncoder(l.out).Encode(entry)
}
