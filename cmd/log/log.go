package log

import (
	"fmt"
	"os"

	"github.com/fatih/color"
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
	msg := fmt.Sprintf(format, args...)

	switch level {
	case INFO:
		fmt.Printf("%s\n", msg)
	case WARN:
		fmt.Printf("%s\n", color.YellowString(msg))
	case ERROR:
		fmt.Printf("%s\n", color.RedString(msg))
	case DEBUG:
		fmt.Printf("[debug] %s\n", color.CyanString(msg))
	case FATAL:
		fmt.Printf("%s\n", color.HiRedString(msg))
	}
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
