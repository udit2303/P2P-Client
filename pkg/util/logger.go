package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
)

// colorize adds ANSI color codes to the message based on level
func colorize(level slog.Level, msg string) string {
	switch level {
	case slog.LevelError:
		return colorRed + msg + colorReset
	case slog.LevelWarn:
		return colorYellow + msg + colorReset
	case slog.LevelInfo:
		return colorGreen + msg + colorReset
	case slog.LevelDebug:
		return colorCyan + msg + colorReset
	default:
		return colorWhite + msg + colorReset
	}
}

// Log level constants
const (
	DebugLevel = slog.LevelDebug
	InfoLevel  = slog.LevelInfo
	WarnLevel  = slog.LevelWarn
	ErrorLevel = slog.LevelError
)

type Logger struct {
	logger *slog.Logger
}

// NewLogger creates a new logger instance
func NewLogger(output io.Writer, level slog.Level) *Logger {
	// If output is os.Stdout or os.Stderr, use our custom colored console handler
	if output == os.Stdout || output == os.Stderr {
		handler := &consoleHandler{
			handler: slog.NewTextHandler(output, &slog.HandlerOptions{
				Level: level,
			}),
			level: level,
		}
		return &Logger{logger: slog.New(handler)}
	}

	// Otherwise, use JSON handler for other outputs (files, etc.)
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize the source location to be more concise
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				// Only show the filename, not the full path
				source.File = filepath.Base(source.File)
				a.Value = slog.AnyValue(source)
			}
			return a
		},
	})

	return &Logger{logger: slog.New(handler)}
}

// consoleHandler is a custom handler for colored console output
type consoleHandler struct {
	handler slog.Handler
	level   slog.Level
}

func (h *consoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format the message with color based on level
	levelStr := r.Level.String()
	switch r.Level {
	case slog.LevelError:
		levelStr = colorize(r.Level, "ERROR")
	case slog.LevelWarn:
		levelStr = colorize(r.Level, "WARN ")
	case slog.LevelInfo:
		levelStr = colorize(r.Level, "INFO ")
	case slog.LevelDebug:
		levelStr = colorize(r.Level, "DEBUG")
	}

	// Format the time
	timeStr := colorize(slog.LevelInfo, r.Time.Format("15:04:05.000"))

	// Build the message parts
	var msgParts []string

	// Add the main message with color
	msgParts = append(msgParts, fmt.Sprintf("%s %s %s", 
		timeStr,
		levelStr,
		r.Message,
	))

	// Add attributes if any
	r.Attrs(func(attr slog.Attr) bool {
		attrStr := fmt.Sprintf("%s=%v", attr.Key, attr.Value)
		switch {
		case attr.Key == "error":
			msgParts = append(msgParts, colorize(slog.LevelError, attrStr))
		case attr.Key == "file" || attr.Key == "path":
			msgParts = append(msgParts, colorize(slog.LevelDebug, attrStr))
		default:
			msgParts = append(msgParts, colorize(slog.LevelInfo, attrStr))
		}
		return true
	})

	// Join all parts and print
	fmt.Fprintln(os.Stdout, strings.Join(msgParts, " "))
	return nil
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &consoleHandler{
		handler: h.handler.WithAttrs(attrs),
		level:   h.level,
	}
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	return &consoleHandler{
		handler: h.handler.WithGroup(name),
		level:   h.level,
	}
}

// DefaultLogger creates a new logger with default settings
func DefaultLogger() *Logger {
	return NewLogger(os.Stdout, InfoLevel)
}

// With adds attributes to the logger
func (l *Logger) With(args ...interface{}) *Logger {
	return &Logger{
		logger: l.logger.With(args...),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, toAttrSlice(args)...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, toAttrSlice(args)...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, toAttrSlice(args)...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, toAttrSlice(args)...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.logger.Error(msg, toAttrSlice(args)...)
	os.Exit(1)
}

// WithRequestID adds a request ID to the logger
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.With("request_id", requestID)
}

// WithError adds an error to the logger
func (l *Logger) WithError(err error) *Logger {
	return l.With("error", err.Error())
}

// toAttrSlice converts key-value pairs to slog.Attr slice
func toAttrSlice(args []interface{}) []any {
	if len(args)%2 != 0 {
		args = append(args, "(MISSING)")
	}
	attrs := make([]any, 0, len(args))
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", args[i])
		}
		attrs = append(attrs, key, args[i+1])
	}
	return attrs
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(ctx context.Context, maxRetries int, initialBackoff time.Duration, fn func() error) error {
	var err error
	backoff := initialBackoff

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i == maxRetries-1 {
			break
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			backoff *= 2
		}
	}

	return fmt.Errorf("after %d attempts, last error: %w", maxRetries, err)
}

// GetCallerInfo returns the file and line number of the caller
func GetCallerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return file, line
}

// PrettyPrint prints any value as formatted JSON
func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}
