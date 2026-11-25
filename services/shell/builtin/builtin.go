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
	"time"

	"github.com/charmbracelet/x/xpty"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterService(shell.API, resource.DefaultServiceModel, resource.Registration[shell.Service, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context, dep resource.Dependencies, c resource.Config, logger logging.Logger,
		) (shell.Service, error) {
			return NewBuiltIn(c.ResourceName(), logger)
		},
	},
	)
}

// NewBuiltIn returns a new shell service for the given robot.
func NewBuiltIn(name resource.Name, logger logging.Logger) (shell.Service, error) {
	return &builtIn{
		Named:   name.AsNamed(),
		logger:  logger,
		workers: utils.NewBackgroundStoppableWorkers(),
	}, nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyReconfigurable
	logger  logging.Logger
	workers *utils.StoppableWorkers
}

// DoCommand answers environment queries a client cannot resolve on its own,
// e.g. the CLI asking where this machine's VIAM_HOME is before placing files.
func (svc *builtIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := cmd[shell.GetViamHomeCommand]; ok {
		return map[string]interface{}{shell.ViamHomeKey: rutils.ViamDotDir}, nil
	}
	return nil, resource.ErrDoUnimplemented
}

func (svc *builtIn) Shell(clientCtx context.Context, extra map[string]interface{}) (
	chan<- string, chan<- map[string]interface{}, <-chan shell.Output, error,
) {
	defaultShellPath, ok := os.LookupEnv("SHELL")
	if !ok {
		defaultShellPath = "/bin/sh"
	}
	shellArgs := []string{"-i"}
	shellEnv := []string{"TERM=xterm-256color"}
	if runtime.GOOS == "windows" {
		// powershell when available, else cmd.exe; inherit the parent environment (nil).
		defaultShellPath = "cmd.exe"
		if ps, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			defaultShellPath = ps
		}
		shellArgs = nil
		shellEnv = nil
	}

	// Combine `clientCtx` (client's context) with `svc.workers.Context()` (this shell
	// service's lifetime context) by using AfterFunc. When either of `clientCtx` or
	// `svc.workers.Context()` is done, jointClientAndServerCtx will be marked done.
	//
	// Use this jointClientAndServerCtx in all added stoppable workers. The passed-in
	// context will just be `svc.workers.Context()`.
	jointClientAndServerCtx, jointClientAndServerCtxCancel := context.WithCancel(svc.workers.Context())
	context.AfterFunc(clientCtx, jointClientAndServerCtxCancel)

	//nolint: gosec
	cmd := exec.CommandContext(jointClientAndServerCtx, defaultShellPath, shellArgs...)
	cmd.Env = shellEnv
	// xpty gives a unix pty or a Windows ConPTY behind one interface.
	f, err := xpty.NewPty(80, 24)
	if err != nil {
		jointClientAndServerCtxCancel()
		return nil, nil, nil, err
	}
	// xpty.Start doesn't give the child a controlling terminal, so interactive shells lose job control on unix
	setControllingTTY(cmd)
	if err := f.Start(cmd); err != nil {
		jointClientAndServerCtxCancel()
		utils.UncheckedError(f.Close())
		return nil, nil, nil, err
	}

	// Close the PTY on ctx cancel.
	context.AfterFunc(jointClientAndServerCtx, func() { utils.UncheckedError(f.Close()) })

	var sizeLock sync.Mutex
	lastSet := time.Time{}
	procOOB := func(data map[string]interface{}) {
		if len(data) == 0 {
			return
		}
		switch data["message"] {
		case "window-change":
			cols, ok := data["cols"].(float64)
			if !ok {
				svc.logger.CErrorw(jointClientAndServerCtx, "invalid window-change message; expected cols to be float64", "value", data["cols"])
				return
			}
			rows, ok := data["rows"].(float64)
			if !ok {
				svc.logger.CErrorw(jointClientAndServerCtx, "invalid window-change message; expected rows to be float64", "value", data["rows"])
				return
			}
			sizeLock.Lock()
			defer sizeLock.Unlock()
			if time.Since(lastSet) < 200*time.Millisecond {
				svc.logger.CDebug(jointClientAndServerCtx, "too many resizes")
				return
			}
			lastSet = time.Now()
			if err := f.Resize(int(cols), int(rows)); err != nil {
				svc.logger.CErrorw(jointClientAndServerCtx, "error setting pty window size", "error", err)
			}
		default:
			svc.logger.CDebugw(jointClientAndServerCtx, "will not process OOB data")
		}
	}
	if msgs, ok := extra["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			msgM, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			procOOB(msgM)
		}
	}

	svc.workers.Add(func(_ context.Context) {
		defer jointClientAndServerCtxCancel()
		if err := xpty.WaitProcess(jointClientAndServerCtx, cmd); err != nil {
			svc.logger.CDebugw(jointClientAndServerCtx, "error waiting for shell process", "error", err)
		}
		if err := f.Close(); err != nil {
			svc.logger.CDebugw(jointClientAndServerCtx, "error closing pty", "error", err)
		}
	})

	input := make(chan string)
	oobInput := make(chan map[string]interface{})
	output := make(chan shell.Output)

	svc.workers.Add(func(_ context.Context) {
		defer close(output)
		var data [4096]byte
		for {
			select {
			case <-jointClientAndServerCtx.Done():
				return
			default:
			}
			n, err := f.Read(data[:])
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					svc.logger.CErrorw(jointClientAndServerCtx, "error reading output", "error", err)
				}
				select {
				case <-jointClientAndServerCtx.Done():
					return
				case output <- shell.Output{EOF: true}:
				}
				return
			}
			if n == 0 {
				continue
			}
			// a rune can split across pty reads; coerce so the proto send stays valid UTF-8
			text := shell.ValidUTF8(string(data[:n]))
			select {
			case <-jointClientAndServerCtx.Done():
				return
			case output <- shell.Output{Output: text}:
			}
		}
	})

	svc.workers.Add(func(_ context.Context) {
		for {
			select {
			case inputData, ok := <-input:
				if ok {
					if _, err := f.Write([]byte(inputData)); err != nil {
						svc.logger.CErrorw(jointClientAndServerCtx, "error writing data", "error", err)
						return
					}
				} else {
					if _, err := f.Write([]byte{4}); err != nil {
						svc.logger.CErrorw(jointClientAndServerCtx, "error writing EOT", "error", err)
					}
					return
				}
			case <-jointClientAndServerCtx.Done():
				if _, err := f.Write([]byte{4}); err != nil {
					svc.logger.CErrorw(jointClientAndServerCtx, "error writing EOT", "error", err)
				}
				return
			}
		}
	})

	svc.workers.Add(func(_ context.Context) {
		for {
			select {
			case oobInputData, ok := <-oobInput:
				if !ok {
					return
				}
				procOOB(oobInputData)
			case <-jointClientAndServerCtx.Done():
				return
			}
		}
	})

	return input, oobInput, output, nil
}

// CopyFilesToMachine places files from the returned FileCopier
// into the given local destination.
func (svc *builtIn) CopyFilesToMachine(
	ctx context.Context,
	sourceType shell.CopyFilesSourceType,
	destination string,
	preserve bool,
	extra map[string]interface{},
) (shell.FileCopier, error) {
	// we make a temporary factory here just because its reusing some code used
	// elsewhere (like the CLI) that keeps the factory around longer.
	factory, err := shell.NewLocalFileCopyFactory(destination, preserve, true)
	if err != nil {
		return nil, err
	}
	return factory.MakeFileCopier(ctx, sourceType)
}

// CopyFilesFromMachine searches for files locally from the given paths and then
// copies them into a FileCopier created by the given FileCopyFactory.
// Note(erd): a future version of this API may prefer to be more iterator based where
// a shell.CopyFilesSourceType and a new reader type is returned. Right now, it's a tad
// more tightly coupled for the sake of reducing the number of exposed abstractions.
func (svc *builtIn) CopyFilesFromMachine(
	ctx context.Context,
	paths []string,
	allowRecursion bool,
	preserve bool,
	copyFactory shell.FileCopyFactory,
	extra map[string]interface{},
) error {
	reader, err := shell.NewLocalFileReadCopier(paths, allowRecursion, false, copyFactory)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(reader.Close(ctx))
	}()
	return reader.ReadAll(ctx)
}

func (svc *builtIn) Close(ctx context.Context) error {
	svc.workers.Stop()
	return nil
}
