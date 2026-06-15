package main

import (
	"flag"
	"io"
	"os"

	"github.com/RogulinSV/streamdeck-statistico/v2/Filesystem"
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

	var logger = OpenLogger(level, *silent, *output)
	logger.Debug("Программа запущена", Logger.Context{})
}

func OpenLogger(level Logger.Level, silent bool, output string) Logger.Logger {
	var logger Logger.Logger
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
		defer func(logger Logger.Logger, output string) {
			var err = Filesystem.CloseWriters(files)
			if err != nil {
				logger.Error("Не удалось закрыть файл журнала {output}: {error}", Logger.Context{
					"output": output,
					"error":  err,
				})
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

	return logger
}
