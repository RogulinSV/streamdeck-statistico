package Scheduler

import (
	"time"

	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
)

type Task struct {
	payload     func()
	interval    time.Duration
	description string
}

func NewTask(payload func(), interval time.Duration, description string) Task {
	return Task{
		payload:     payload,
		interval:    interval,
		description: description,
	}
}

type Scheduled struct {
	task    Task
	channel chan bool
	closed  bool
	ticker  *time.Ticker
}

func NewScheduled(task Task, channel chan bool, ticker *time.Ticker) *Scheduled {
	return &Scheduled{
		task:    task,
		channel: channel,
		ticker:  ticker,
		closed:  false,
	}
}

func (s *Scheduled) Close() {
	if !s.closed {
		close(s.channel)
		s.closed = true
	}
}

type Scheduler struct {
	logger    Logger.Logger
	scheduler []*Scheduled
}

func NewScheduler(logger Logger.Logger) Scheduler {
	return Scheduler{
		logger:    logger,
		scheduler: make([]*Scheduled, 0),
	}
}

func (s Scheduler) Schedule(task Task) *Scheduled {
	var ticker *time.Ticker
	var channel chan bool
	var scheduled *Scheduled

	s.logger.Debug("Добавление задачи {task} в планировщик каждые {interval}", Logger.Context{
		"task":     task.description,
		"interval": task.interval,
	})
	ticker = time.NewTicker(task.interval)
	channel = make(chan bool)
	go func(payload func(), logger Logger.Logger) {
		for {
			select {
			case <-ticker.C:
				logger.Debug("Запуск задачи {task} через планировщик", Logger.Context{
					"task": task.description,
				})
				payload()
			case <-channel:
				logger.Debug("Завершение запуска задачи {task} через планировщик", Logger.Context{
					"task": task.description,
				})
				return
			}
		}
	}(task.payload, s.logger)

	scheduled = NewScheduled(task, channel, ticker)
	s.scheduler = append(s.scheduler, scheduled)

	return scheduled
}

func (s Scheduler) Close() {
	for _, scheduled := range s.scheduler {
		scheduled.Close()
	}
}
