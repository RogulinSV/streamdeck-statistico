package FileSystem

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrEmptyPath = errors.New("path is empty")
	ErrIsDir     = errors.New("path is a directory")
)

func ResolveFilePath(input string) (string, error) {
	var output string
	var info os.FileInfo
	var err error

	if input == "" {
		return "", ErrEmptyPath
	}

	output = filepath.Clean(input)
	output, err = filepath.Abs(output)
	if err != nil {
		return "", err
	}
	output, err = filepath.EvalSymlinks(output)
	if err != nil {
		return "", err
	}
	info, err = os.Stat(output)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
	} else if info.IsDir() {
		return "", ErrIsDir
	}

	return output, nil
}

func SplitFilePath(input ...string) string {
	return strings.Join(input, string(os.PathSeparator))
}

func OpenWriters(input ...string) ([]io.Writer, error) {
	var output []io.Writer
	var file *os.File
	var mode = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	var perm os.FileMode = 0744
	var err error

	for _, path := range input {
		path, err = ResolveFilePath(path)
		if err != nil {
			return nil, errors.Join(err, CloseWriters(output))
		}
		file, err = os.OpenFile(path, mode, perm)
		if err != nil {
			return nil, errors.Join(err, CloseWriters(output))
		}
		output = append(output, file)
	}

	return output, nil
}

func CloseWriters(writers []io.Writer) error {
	var writer io.Writer
	var err error
	var errs []error

	for _, writer = range writers {
		switch writer.(type) {
		case *os.File:
			err = writer.(*os.File).Close()
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
