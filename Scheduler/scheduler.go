package Scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Context"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
)

type Metric struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Units string `json:"units"`
}

func NewMetric(title string, value string) Metric {
	return Metric{
		Title: title,
		Value: value,
		Units: "",
	}
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
	if metric, ok := r[name]; ok {
		return metric
	}

	return Metric{
		Title: "",
		Value: "",
		Units: "",
	}
}

func AddMetric(name string, metric Metric) Result {
	return NewResult().AddMetric(name, metric)
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
	output  chan Result
}

func NewWorker(id string, timeout time.Duration, delay time.Duration, runner Runner) *Worker {
	return &Worker{
		id:      id,
		timeout: timeout,
		delay:   delay,
		runner:  runner,
	}
}

func (w *Worker) Run(context context.Context) {
	w.runner.Run(context, w.output)
}

func (w *Worker) Done() <-chan struct{} {
	return w.context.Done()
}

func (w *Worker) Output() <-chan Result {
	return w.output
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

func (s *Scheduler) Start(worker *Worker) {
	s.Stop(worker)

	worker.output = make(chan Result)
	worker.context, worker.cancel = Context.WithCancel(s.context)
	s.workers.Store(worker.id, worker)
	s.wg.Add(1)
	var done = func() {
		s.wg.Done()
		s.workers.Delete(worker.id)
	}

	go func() {
		defer done()

		s.logger.Debug("Воркер {worker} запущен", Logger.Context{
			"worker": worker.Describe(),
		})

		var timeout context.Context
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
				timeout, cancel = Context.WithTimeout(uint(worker.timeout.Seconds()), worker.context)
			} else {
				timeout, cancel = Context.WithCancel(worker.context)
			}
			s.logger.Debug("Запуск задачи воркера {worker}", Logger.Context{
				"worker": worker.Describe(),
			})
			start = time.Now()
			worker.Run(timeout)
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
}

func (s *Scheduler) Stop(worker *Worker) {
	if w, ok := s.workers.Load(worker.id); ok {
		s.logger.Debug("Воркер {worker} остановливается", Logger.Context{
			"worker": w.(*Worker).Describe(),
		})
		w.(*Worker).cancel()
		close(w.(*Worker).output)
	}
}

func (s *Scheduler) GetWorker(id string) *Worker {
	if worker, ok := s.workers.Load(id); ok {
		return worker.(*Worker)
	}

	return nil
}

func (s *Scheduler) HasWorker(id string) bool {
	return s.GetWorker(id) != nil
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}
