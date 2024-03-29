package wait

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"strings"
	"time"
)

// Implement interface
var _ Strategy = (*ExitStrategy)(nil)

// ExitStrategy will wait until container exit
type ExitStrategy struct {
	// all Strategies should have a timeout to avoid waiting infinitely
	exitTimeout time.Duration

	// additional properties
	PollInterval time.Duration

	Code *int
}

//NewExitStrategy constructs with polling interval of 100 milliseconds without timeout by default
func NewExitStrategy() *ExitStrategy {
	return &ExitStrategy{
		PollInterval: defaultPollInterval(),
	}

}

func (ws *ExitStrategy) WithExitCode(code *int) *ExitStrategy {
	ws.Code = code
	return ws
}

// fluent builders for each property
// since go has neither covariance nor generics, the return type must be the type of the concrete implementation
// this is true for all properties, even the "shared" ones

// WithExitTimeout can be used to change the default exit timeout
func (ws *ExitStrategy) WithExitTimeout(exitTimeout time.Duration) *ExitStrategy {
	ws.exitTimeout = exitTimeout
	return ws
}

// WithPollInterval can be used to override the default polling interval of 100 milliseconds
func (ws *ExitStrategy) WithPollInterval(pollInterval time.Duration) *ExitStrategy {
	ws.PollInterval = pollInterval
	return ws
}

// ForExit is the default construction for the fluid interface.
//
// For Example:
// wait.
//     ForExit().
//     WithPollInterval(1 * time.Second)
func ForExit() *ExitStrategy {
	return NewExitStrategy()
}

func IsExited(ctx context.Context, target StrategyTarget) error {
	state, err := target.State(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "No such container") {
			return err
		} else {
			return nil
		}
	}
	if !state.Running {
		return errors.New(fmt.Sprintf("container id :%s is exited", target.GetContainerID()[:12]))
	}
	return nil
}

// WaitUntilReady implements Strategy.WaitUntilReady
func (ws *ExitStrategy) WaitUntilReady(ctx context.Context, target StrategyTarget) (err error) {
	// limit context to exitTimeout
	if ws.exitTimeout > 0 {
		var cancelContext context.CancelFunc
		ctx, cancelContext = context.WithTimeout(ctx, ws.exitTimeout)
		defer cancelContext()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			state, err := target.State(ctx)
			if err != nil {
				if !strings.Contains(err.Error(), "No such container") {
					return err
				} else {
					return nil
				}
			}
			if state.Running {
				time.Sleep(ws.PollInterval)
				continue
			} else {
				if ws.Code != nil {
					if state.ExitCode != *ws.Code {
						return errors.New("exit with wrong exit code")
					}
				}
			}
			return nil
		}
	}
}
