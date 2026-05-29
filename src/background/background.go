package background

import (
	"context"
	"fmt"

	"github.com/EgorKo25/EasyNote/src/logger"
	"golang.org/x/sync/errgroup"
)

type Process interface {
	Run(ctx context.Context) error
	Info() string
}

func NewRunner() *Runner { return &Runner{group: new(errgroup.Group)} }

type Runner struct {
	group *errgroup.Group
}

func (r *Runner) Run(ctx context.Context, processes ...Process) {
	log := logger.Get("background/Run")

	nextGroup, processCtx := errgroup.WithContext(ctx)
	for _, process := range processes {
		nextGroup.Go(func() error {
			return process.Run(processCtx)
		})
		log.Info(process.Info())
	}

	r.group = nextGroup

	log.Info("all background processes have been started")
}

func (r *Runner) Wait() error {
	if err := r.group.Wait(); err != nil {
		return fmt.Errorf("wait background processes: %w", err)
	}

	return nil
}
