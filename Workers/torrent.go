package Workers

import (
	"context"

	"github.com/RogulinSV/streamdeck-statistico/v2/Scheduler"
)

type TorrentRunner struct {
}

func NewTorrentRunner() Scheduler.Runner {
	return &TorrentRunner{}
}

func (r *TorrentRunner) Run(context context.Context, result chan<- Scheduler.Result) {

}
