package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	service string
	level   Level
}

func New(service string) *Logger {
	level := LevelInfo
	if envLevel := strings.ToUpper(os.Getenv("LOG_LEVEL")); envLevel != "" {
		switch envLevel {
		case "DEBUG":
			level = LevelDebug
		case "INFO":
			level = LevelInfo
		case "WARN":
			level = LevelWarn
		case "ERROR":
			level = LevelError
		}
	}
	return &Logger{service: service, level: level}
}

type logEntry struct {
	Level     string    `json:"level"`
	Time      time.Time `json:"time"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	RequestID string    `json:"requestId,omitempty"`
	MatchID   string    `json:"matchId,omitempty"`
}

func (l *Logger) log(level Level, msg string, fields ...any) {
	if level < l.level {
		return
	}
	entry := logEntry{
		Level:   level.String(),
		Time:    time.Now().UTC(),
		Service: l.service,
		Message: msg,
	}
	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		switch key {
		case "error":
			entry.Error = fmt.Sprint(fields[i+1])
		case "requestId":
			entry.RequestID = fmt.Sprint(fields[i+1])
		case "matchId":
			entry.MatchID = fmt.Sprint(fields[i+1])
		}
	}
	data, _ := json.Marshal(entry)
	_ = log.Output(2, string(data))
}

func (l *Logger) Debug(msg string, fields ...any) {
	l.log(LevelDebug, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...any) {
	l.log(LevelInfo, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...any) {
	l.log(LevelWarn, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...any) {
	l.log(LevelError, msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...any) {
	l.log(LevelError, msg, fields...)
	os.Exit(1)
}
