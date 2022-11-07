// Package builtin contains a shell service, along with a gRPC server and client
package builtin

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
)

func init() {
	registry.RegisterService(shell.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, dep registry.Dependencies, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(logger)
		},
	},
	)
}

// NewBuiltIn returns a new shell service for the given robot.
func NewBuiltIn(logger golog.Logger) (shell.Service, error) {
	return &builtIn{logger: logger}, nil
}

type builtIn struct {
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *builtIn) Shell(ctx context.Context, extra map[string]interface{}) (chan<- string, <-chan shell.Output, error) {
	if runtime.GOOS == "windows" {
		return nil, nil, errors.New("shell not supported on windows yet; sorry")
	}
	defaultShellPath, ok := os.LookupEnv("SHELL")
	if !ok {
		defaultShellPath = "/bin/sh"
	}

	ctxCancel, cancel := context.WithCancel(ctx)
	//nolint:gosec
	cmd := exec.CommandContext(ctxCancel, defaultShellPath, "-i")
	f, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	svc.activeBackgroundWorkers.Add(2)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		<-ctx.Done()
	})
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		if err := cmd.Wait(); err != nil {
			svc.logger.Debugw("error waiting for cmd", "error", err)
		}
		if err := f.Close(); err != nil {
			svc.logger.Debugw("error closing pty", "error", err)
		}
	})

	input := make(chan string)
	output := make(chan shell.Output)

	utils.PanicCapturingGo(func() {
		defer close(output)
		var data [64]byte
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := f.Read(data[:])
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					svc.logger.Errorw("error reading output", "error", err)
				}
				select {
				case <-ctx.Done():
					return
				case output <- shell.Output{EOF: true}:
				}
				return
			}
			if n == 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case output <- shell.Output{Output: string(data[:n])}:
			}
		}
	})

	utils.PanicCapturingGo(func() {
		for {
			select {
			case inputData, ok := <-input:
				if ok {
					if _, err := f.Write([]byte(inputData)); err != nil {
						svc.logger.Errorw("error writing data", "error", err)
						return
					}
				} else {
					if _, err := f.Write([]byte{4}); err != nil {
						svc.logger.Errorw("error writing EOT", "error", err)
						return
					}
					return
				}
			case <-ctx.Done():
				if _, err := f.Write([]byte{4}); err != nil {
					svc.logger.Errorw("error writing EOT", "error", err)
					return
				}
			}
		}
	})

	return input, output, nil
}

func (svc *builtIn) Close() {
	svc.activeBackgroundWorkers.Wait()
}
