package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Filesystem"
	"github.com/RogulinSV/streamdeck-statistico/v2/Http"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
	"github.com/RogulinSV/streamdeck-statistico/v2/Websocket"
)

var pwd string

func init() {
	var err error

	pwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

type Result struct {
	mutex *sync.Mutex
	title string
	value string
	units string
}

func NewResult(title string) *Result {
	return &Result{
		mutex: &sync.Mutex{},
		title: title,
		value: "",
		units: "",
	}
}

func (r *Result) SetValue(value string, units string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.value = value
	r.units = units
}

var results = make(map[string]*Result)

func SetResult(channel string, result *Result) {
	results[channel] = result
}

func GetResult(channel string) *Result {
	var result, ok = results[channel]

	if ok {
		return result
	}

	return nil
}

type Task struct {
	title       string
	payload     func(r *Result)
	interval    uint
	description string
}

func NewTask(title string, description string, payload func(r *Result), interval uint) Task {
	return Task{
		title:       title,
		payload:     payload,
		interval:    interval,
		description: description,
	}
}

func (t Task) Schedule(result *Result, scheduler Scheduler.Scheduler) *Scheduler.Scheduled {
	return scheduler.Schedule(
		Scheduler.NewTask(
			func() {
				t.payload(result)
			},
			time.Second*time.Duration(t.interval),
			t.description,
		),
	)
}

func (t Task) Execute(result *Result) {
	t.payload(result)
}

var tasks = make(map[string]Task)

func NewTaskEachSecond(description string, title string, payload func(r *Result)) Task {
	return NewTask(title, description, payload, 1)
}

func Register(tasks map[string]Task, logger Logger.Logger) {
	tasks["utorrent/clients"] = NewTaskEachSecond("Загрузка количества клиентов uTorrent", "uClients", func(r *Result) {
		r.SetValue("1", "ppl")
	})
}

func main() {
	var port = flag.Int("port", 8080, "Порт сервера")
	var v = flag.Bool("v", false, "Флаг уровня отладки: общий")
	var vv = flag.Bool("vv", false, "Флаг уровня отладки: усиленный")
	var vvv = flag.Bool("vvv", false, "Флаг уровня отладки: максимальный")
	var silent = flag.Bool("silent", false, "Режим работы без вывода отладочной информации")
	var output = flag.String("output", "", "Путь к файлу с журналом")
	flag.Parse()

	var level = Logger.ERROR
	if *vvv {
		level = Logger.DEBUG
	} else if *vv {
		level = Logger.INFO
	} else if *v {
		level = Logger.WARN
	}

	var logger, cleanup = NewLogger(level, *silent, *output)
	defer cleanup()
	logger.Info("Программа запущена", Logger.Context{})

	Register(tasks, logger.WithPrefix("[task] "))
	var scheduler = Scheduler.NewScheduler(logger.WithPrefix("[scheduler] "))
	defer scheduler.Close()

	var handle = func(stack Http.Stack, channel string, logger Logger.Logger) {
		var connection *Websocket.Connection
		var ctx = Logger.Context{
			"channel": channel,
		}

		logger.Debug("Обновление HTTP-сервера: {channel}", ctx)
		connection = Websocket.NewConnection(stack, logger.WithPrefix("[ws] "))
		if connection == nil {
			logger.Error("Не удалось обновить HTTP-сервер: {channel}", ctx)
			return
		}
		defer connection.Close()

		logger.Debug("Переход в режим чтения канала {channel}", ctx)
		for {
			var message *Websocket.Message
			message = connection.Read()
			if message != nil {
				logger.Debug("Обработка полученных WS-данных из канала {channel}: {message}", Logger.Context{
					"message": message.Describe(),
				}.With(ctx))
				message = HandleMessage(channel, message, scheduler)
				if message != nil {
					logger.Debug("Отправка сформированных WS-данных в канал {channel}: {message}", Logger.Context{
						"message": message.Describe(),
					}.With(ctx))
					if !connection.Write(message) {
						logger.Error("Не удалось отправить сформированные WS-данные в канал {channel}", ctx)
					}
				} else {
					logger.Error("Не удалось обработать WS-данные из канала {channel}", ctx)
				}
			} else {
				logger.Error("Не удалось получить WS-данные из канала {channel}", ctx)
			}
		}
	}

	var route string
	var handler = http.NewServeMux()

	route = "/subscribe/{channel}"
	handler.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		var stack = Http.NewStack(r, w)
		var channel = r.PathValue("channel")
		handle(stack, channel, logger.WithPrefix("[http] "))
	})
	logger.Info("Добавлен обработчик запросов: {route}", Logger.Context{
		"route": route,
	})

	var server = &http.Server{
		Addr:    ":" + strconv.Itoa(*port),
		Handler: handler,
	}
	go func(logger Logger.Logger) {
		logger.Debug("Запуск HTTP-сервера {port}", Logger.Context{
			"port": server.Addr,
		})
		if err := server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("Не удалось запустить HTTP-сервер: {error}", Logger.Context{
					"error": err,
				})
			}
		}
	}(logger)

	var stop = make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Debug("Остановка HTTP-сервера {port}", Logger.Context{
		"port": server.Addr,
	})
	var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Warn("Принудительное завершение работы: {error}", Logger.Context{
			"error": err,
		})
	}

	logger.Info("Программа завершена", Logger.Context{})
}

func NewLogger(level Logger.Level, silent bool, output string) (Logger.Logger, func()) {
	var logger Logger.Logger
	var cleanup = func() {
		// pass...
	}
	var files []io.Writer
	var writers []io.Writer

	if !silent {
		files = append(files, os.Stdout)
		logger = Logger.NewLogger(level, files)
	} else {
		logger = Logger.NewNullLogger()
	}

	if output != "" {
		var file, err = Filesystem.ResolveFilePath(output)
		if err != nil {
			logger.Error("Не удалось проверить файл журнала {output}: {error}", Logger.Context{
				"output": output,
				"error":  err,
			})
			file = Filesystem.SplitFilePath(pwd, "debug.log")
		}
		writers, err = Filesystem.OpenWriters(file)
		cleanup = func(logger Logger.Logger, output string) func() {
			return func() {
				var err = Filesystem.CloseWriters(files)
				if err != nil {
					logger.Error("Не удалось закрыть файл журнала {output}: {error}", Logger.Context{
						"output": output,
						"error":  err,
					})
				}
			}
		}(logger, output)
		if err != nil {
			logger.Error("Не удалось открыть файл журнала на запись {output}: {error}", Logger.Context{
				"output": output,
				"error":  err,
			})
		} else {
			files = append(files, writers...)
			logger = Logger.NewLogger(level, files)
		}
	}

	return logger, cleanup
}

var scheduled map[string]*Scheduler.Scheduled

func SetScheduled(channel string, s *Scheduler.Scheduled) {
	scheduled[channel] = s
}

func IsScheduled(channel string) bool {
	if _, ok := scheduled[channel]; !ok {
		return false
	}
	return true
}

type Output struct {
	title string
	value string
	units string
}

func HandleMessage(channel string, message *Websocket.Message, scheduler Scheduler.Scheduler) *Websocket.Message {
	var result *Result
	var data []byte
	var err error

	if message.IsText() {
		result = GetResult(channel)
		if result == nil && !IsScheduled(channel) {
			if task, ok := tasks[channel]; ok {
				result = NewResult(task.title)
				SetResult(channel, result)
				SetScheduled(channel, task.Schedule(result, scheduler))
			}
		}
	}

	if result != nil {
		data, err = json.Marshal(Output{
			title: result.title,
			value: result.value,
			units: result.units,
		})
		if err != nil {

		}
	}

	return Websocket.NewMessage(1, data)
}
