package main

import (
	"flag"
	"io"
	"os"

	"github.com/RogulinSV/streamdeck-statistico/v2/FileSystem"
	"github.com/RogulinSV/streamdeck-statistico/v2/Logger"
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
	// var port = flag.Int("port", 8080, "Порт сервера")
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

	var logger = NewLogger(level, *silent, *output)
	defer logger.Close()

	logger.Info("Программа запущена", Logger.Context{})
	var l1 = logger.WithPrefix("[1] ")
	var l2 = logger.WithPrefix("[2] ")
	l2.Debug("test {num}", Logger.Context{"num": 1})
	l1.Debug("test {num}", Logger.Context{"num": 2})
	l2.Warn("test {num}", Logger.Context{"num": 3})
}

// NewLogger returns new logger instance with cleanup function
func NewLogger(level Logger.Level, silent bool, output string) Logger.ClosableLogger {
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

	return Logger.NewClosableLogger(logger, cleanup)
}
