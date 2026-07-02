package Scheduler

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
)

func NewTimeoutContext(timeout uint) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
}

func NewSignalContext(signals ...os.Signal) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), signals...)
}

type Metric struct {
	title string
	value string
	units string
}

func NewMetric(title string, value string, units string) Metric {
	return Metric{
		title: title,
		value: value,
		units: units,
	}
}

func (m Metric) Title() string {
	return m.title
}

func (m Metric) Value() string {
	return m.value
}

func (m Metric) Units() string {
	return m.units
}

type Result map[string]Metric

func NewResult() Result {
	return make(Result)
}

func (r Result) AddMetric(name string, metric Metric) Result {
	r[name] = metric

	return r
}

func (r Result) HasMetric(name string) bool {
	var _, ok = r[name]

	return ok
}

func (r Result) GetMetric(name string) Metric {
	return r[name]
}

type Runner interface {
	Run(context context.Context, result chan<- Result)
}

type Worker struct {
	id      string
	runner  Runner
	timeout time.Duration
	delay   time.Duration
	context context.Context
	cancel  context.CancelFunc
}

func NewWorker(id string, timeout time.Duration, delay time.Duration, runner Runner) *Worker {
	return &Worker{
		id:      id,
		timeout: timeout,
		delay:   delay,
		runner:  runner,
	}
}

func (w *Worker) Run(context context.Context, result chan<- Result) {
	w.runner.Run(context, result)
}

func (w *Worker) Describe() string {
	return fmt.Sprintf("(%s)", w.id)
}

// Scheduler структура планировщика задач
type Scheduler struct {
	context context.Context
	logger  Logger.Logger
	workers sync.Map
	wg      sync.WaitGroup
}

// NewScheduler конструктор планировщика задач
func NewScheduler(context context.Context, logger Logger.Logger) *Scheduler {
	return &Scheduler{
		context: context,
		logger:  logger,
	}
}

func (s *Scheduler) Start(worker *Worker) <-chan Result {
	s.Stop(worker.id)

	var result = make(chan Result)
	worker.context, worker.cancel = context.WithCancel(s.context)
	s.workers.Store(worker.id, worker)
	s.wg.Add(1)
	var done = func() {
		s.wg.Done()
		s.workers.Delete(worker.id)
		close(result)
	}

	go func() {
		defer done()

		s.logger.Debug("Воркер {worker} запущен", Logger.Context{
			"worker": worker.Describe(),
		})

		var ctx context.Context
		var cancel context.CancelFunc
		var duration time.Duration
		var start time.Time

		for {
			if worker.context.Err() != nil {
				s.logger.Debug("Воркер {worker} остановлен", Logger.Context{
					"worker": worker.Describe(),
				})
				return
			}

			if worker.timeout > 0 {
				ctx, cancel = context.WithTimeout(worker.context, worker.timeout)
			} else {
				ctx, cancel = context.WithCancel(worker.context)
			}
			s.logger.Debug("Запуск задачи воркера {worker}", Logger.Context{
				"worker": worker.Describe(),
			})
			start = time.Now()
			worker.Run(ctx, result)
			duration = time.Now().Sub(start)
			cancel()
			s.logger.Debug("Завершение задачи воркера {worker}: {duration}", Logger.Context{
				"worker":   worker.Describe(),
				"duration": duration.String(),
			})

			if worker.context.Err() != nil || worker.delay == 0 {
				s.logger.Debug("Воркер {worker} остановлен", Logger.Context{
					"worker": worker.Describe(),
				})
				return
			}

			select {
			case <-time.After(worker.delay):
				// await
			case <-worker.context.Done():
				s.logger.Debug("Воркер {worker} остановлен", Logger.Context{
					"worker": worker.Describe(),
				})
				return
			}
		}
	}()

	return result
}

func (s *Scheduler) Stop(id string) {
	if worker, ok := s.workers.Load(id); ok {
		s.logger.Debug("Воркер {worker} остановливается", Logger.Context{
			"worker": worker.(*Worker).Describe(),
		})
		worker.(*Worker).cancel()
	}
}

func (s *Scheduler) IsRunning(id string) bool {
	var _, ok = s.workers.Load(id)

	return ok
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}
