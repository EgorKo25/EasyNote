package background_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/EgorKo25/EasyNote/src/background"
)

var (
	errPollingFailed   = errors.New("polling failed")
	errSchedulerFailed = errors.New("scheduler failed")
)

type ProcessFunc func(ctx context.Context) error

func (f ProcessFunc) Run(ctx context.Context) error {
	return f(ctx)
}
func (f ProcessFunc) Info() string { return "test info" }

func TestWaitReturnsProcessError(t *testing.T) {
	t.Parallel()

	runner := background.NewRunner()

	runner.Run(
		context.Background(),
		ProcessFunc(func(context.Context) error {
			return errPollingFailed
		}),
	)

	err := runner.Wait()

	require.ErrorIs(t, err, errPollingFailed)
}

func TestRunCancelsProcessesWhenOneFails(t *testing.T) {
	t.Parallel()

	ctxObserved := make(chan error, 1)
	runner := background.NewRunner()

	runner.Run(
		context.Background(),
		ProcessFunc(func(context.Context) error {
			return errSchedulerFailed
		}),
		ProcessFunc(func(ctx context.Context) error {
			<-ctx.Done()

			ctxObserved <- ctx.Err()

			return nil
		}),
	)

	err := runner.Wait()

	require.ErrorIs(t, err, errSchedulerFailed)
	require.ErrorIs(t, <-ctxObserved, context.Canceled)
}
