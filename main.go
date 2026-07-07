package main

import (
	"flag"
	"io"
	"os"
	"syscall"

	"github.com/RogulinSV/streamdeck-statistico/v2/Context"
	"github.com/RogulinSV/streamdeck-statistico/v2/FileSystem"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
	"github.com/RogulinSV/streamdeck-statistico/v2/Workers"
	"github.com/RogulinSV/streamdeck-statistico/v2/Workers/Torrent"
)

var pwd string

func init() {
	var err error

	pwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

func main() {
	var port = flag.Uint("port", 8080, "Порт сервера")
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

	var logger = NewLogger("[kernel] ", level, *silent, *output)
	defer logger.Close()

	logger.Info("Программа запущена", Logger.Context{})
	var context, cancel = Context.HandleSignal(syscall.SIGINT, syscall.SIGTERM)
	var scheduler = Scheduler.NewScheduler(context, logger.WithPrefix("[scheduler] "))
	defer cancel()

	logger.Debug("Загрузка фоновых задач", Logger.Context{})
	var registry = Workers.NewWorkerRegistry()
	registry.Add("torrent", Torrent.NewRunner(logger.WithPrefix("[worker.torrent] ")), 10, 5)
	logger.Info("Загружено фоновых задач: {count}", Logger.Context{
		"count": registry.Count(),
	})

	logger.Debug("Запуск сервера в фоновом режиме", Logger.Context{})
	go func(logger Logger.Logger) {
		var server = Workers.NewServerRunner(uint16(*port), scheduler, registry, logger)
		var worker = Scheduler.NewWorker("server", 0, 0, server)
		scheduler.Start(worker)

		for {
			select {
			case result, ok := <-worker.Output():
				if ok {
					logger.Debug("Изменилось количество клиентов: {value}", Logger.Context{
						"value": result.GetMetric("clients").Value,
					})
				}
			case <-worker.Done():
				return
			case <-context.Done():
				return
			}
		}
	}(logger.WithPrefix("[worker:server] "))

	<-context.Done()
	logger.Debug("Завершение программы", Logger.Context{})
	scheduler.Wait()
	logger.Info("Программа завершена", Logger.Context{})
}

// NewLogger возвращает журнал с функцией очистки по завершению работы
func NewLogger(prefix string, level Logger.Level, silent bool, output string) Logger.ClosableLogger {
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
		var file, err = FileSystem.ResolveFilePath(output)
		if err != nil {
			logger.Error("Не удалось проверить файл журнала {output}: {error}", Logger.Context{
				"output": output,
				"error":  err,
			})
			file = FileSystem.SplitFilePath(pwd, "debug.log")
		}
		writers, err = FileSystem.OpenWriters(file)
		cleanup = func(logger Logger.Logger, output string) func() {
			return func() {
				logger.Debug("...", Logger.Context{})
				var err = FileSystem.CloseWriters(files)
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

	return Logger.NewClosableLogger(logger, cleanup).WithPrefix(prefix).(Logger.ClosableLogger)
}
