package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

type LogLevel int

const (
	INFO LogLevel = iota
	WARN
	ERROR
	DEBUG
	FATAL
)

func logMessage(level LogLevel, format string, args ...any) {
	var logLevel string
	switch level {
	case INFO:
		logLevel = "info"
	case WARN:
		logLevel = "warn"
	case ERROR:
		logLevel = "error"
	case DEBUG:
		logLevel = "debug"
	case FATAL:
		logLevel = "fatal"
	}
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s: %s\n", logLevel, time.Now().Format(time.RFC3339), msg)
}

func Info(format string, args ...any) {
	logMessage(INFO, format, args...)
}

// Warn logs warning messages
func Warn(format string, args ...any) {
	logMessage(WARN, format, args...)
}

// Error logs error messages
func Error(format string, args ...any) {
	logMessage(ERROR, format, args...)
}

// Debug logs debug messages
func Debug(format string, args ...any) {
	logMessage(DEBUG, format, args...)
}

func Fatal(format string, args ...any) {
	logMessage(FATAL, format, args...)
	os.Exit(1)
}
