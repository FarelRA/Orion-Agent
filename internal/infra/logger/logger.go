package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Level represents log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Colors for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// Logger implements waLog.Logger with colored output.
type Logger struct {
	module string
	level  Level
	output io.Writer
}

// New creates a new Logger.
func New(module string, level string) *Logger {
	return &Logger{
		module: module,
		level:  parseLevel(level),
		output: os.Stderr,
	}
}

// parseLevel converts string level to Level.
func parseLevel(level string) Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Sub creates a sub-logger with a new module name.
func (l *Logger) Sub(module string) waLog.Logger {
	newModule := module
	if l.module != "" {
		newModule = l.module + "/" + module
	}
	return &Logger{
		module: newModule,
		level:  l.level,
		output: l.output,
	}
}

// Debugf logs a debug message.
func (l *Logger) Debugf(msg string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.log(LevelDebug, msg, args...)
	}
}

// Infof logs an info message.
func (l *Logger) Infof(msg string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.log(LevelInfo, msg, args...)
	}
}

// Warnf logs a warning message.
func (l *Logger) Warnf(msg string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.log(LevelWarn, msg, args...)
	}
}

// Errorf logs an error message.
func (l *Logger) Errorf(msg string, args ...interface{}) {
	if l.level <= LevelError {
		l.log(LevelError, msg, args...)
	}
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05.000")
	levelStr, levelColor := l.levelString(level)

	formattedMsg := fmt.Sprintf(msg, args...)

	moduleStr := ""
	if l.module != "" {
		moduleStr = fmt.Sprintf("%s[%s]%s ", colorCyan, l.module, colorReset)
	}

	fmt.Fprintf(l.output, "%s%s%s %s%s%s %s%s\n",
		colorGray, timestamp, colorReset,
		levelColor, levelStr, colorReset,
		moduleStr, formattedMsg)
}

func (l *Logger) levelString(level Level) (string, string) {
	switch level {
	case LevelDebug:
		return "DBG", colorBlue
	case LevelInfo:
		return "INF", colorGreen
	case LevelWarn:
		return "WRN", colorYellow
	case LevelError:
		return "ERR", colorRed
	default:
		return "???", colorReset
	}
}

// Ensure Logger implements waLog.Logger.
var _ waLog.Logger = (*Logger)(nil)
