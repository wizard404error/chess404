package logging

import (
	"log/slog"
	"os"
	"strings"
)

type Logger struct {
	slogger *slog.Logger
}

func New(service string) *Logger {
	level := parseLevel()
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slogger := slog.New(handler).With("service", service)
	return &Logger{slogger: slogger}
}

func parseLevel() slog.Level {
	switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l *Logger) Debug(msg string, fields ...any) {
	l.slogger.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...any) {
	l.slogger.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...any) {
	l.slogger.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...any) {
	l.slogger.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...any) {
	l.slogger.Error(msg, fields...)
	os.Exit(1)
}
