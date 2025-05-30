package logger

import (
	"time"
)

type logLevel uint8

// Logs with this level will appear only if app running in debug mode
const DebugLogLevel logLevel = 0
const InfoLogLevel logLevel = 1
const WarningLogLevel logLevel = 2
const ErrorLogLevel logLevel = 3
// Logs with this level will be handled immediately after calling Log().
// Also os.Exit(1) will be called after log creation.
const FatalLogLevel logLevel = 4
// Logs with this level will be handled immediately after calling Log().
// Also will cause panic after log creation.
const PanicLogLevel logLevel = 5

var logLevelToStrMap = map[logLevel]string{
    DebugLogLevel: "DEBUG",
    InfoLogLevel: "INFO",
    WarningLogLevel: "WARNING",
    ErrorLogLevel: "ERROR",
    FatalLogLevel: "FATAL",
    PanicLogLevel: "PANIC",
}

func (s logLevel) String() string{
    return logLevelToStrMap[s]
}

type LogEntry struct {
    Timestamp time.Time `json:"ts"`
    Service   string    `json:"service"`
    Instance  string    `json:"instance"`
    rawLevel  logLevel
    Level     string    `json:"level"`
    Source    string    `json:"source,omitempty"`
    Message   string    `json:"msg"`
    Error     string    `json:"error,omitempty"`
}

// Creates a new log entry. Timestamp is time.Now().
// If level is not error, fatal or panic, then Error will be empty, even if err specified.
func NewLogEntry(level logLevel, src string, msg string, err string) LogEntry {
    e := LogEntry{
        Timestamp: time.Now(),
        Service: "sentinel",
        Instance: "default", // TODO replace "default" with service id
        rawLevel: level,
        Level: level.String(),
        Source: src,
        Message: msg,
    }

    // error, fatal, panic
    if level >= ErrorLogLevel {
        e.Error = err
    }

    return e
}

