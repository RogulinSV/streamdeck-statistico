package Context

import (
	"context"
	"os"
	"os/signal"
	"time"
)

func WithTimeout(timeout uint, parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	return context.WithTimeout(parent, time.Duration(timeout)*time.Second)
}

func WithCancel(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(parent)
}

func HandleSignal(signals ...os.Signal) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), signals...)
}

func Canceled(c context.Context) bool {
	return c.Err() != nil
}
