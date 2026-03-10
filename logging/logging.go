package logging

import (
	"encoding/json"
	"os"
	"time"
)

type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.write("info", msg, fields)
}

func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.write("error", msg, fields)
}

func (l *Logger) write(level, msg string, fields map[string]interface{}) {
	entry := map[string]interface{}{
		"level": level,
		"msg":   msg,
		"time":  time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range fields {
		entry[k] = v
	}
	json.NewEncoder(os.Stdout).Encode(entry)
}
