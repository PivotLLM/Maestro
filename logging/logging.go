/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/PivotLLM/Maestro/global"
)

// Logger provides structured logging with the required format
type Logger struct {
	logger  *log.Logger
	level   string
	logFile *os.File
}

// New creates a new logger instance that writes to the specified file
func New(logPath string) (*Logger, error) {
	// Expand tilde in path
	if len(logPath) >= 2 && logPath[:2] == "~/" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			logPath = filepath.Join(homeDir, logPath[2:])
		}
	}

	// Ensure log directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file (append mode)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	logger := log.New(logFile, "", 0) // No default prefix/flags since we format ourselves
	return &Logger{
		logger:  logger,
		level:   global.LogLevelInfo,
		logFile: logFile,
	}, nil
}

// Sync flushes any buffered log data to disk
func (l *Logger) Sync() error {
	if l.logFile != nil {
		return l.logFile.Sync()
	}
	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.logFile != nil {
		// Flush before closing
		_ = l.logFile.Sync()
		return l.logFile.Close()
	}
	return nil
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level string) {
	l.level = level
}

// shouldLog determines if a message should be logged based on the current level
func (l *Logger) shouldLog(level string) bool {
	levels := map[string]int{
		global.LogLevelDebug: 0,
		global.LogLevelInfo:  1,
		global.LogLevelWarn:  2,
		global.LogLevelError: 3,
		global.LogLevelFatal: 4,
	}

	currentLevel, exists := levels[l.level]
	if !exists {
		currentLevel = levels[global.LogLevelInfo]
	}

	messageLevel, exists := levels[level]
	if !exists {
		messageLevel = levels[global.LogLevelInfo]
	}

	return messageLevel >= currentLevel
}

// formatMessage formats a log message with the required format
func (l *Logger) formatMessage(level, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	pid := os.Getpid()
	return fmt.Sprintf("%s [%s] [%d] %s", timestamp, level, pid, message)
}

// log performs the actual logging
func (l *Logger) log(level, message string) {
	if l.shouldLog(level) {
		formatted := l.formatMessage(level, message)
		l.logger.Println(formatted)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	l.log(global.LogLevelDebug, message)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.log(global.LogLevelInfo, message)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.log(global.LogLevelWarn, message)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.log(global.LogLevelError, message)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string) {
	l.log(global.LogLevelFatal, message)
	_ = l.Close() // Ensure log is flushed before exit
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}
