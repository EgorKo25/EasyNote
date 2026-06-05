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
		storage: storage,
		sender:  sender,
		planner: planner,
		cfg:     cfg,
		clock:   newSystemClock(),
	}

	for _, opt := range opts {
		opt(runner)
	}

	if runner.retrier == nil {
		runner.retrier = NewBackoffRetrier(cfg, runner.clock)
	}

	return runner
}

type Runner struct {
	storage Storage
	sender  Sender
	planner Planner
	retrier Retrier
	cfg     config.Scheduler
	clock   Clock
}

func (r *Runner) Info() string {
	return fmt.Sprintf("run scheduler reminder runner, with poll interval: %s", r.cfg.PollInterval)
}
