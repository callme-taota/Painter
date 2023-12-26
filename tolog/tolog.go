package tolog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Constants representing different log levels.
const (
	ToLogStatusInfo    = "info"
	ToLogStatusWarning = "warning"
	ToLogStatusError   = "error"
	ToLogStatusDebug   = "debug"
	ToLogStatusNotice  = "notice"
	ToLogStatusUnknown = "unknown"
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

// ToLogOptions is a function type for specifying log options using functional options pattern.
type ToLogOptions func(l *ToLog)

// WithType sets the log type using functional options.
func WithType(level string) ToLogOptions {
	return func(l *ToLog) {
		if level != ToLogStatusInfo && level != ToLogStatusWarning && level != ToLogStatusError && level != ToLogStatusNotice && level != ToLogStatusDebug {
			level = ToLogStatusUnknown
		}
		l.logType = level
		CreateFullLog(l)
	}
}

// WithContext sets the log context using functional options.
func WithContext(ctx string) ToLogOptions {
	return func(l *ToLog) {
		l.logContext = ctx
	}
}

// Log creates a new ToLog instance with default values and applies any specified options.
func Log(options ...ToLogOptions) *ToLog {
	tolog := &ToLog{
		logType:    ToLogStatusInfo,
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
	if level != ToLogStatusInfo && level != ToLogStatusWarning && level != ToLogStatusError && level != ToLogStatusNotice && level != ToLogStatusDebug {
		level = ToLogStatusUnknown
	}
	l.logType = level
	CreateFullLog(l)
	return l
}

// Info sets the log type to "info" and sets the log context for an existing ToLog instance.
func (l *ToLog) Info(ctx string) *ToLog {
	l.logType = ToLogStatusInfo
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Infof sets the log type to "info" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Infof(format string, a ...any) *ToLog {
	l.logType = ToLogStatusInfo
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Infoln sets the log type to "info" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Infoln(a ...any) *ToLog {
	l.logType = ToLogStatusInfo
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Warning sets the log type to "warning" and sets the log context for an existing ToLog instance.
func (l *ToLog) Warning(ctx string) *ToLog {
	l.logType = ToLogStatusWarning
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Warningf sets the log type to "warning" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Warningf(format string, a ...any) *ToLog {
	l.logType = ToLogStatusWarning
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Warningln sets the log type to "warning" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Warningln(a ...any) *ToLog {
	l.logType = ToLogStatusWarning
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Error sets the log type to "error" and sets the log context for an existing ToLog instance.
func (l *ToLog) Error(ctx string) *ToLog {
	l.logType = ToLogStatusError
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Errorf sets the log type to "error" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Errorf(format string, a ...any) *ToLog {
	l.logType = ToLogStatusError
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Errorln sets the log type to "error" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Errorln(a ...any) *ToLog {
	l.logType = ToLogStatusError
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Notice sets the log type to "notice" and sets the log context for an existing ToLog instance.
func (l *ToLog) Notice(ctx string) *ToLog {
	l.logType = ToLogStatusNotice
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Noticef sets the log type to "notice" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Noticef(format string, a ...any) *ToLog {
	l.logType = ToLogStatusNotice
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Noticeln sets the log type to "notice" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Noticeln(a ...any) *ToLog {
	l.logType = ToLogStatusNotice
	l.logContext = fmt.Sprintln(a...)
	CreateFullLog(l)
	return l
}

// Debug sets the log type to "debug" and sets the log context for an existing ToLog instance.
func (l *ToLog) Debug(ctx string) *ToLog {
	l.logType = ToLogStatusDebug
	l.logContext = ctx
	CreateFullLog(l)
	return l
}

// Debugf sets the log type to "debug" and sets the formatted log context for an existing ToLog instance.
func (l *ToLog) Debugf(format string, a ...any) *ToLog {
	l.logType = ToLogStatusDebug
	l.logContext = fmt.Sprintf(format, a...)
	CreateFullLog(l)
	return l
}

// Debugln sets the log type to "debug" and sets the log context with a newline for an existing ToLog instance.
func (l *ToLog) Debugln(a ...any) *ToLog {
	l.logType = ToLogStatusDebug
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
		initLog()
	}
	logFile.WriteString(l.FullLog + "\n")
}

// WriteSafe writes the full log to the log file using a concurrent channel.
func (l *ToLog) WriteSafe() {
	CreateFullLog(l)
	if logFile == nil {
		initLog()
	}
	writeChannel <- l.FullLog + "\n"
}

// PrintAndWrite prints the full log to the console and writes it to the log file.
func (l *ToLog) PrintAndWrite() {
	CreateFullLog(l)
	fmt.Println(l.FullLog)
	if logFile == nil {
		initLog()
	}
	logFile.WriteString(l.FullLog + "\n")
}

func (l *ToLog) PrintAndWriteSafe() {
	CreateFullLog(l)
	fmt.Println(l.FullLog)
	if logFile == nil {
		initLog()
	}
	writeChannel <- l.FullLog + "\n"
}

// writeToFile is a goroutine that continuously writes log entries to the log file using the channel.
func writeToFile() {
	for {
		select {
		case logEntry := <-writeChannel:
			logFile.WriteString(logEntry)
		}
	}
}

// initLog initializes the log file and sets up the writeToFile goroutine.
func initLog() error {
	currentDay := time.Now().Format("2006-01-02")
	logFilePath := "./tolog/logs/painter-blog-tolog-" + currentDay + ".log"

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
		logFile.Close()
	}
}
