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

// Context структура контекста отладочного журнала
type Context map[string]any

// With метод реализует расширение данных контекста отладочного журнала
func (c Context) With(context Context) Context {
	for name, value := range context {
		c[name] = value
	}

	return c
}

// Logger интерфейс отладочного журнала
type Logger interface {
	Debug(message string, context Context)
	Info(message string, context Context)
	Warn(message string, context Context)
	Error(message string, context Context)
	Fatal(message string, context Context)
	WithPrefix(prefix string) Logger
}

// Level тип данных уровня важности сообщений отладочного журнала
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

// Handler интерфейс обработчика сообщений отладочного журнала
type Handler interface {
	Write(level Level, message string, context Context)
	WithPrefix(prefix string) Handler
}

// NullHandler структура обработчика сообщений без полезной нагрузки
type NullHandler struct{}

// Write метод реализует интерфейс обработчика сообщений
func (h *NullHandler) Write(level Level, message string, context Context) {
	// pass...
}

// WithPrefix метод реализует интерфейс обработчика сообщений
func (h *NullHandler) WithPrefix(prefix string) Handler {
	return h
}

// format метод реализует форматирование сообщения отладочного журнала
func (h *NullHandler) format(message string, level Level) string {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	var timestamp = time.Now().Format("2006-01-02 15:04:05")

	return fmt.Sprintf("[%s %dMiB] %s %s", timestamp, memory.Sys/1024/1024, prefixes[level], message)
}

// replace метод реализует замену плейсхолдеров сообщения отладочного журнала
func (h *NullHandler) replace(message string, context Context) string {
	for key, value := range context {
		message = strings.ReplaceAll(message, "{"+key+"}", fmt.Sprintf("%v", value))
	}

	return message
}

// FilterHandler структура обработчика сообщений с функцией фильтрации
type FilterHandler struct {
	handler Handler
	filter  func(Level) bool
}

// NewFilterHandler конструктор обработчика сообщений с функцией фильтрации
func NewFilterHandler(handler Handler, filter func(Level) bool) *FilterHandler {
	return &FilterHandler{
		handler: handler,
		filter:  filter,
	}
}

// Write метод реализует интерфейс обработчика сообщений
func (h *FilterHandler) Write(level Level, message string, context Context) {
	if h.filter(level) {
		h.handler.Write(level, message, context)
	}
}

// WithPrefix метод реализует интерфейс обработчика сообщений
func (h *FilterHandler) WithPrefix(prefix string) Handler {
	return NewFilterHandler(
		h.handler.WithPrefix(prefix),
		h.filter,
	)
}

// SyslogHandler структура обработчика сообщений отладочного журнала на базе системного журнала log.Logger
type SyslogHandler struct {
	NullHandler
	prefix string
	log    *log.Logger
}

// NewSyslogHandler конструктор обработчика сообщений отладочного журнала на базе системного журнала log.Logger
func NewSyslogHandler(prefix string, log *log.Logger) *SyslogHandler {
	return &SyslogHandler{
		prefix: prefix,
		log:    log,
	}
}

// Write метод реализует отправку сообщений в системный журнал log.Logger
func (h *SyslogHandler) Write(level Level, message string, context Context) {
	h.log.SetPrefix(h.format(h.prefix, level))
	if level < FATAL {
		h.log.Println(h.replace(message, context))
	} else {
		h.log.Fatalln(h.replace(message, context))
	}
}

// WithPrefix метод реализует интерфейс обработчика сообщений
func (h *SyslogHandler) WithPrefix(prefix string) Handler {
	return NewSyslogHandler(prefix, h.log)
}

// NullLogger структура отладочного журнала без полезной нагрузки
type NullLogger struct{}

// NewNullLogger конструктор отладочного журнала без полезной нагрузки
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

// MultiLogger структура отладочного журнала с коллекцией обработчиков сообщений
type MultiLogger struct {
	handlers []Handler
}

// NewMultiLogger конструктор отладочного журнала с коллекцией обработчиков сообщений
func NewMultiLogger(handlers ...Handler) *MultiLogger {
	return &MultiLogger{
		handlers: handlers,
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

// decorate метод реализует декоратор коллекции обработчиков сообщений
func (l MultiLogger) decorate(decorator func(handler Handler) Handler) {
	for i, handler := range l.handlers {
		l.handlers[i] = decorator(handler)
	}
}

// WithPrefix метод реализует интерфейс отладочного журнала
func (l MultiLogger) WithPrefix(prefix string) Logger {
	var handlers []Handler
	for _, handler := range l.handlers {
		handlers = append(handlers, handler.WithPrefix(prefix))
	}

	return NewMultiLogger(handlers...)
}

// NewSyslogLogger конструктор отладочного журнала с обработчиком сообщений через запись в файлы
func NewSyslogLogger(writers ...io.Writer) MultiLogger {
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

	handler = &SyslogHandler{
		prefix: prefix,
		log:    log.New(writer, prefix, log.Lmsgprefix),
	}

	return MultiLogger{
		handlers: []Handler{handler},
	}
}

// MutexLogger структура отладочного журнала с функцией атомарной записи сообщений
type MutexLogger struct {
	mutex  *sync.Mutex
	logger Logger
}

// NewMutexLogger конструктор отладочного журнала с функцией атомарной записи сообщений
func NewMutexLogger(logger Logger) MutexLogger {
	return MutexLogger{
		mutex:  &sync.Mutex{},
		logger: logger,
	}
}

func (l MutexLogger) Debug(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Debug(message, context)
}

func (l MutexLogger) Info(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Info(message, context)
}

func (l MutexLogger) Warn(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Warn(message, context)
}

func (l MutexLogger) Error(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Error(message, context)
}

func (l MutexLogger) Fatal(message string, context Context) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.logger.Fatal(message, context)
}

// WithPrefix метод реализует интерфейс отладочного журнала
func (l MutexLogger) WithPrefix(prefix string) Logger {
	return NewMutexLogger(l.logger.WithPrefix(prefix))
}

// ClosableLogger структура отладочного журнала с функцией финализации состояния
type ClosableLogger struct {
	logger Logger
	close  func()
}

// NewClosableLogger конструктор отладочного журнала с функцией финализации состояния
func NewClosableLogger(logger Logger, close func()) ClosableLogger {
	return ClosableLogger{
		logger: logger,
		close:  close,
	}
}

// Close метод реализует финализацию состояния
func (l ClosableLogger) Close() {
	l.close()
}

func (l ClosableLogger) Debug(message string, context Context) {
	l.logger.Debug(message, context)
}

func (l ClosableLogger) Info(message string, context Context) {
	l.logger.Info(message, context)
}

func (l ClosableLogger) Warn(message string, context Context) {
	l.logger.Warn(message, context)
}

func (l ClosableLogger) Error(message string, context Context) {
	l.logger.Error(message, context)
}

func (l ClosableLogger) Fatal(message string, context Context) {
	l.logger.Fatal(message, context)
}

// WithPrefix метод реализует интерфейс отладочного журнала
func (l ClosableLogger) WithPrefix(prefix string) Logger {
	return NewClosableLogger(l.logger.WithPrefix(prefix), l.close)
}

// NewLogger конструктор отладочного журнала по умолчанию
func NewLogger(level Level, files []io.Writer) Logger {
	if len(files) == 0 {
		return NewNullLogger()
	}

	var logger = NewSyslogLogger(files...)
	var filter = func(l Level) bool {
		return l >= level
	}
	logger.decorate(func(handler Handler) Handler {
		return NewFilterHandler(handler, filter)
	})

	return NewMutexLogger(logger)
}
