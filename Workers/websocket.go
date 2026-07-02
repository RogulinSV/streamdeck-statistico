package Workers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
	"github.com/RogulinSV/streamdeck-statistico/v2/WebSocket"
)

type workerRegistryEntry struct {
	worker *Scheduler.Worker
	result Scheduler.Result
}

type WorkerRegistry struct {
	entries map[string]workerRegistryEntry
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		entries: make(map[string]workerRegistryEntry),
	}
}

func (r *WorkerRegistry) Add(channel string, runner Scheduler.Runner, timeout uint, delay uint) {
	var worker = Scheduler.NewWorker(channel, time.Duration(timeout), time.Duration(delay), runner)
	r.entries[channel] = workerRegistryEntry{
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
	if entry, ok := r.entries[channel]; ok {
		return entry.worker
	}

	return nil
}

func (r *WorkerRegistry) GetResult(channel string) Scheduler.Result {
	if entry, ok := r.entries[channel]; ok {
		return entry.result
	}

	return nil
}

func (r *WorkerRegistry) setResult(channel string, result Scheduler.Result) bool {
	var entry, ok = r.entries[channel]
	if ok {
		entry.result = result
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

type Counter struct {
	value    int
	unsigned bool
}

func NewCounter(unsigned bool) *Counter {
	return &Counter{
		unsigned: unsigned,
	}
}

func (c *Counter) Value() int {
	return c.value
}

func (c *Counter) Increment(value int) int {
	c.value += value

	return c.Value()
}

func (c *Counter) Decrement(value int) int {
	if !c.unsigned || c.value > value {
		c.value -= value
	} else {
		c.value = 0
	}

	return c.Value()
}

func (r *ServerRunner) Run(context context.Context, result chan<- Scheduler.Result) {
	var handler = http.NewServeMux()
	var route string
	var clients = NewCounter(true)
	var send = func(value int) {
		result <- Scheduler.NewResult().AddMetric("clients", Scheduler.NewMetric("Клиенты", strconv.Itoa(value), ""))
	}

	route = "/subscribe/{channel}"
	handler.HandleFunc(route, func(response http.ResponseWriter, request *http.Request) {
		var channel = request.PathValue("channel")
		var connection = WebSocket.NewConnection(response, request, r.logger)
		if connection != nil {
			send(clients.Increment(1))
			r.handle(channel, connection, context)
			send(clients.Decrement(1))
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
		r.shutdown(server)
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

func (r *ServerRunner) shutdown(server *http.Server) {
	var context, cancel = Scheduler.NewTimeoutContext(5)
	defer cancel()

	r.logger.Debug("Остановка HTTP-сервера", Logger.Context{})
	var err = server.Shutdown(context)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.logger.Error("Не удалось остановить HTTP-сервер: {error}", Logger.Context{
			"error": err,
		})
	} else {
		r.logger.Info("Остановлен HTTP-сервер", Logger.Context{})
	}
}

func (r *ServerRunner) handle(channel string, connection *WebSocket.Connection, context context.Context) {
	defer connection.Close()
	go func() {
		<-context.Done()
		connection.Close()
	}()

	var message *WebSocket.Message
	for {
		message = connection.Read()
		if message != nil && message.IsText() {
			var key = message.GetText()
			if r.registry.Has(channel) {
				if !r.scheduler.IsRunning(channel) {
					var worker = r.registry.GetWorker(channel)
					var output = r.scheduler.Start(worker)
					go func() {
						select {
						case result, ok := <-output:
							if !ok {
								r.logger.Error("Не удалось прочитать ответ воркера {worker}", Logger.Context{
									"worker": worker.Describe(),
								})
								return
							}
							if !r.registry.setResult(channel, result) {
								r.logger.Error("Не удалось обновить ответ воркера {worker}", Logger.Context{
									"worker": worker.Describe(),
								})
							}
						case <-context.Done():
							return
						}
					}()
				}

				var result = r.registry.GetResult(channel)
				var metric Scheduler.Metric
				if result.HasMetric(key) {
					metric = result.GetMetric(key)
				}

				var data, err = json.Marshal(metric)
				if err != nil {
					message = WebSocket.NewTextMessage(data)
					connection.Write(message)
				} else {
					r.logger.Error("Не удалось преобразовать ответ в JSON-формат: {error}", Logger.Context{
						"error": err,
					})
				}
			}
		} else if context.Err() != nil {
			return
		}
	}
}
