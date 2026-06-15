package Logger

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Context map[string]any

type Logger interface {
	Debug(message string, context Context)
	Info(message string, context Context)
	Warn(message string, context Context)
	Error(message string, context Context)
	Fatal(message string, context Context)
	WithPrefix(prefix string) Logger
}

type Level uint8

const (
	DEBUG Level = iota + 1
	INFO
	WARN
	ERROR
	FATAL
)

var prefixes = map[Level]string{
	DEBUG: " ...",
	INFO:  "[OK]",
	WARN:  "[WR]",
	ERROR: "[ER]",
	FATAL: "[FATAL]",
}

type Handler struct {
	filter func(Level) bool
	logger *log.Logger
	prefix string
}

func (h Handler) Write(level Level, message string, context Context) {
	if h.filter(level) {
		h.logger.SetPrefix(h.decorate(level, h.prefix))
		if level < FATAL {
			h.logger.Println(h.format(message, context))
		} else {
			h.logger.Fatalln(h.format(message, context))
		}
	}
}

func (h Handler) decorate(level Level, prefix string) string {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	var timestamp = time.Now().Format("2006-01-02 15:04:05")

	return fmt.Sprintf("[%s %dMib] %s %s", timestamp, memory.Sys/1024/1024, prefixes[level], prefix)
}

func (h Handler) format(message string, context Context) string {
	for key, value := range context {
		message = strings.ReplaceAll(message, "{"+key+"}", fmt.Sprintf("%v", value))
	}

	return message
}

type NullLogger struct{}

func NewNullLogger() NullLogger {
	return NullLogger{}
}

func (l NullLogger) Debug(message string, context Context) {}
func (l NullLogger) Info(message string, context Context)  {}
func (l NullLogger) Warn(message string, context Context)  {}
func (l NullLogger) Error(message string, context Context) {}
func (l NullLogger) Fatal(message string, context Context) {}
func (l NullLogger) WithPrefix(prefix string) Logger {
	return l
}

type MultiLogger struct {
	handlers []Handler
}

func NewMultiLogger(level Level, writers ...io.Writer) MultiLogger {
	var writer io.Writer
	var handler Handler
	var prefix = ""

	if len(writers) > 0 {
		if len(writers) > 1 {
			writer = io.MultiWriter(writers...)
		} else {
			writer = writers[0]
		}
	}

	handler = Handler{
		filter: func(l Level) bool {
			return l >= level
		},
		logger: log.New(writer, prefix, log.Lmsgprefix),
		prefix: prefix,
	}

	return MultiLogger{
		handlers: []Handler{handler},
	}
}

func (l MultiLogger) Debug(message string, context Context) {
	for _, handler := range l.handlers {
		handler.Write(DEBUG, message, context)
	}
}

func (l MultiLogger) Info(message string, context Context) {
	for _, handler := range l.handlers {
		handler.Write(INFO, message, context)
	}
}

func (l MultiLogger) Warn(message string, context Context) {
	for _, handler := range l.handlers {
		handler.Write(WARN, message, context)
	}
}

func (l MultiLogger) Error(message string, context Context) {
	for _, handler := range l.handlers {
		handler.Write(ERROR, message, context)
	}
}

func (l MultiLogger) Fatal(message string, context Context) {
	for _, handler := range l.handlers {
		handler.Write(FATAL, message, context)
	}
}

func (l MultiLogger) WithPrefix(prefix string) Logger {
	var logger = l
	for _, handler := range logger.handlers {
		handler.prefix = prefix
	}

	return logger
}

type SafeLogger struct {
	mutex  *sync.Mutex
	logger Logger
}

func NewSafeLogger(logger Logger) SafeLogger {
	return SafeLogger{
		mutex:  &sync.Mutex{},
		logger: logger,
	}
}

func (l SafeLogger) Debug(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Debug(message, context)
}

func (l SafeLogger) Info(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Info(message, context)
}

func (l SafeLogger) Warn(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Warn(message, context)
}

func (l SafeLogger) Error(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Error(message, context)
}

func (l SafeLogger) Fatal(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Fatal(message, context)
}

func (l SafeLogger) WithPrefix(prefix string) Logger {
	l.logger = l.logger.WithPrefix(prefix)

	return l
}

func NewLogger(level Level, files []io.Writer) Logger {
	var logger Logger
	var writers []io.Writer

	writers = append(writers, files...)
	if len(writers) == 0 {
		logger = NewNullLogger()
	} else {
		logger = NewMultiLogger(level, writers...)
		logger = NewSafeLogger(logger)
	}

	return logger
}
