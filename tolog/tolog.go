package tolog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Constants representing different log levels.
const (
	StatusInfo    = "info"
	StatusWarning = "warning"
	StatusError   = "error"
	StatusDebug   = "debug"
	StatusNotice  = "notice"
	StatusUnknown = "unknown"
)

// Variables for managing log file and writing to file concurrently.
var logFile *os.File
var writeChannel chan string

// ToLog represents a log entry with various attributes.
type ToLog struct {
	logType    string
	logContext string
	logTime    string
	FullLog    string
}

// Options is a function type for specifying log options using functional options pattern.
type Options func(l *ToLog)

// WithType sets the log type using functional options.
func WithType(level string) Options {
	return func(l *ToLog) {
		if level != StatusInfo && level != StatusWarning && level != StatusError && level != StatusNotice && level != StatusDebug {
			level = StatusUnknown
		}
		l.logType = level
		CreateFullLog(l)
	}
}

// WithContext sets the log context using functional options.
func WithContext(ctx string) Options {
	return func(l *ToLog) {
		l.logContext = ctx
	}
}

// Log creates a new ToLog instance with default values and applies any specified options.
func Log(options ...Options) *ToLog {
	tolog := &ToLog{
		logType:    StatusInfo,
		logContext: "",
		logTime:    time.Now().Format("2006-01-02 15:04:05"),
	}

	for _, option := range options {
		option(tolog)
	}

	return tolog
}

// Context sets the log context for an existing ToLog instance.
func (l *ToLog) Context(ctx string) *ToLog {
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Type sets the log type for an existing ToLog instance.
func (l *ToLog) Type(level string) *ToLog {
	level = strings.ToLower(level)
	if level != StatusInfo && level != StatusWarning && level != StatusError && level != StatusNotice && level != StatusDebug {
		level = StatusUnknown
	}
	l.logType = level
	CreateFullLog(l)
	return l
}

// Info sets the log type to "info" and sets the log context for an existing ToLog instance.
func (l *ToLog) Info(ctx string) *ToLog {
	l.logType = StatusInfo
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Infof sets the log type to "info" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Infof(format string, a ...any) *ToLog {
	l.logType = StatusInfo
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Infoln sets the log type to "info" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Infoln(a ...any) *ToLog {
	l.logType = StatusInfo
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Warning sets the log type to "warning" and sets the log context for an existing ToLog instance.
func (l *ToLog) Warning(ctx string) *ToLog {
	l.logType = StatusWarning
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Warningf sets the log type to "warning" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Warningf(format string, a ...any) *ToLog {
	l.logType = StatusWarning
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Warningln sets the log type to "warning" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Warningln(a ...any) *ToLog {
	l.logType = StatusWarning
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Error sets the log type to "error" and sets the log context for an existing ToLog instance.
func (l *ToLog) Error(ctx string) *ToLog {
	l.logType = StatusError
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Errorf sets the log type to "error" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Errorf(format string, a ...any) *ToLog {
	l.logType = StatusError
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Errorln sets the log type to "error" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Errorln(a ...any) *ToLog {
	l.logType = StatusError
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Notice sets the log type to "notice" and sets the log context for an existing ToLog instance.
func (l *ToLog) Notice(ctx string) *ToLog {
	l.logType = StatusNotice
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Noticef sets the log type to "notice" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Noticef(format string, a ...any) *ToLog {
	l.logType = StatusNotice
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Noticeln sets the log type to "notice" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Noticeln(a ...any) *ToLog {
	l.logType = StatusNotice
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Debug sets the log type to "debug" and sets the log context for an existing ToLog instance.
func (l *ToLog) Debug(ctx string) *ToLog {
	l.logType = StatusDebug
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Debugf sets the log type to "debug" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Debugf(format string, a ...any) *ToLog {
	l.logType = StatusDebug
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Debugln sets the log type to "debug" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Debugln(a ...any) *ToLog {
	l.logType = StatusDebug
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// PrintLog prints the full log to the console for an existing ToLog instance.
func (l *ToLog) PrintLog() *ToLog {
	CreateFullLog(l)
	fmt.Println(l.FullLog)
	return l
}

// CreateFullLog creates the full log message by combining log time, type, and context.
func CreateFullLog(l *ToLog) {
	fullLog := "[" + l.logTime + "] " + "[" + l.logType + "] " + l.logContext
	l.FullLog = fullLog
}

// Write writes the full log to the log file.
func (l *ToLog) Write() {
	CreateFullLog(l)
	if logFile == nil {
		err := initLog()
		if err != nil {
			return
		}
	}
	_, err := logFile.WriteString(l.FullLog + "\n")
	if err != nil {
		return
	}
}

// WriteSafe writes the full log to the log file using a concurrent channel.
func (l *ToLog) WriteSafe() {
	CreateFullLog(l)
	if logFile == nil {
		err := initLog()
		if err != nil {
			return
		}
	}
	writeChannel <- l.FullLog + "\n"
}

// PrintAndWrite prints the full log to the console and writes it to the log file.
func (l *ToLog) PrintAndWrite() {
	CreateFullLog(l)
	fmt.Println(l.FullLog)
	if logFile == nil {
		err := initLog()
		if err != nil {
			return
		}
	}
	_, err := logFile.WriteString(l.FullLog + "\n")
	if err != nil {
		return
	}
}

func (l *ToLog) PrintAndWriteSafe() {
	CreateFullLog(l)
	fmt.Println(l.FullLog)
	if logFile == nil {
		err := initLog()
		if err != nil {
			return
		}
	}
	writeChannel <- l.FullLog + "\n"
}

// writeToFile is a goroutine that continuously writes log entries to the log file using the channel.
func writeToFile() {
	for {
		select {
		case logEntry := <-writeChannel:
			_, err := logFile.WriteString(logEntry)
			if err != nil {
				return
			}
		}
	}
}

// initLog initializes the log file and sets up the writeToFile goroutine.
func initLog() error {
	currentDay := time.Now().Format("2006-01-02")
	logFilePath := "./tolog/logs/painter-blog-log-" + currentDay + ".log"

	file, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("[error]", err)
		return err
	}
	logFile = file

	writeChannel = make(chan string)
	go writeToFile()

	return nil
}

// CloseLogFile closes the log file.
func CloseLogFile() {
	if logFile != nil {
		err := logFile.Close()
		if err != nil {
			return
		}
	}
}
