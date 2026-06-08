package scheduler

import (
	"fmt"

	"github.com/EgorKo25/EasyNote/src/config"
)

func NewRunner(
	storage Storage,
	sender Sender,
	planner Planner,
	cfg config.Scheduler,
	opts ...Option,
) *Runner {
	runner := &Runner{
		Storage: storage,
		Sender:  sender,
		Planner: planner,
		cfg:     cfg,
		Clock:   newSystemClock(),
	}

	for _, opt := range opts {
		opt(runner)
	}

	if runner.Retrier == nil {
		runner.Retrier = NewBackoffRetrier(cfg, runner.Clock)
	}

	return runner
}

type Runner struct {
	Storage
	Sender
	Planner
	Retrier
	Clock

	cfg config.Scheduler
}

func (r *Runner) Info() string {
	return fmt.Sprintf("run scheduler reminder runner, with poll interval: %s", r.cfg.PollInterval)
}
