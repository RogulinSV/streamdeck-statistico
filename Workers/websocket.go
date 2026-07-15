package Workers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Context"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
	"github.com/RogulinSV/streamdeck-statistico/v2/WebSocket"
)

type entry struct {
	worker *Scheduler.Worker
	result Scheduler.Result
}

type WorkerRegistry struct {
	entries map[string]entry
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		entries: make(map[string]entry),
	}
}

func (r *WorkerRegistry) Add(channel string, runner Scheduler.Runner, timeout uint, delay uint) {
	var worker = Scheduler.NewWorker(channel, time.Duration(timeout)*time.Second, time.Duration(delay)*time.Second, runner)
	r.entries[channel] = entry{
		worker: worker,
		result: Scheduler.NewResult(),
	}
}

func (r *WorkerRegistry) Has(channel string) bool {
	var _, ok = r.entries[channel]

	return ok
}

func (r *WorkerRegistry) Count() uint {
	return uint(len(r.entries))
}

func (r *WorkerRegistry) GetWorker(channel string) *Scheduler.Worker {
	if e, ok := r.entries[channel]; ok {
		return e.worker
	}

	return nil
}

func (r *WorkerRegistry) GetResult(channel string) Scheduler.Result {
	if e, ok := r.entries[channel]; ok {
		return e.result
	}

	return nil
}

func (r *WorkerRegistry) SetResult(channel string, result Scheduler.Result) bool {
	var e entry
	var ok bool

	if e, ok = r.entries[channel]; ok {
		e.result.Merge(result)
		r.entries[channel] = e
	}

	return ok
}

type ServerRunner struct {
	port      uint16
	scheduler *Scheduler.Scheduler
	registry  *WorkerRegistry
	logger    Logger.Logger
}

func NewServerRunner(port uint16, scheduler *Scheduler.Scheduler, registry *WorkerRegistry, logger Logger.Logger) *ServerRunner {
	return &ServerRunner{
		port:      port,
		scheduler: scheduler,
		registry:  registry,
		logger:    logger,
	}
}

func (r *ServerRunner) Defaults() Scheduler.Result {
	return Scheduler.AddMetric(
		"clients",
		Scheduler.NewMetric("Клиенты", "0"),
	)
}

func (r *ServerRunner) Run(context context.Context, result chan<- Scheduler.Result) {
	var handler = http.NewServeMux()
	var route string
	var clients sync.Map
	var stat = func(clients *Counter) {
		if context.Err() == nil {
			result <- Scheduler.AddMetric("clients", Scheduler.NewMetric("Клиенты", clients.ToStr()))
		}
	}

	route = "/subscribe/{channel}"
	handler.HandleFunc(route, func(response http.ResponseWriter, request *http.Request) {
		var channel = request.PathValue("channel")
		var prefix = r.logger.GetPrefix() + " (url:" + strings.Replace(route, "{channel}", channel, 1) + ") "
		var logger = r.logger.WithPrefix(prefix)

		var counter *Counter
		if c, ok := clients.Load(channel); ok {
			counter = c.(*Counter)
			counter.Increment(1)
		} else {
			counter = NewCounter(1)
			clients.Store(channel, counter)
		}
		stat(counter)
		defer func() {
			counter.Decrement(1)
			stat(counter)
		}()

		var connection = WebSocket.NewConnection(response, request, logger)
		if connection != nil {
			r.handle(channel, connection, counter, context)
			connection.Close()
		}
	})
	r.logger.Info("Добавлен обработчик запросов: {route}", Logger.Context{
		"route": route,
	})

	var server = &http.Server{
		Addr:    ":" + strconv.Itoa(int(r.port)),
		Handler: handler,
	}
	go func() {
		<-context.Done()
		r.stop(server)
	}()
	r.logger.Debug("Запуск HTTP-сервера {port}", Logger.Context{
		"port": server.Addr,
	})
	var err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.logger.Error("Не удалось запустить HTTP-сервер: {error}", Logger.Context{
			"error": err,
		})
	}
}

func (r *ServerRunner) stop(server *http.Server) {
	var timeout, cancel = Context.WithTimeout(5, nil)
	defer cancel()

	r.logger.Debug("Остановка HTTP-сервера", Logger.Context{})
	var err = server.Shutdown(timeout)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.logger.Error("Не удалось остановить HTTP-сервер: {error}", Logger.Context{
			"error": err,
		})
	} else {
		r.logger.Info("Остановлен HTTP-сервер", Logger.Context{})
	}
}

func (r *ServerRunner) handle(channel string, connection *WebSocket.Connection, counter *Counter, context context.Context) {
	var message *WebSocket.Message

	for {
		if context.Err() != nil {
			break
		}

		message = connection.Read()
		if message == nil {
			break
		}
		if !message.IsText() {
			continue
		}

		var metric = Scheduler.NewMetric("", "")

		if r.registry.Has(channel) {
			if !r.scheduler.HasWorker(channel) {
				var worker = r.registry.GetWorker(channel)
				if !r.registry.SetResult(channel, worker.Defaults()) {
					r.logger.Error("Не удалось установить ответ по умолчанию для воркера {worker}", Logger.Context{
						"worker": worker.Describe(),
					})
				}
				r.scheduler.Start(worker)

				go func(worker *Scheduler.Worker) {
					for {
						select {
						case result, ok := <-worker.Output():
							if !ok {
								r.logger.Error("Не удалось прочитать ответ воркера {worker}", Logger.Context{
									"worker": worker.Describe(),
								})
								return
							}
							if !r.registry.SetResult(channel, result) {
								r.logger.Error("Не удалось обновить ответ воркера {worker}", Logger.Context{
									"worker": worker.Describe(),
								})
							}
						case <-worker.Done():
							return
						case <-context.Done():
							return
						}
					}
				}(worker)
			}

			var key = message.GetText()
			var result = r.registry.GetResult(channel)
			if result.HasMetric(key) {
				metric = result.GetMetric(key)
			}
		}

		if context.Err() != nil {
			break
		}

		var data, err = json.Marshal(metric)
		if err == nil {
			message = WebSocket.NewTextMessage(data)
			if !connection.Write(message) {
				break
			}
		} else {
			r.logger.Error("Не удалось преобразовать ответ в JSON-формат: {error}", Logger.Context{
				"error": err,
			})
		}
	}

	if counter.Value() <= 1 {
		var worker = r.scheduler.GetWorker(channel)
		if worker != nil {
			r.logger.Debug("Отмена воркера {worker}", Logger.Context{
				"worker": worker.Describe(),
			})
			r.scheduler.Stop(worker)
		}
	}
}
