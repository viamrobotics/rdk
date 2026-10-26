// Package builtin contains a shell service, along with a gRPC server and client
package builtin

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
		Named:  name.AsNamed(),
		logger: logger,
	}, nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyReconfigurable
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// utf8Chunker reassembles pty reads so every emitted chunk is valid UTF-8. A multibyte
// rune can straddle a read boundary, and proto3 string fields reject invalid UTF-8, so a
// split rune would otherwise fail to marshal and tear down the whole shell stream.
type utf8Chunker struct {
	carry []byte
}

// push returns the portion of this read that is safe to send now, holding back a trailing
// incomplete rune for the next read. Genuinely malformed bytes are replaced rather than
// dropped so the stream survives non-UTF-8 output.
func (c *utf8Chunker) push(b []byte) string {
	chunk := make([]byte, 0, len(c.carry)+len(b))
	chunk = append(append(chunk, c.carry...), b...)
	boundary := safeUTF8Boundary(chunk)
	out := string(chunk[:boundary])
	c.carry = append([]byte(nil), chunk[boundary:]...)
	if !utf8.ValidString(out) {
		out = strings.ToValidUTF8(out, "�")
	}
	return out
}

// flush returns any held-back trailing bytes, sanitized, for use at EOF.
func (c *utf8Chunker) flush() string {
	if len(c.carry) == 0 {
		return ""
	}
	out := strings.ToValidUTF8(string(c.carry), "�")
	c.carry = nil
	return out
}

// safeUTF8Boundary returns the length of the prefix of b whose final rune is complete.
// Only a trailing incomplete multibyte sequence (at most utf8.UTFMax-1 bytes) is withheld;
// an invalid-but-complete byte counts as whole so it can be sanitized downstream.
func safeUTF8Boundary(b []byte) int {
	for i := len(b) - 1; i >= 0 && i > len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.FullRune(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}

// DoCommand answers environment queries a client cannot resolve on its own,
// e.g. the CLI asking where this machine's VIAM_HOME is before placing files.
func (svc *builtIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := cmd[shell.GetViamHomeCommand]; ok {
		return map[string]interface{}{shell.ViamHomeKey: rutils.ViamDotDir}, nil
	}
	return nil, resource.ErrDoUnimplemented
}

func (svc *builtIn) Shell(ctx context.Context, extra map[string]interface{}) (
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

	ctxCancel, cancel := context.WithCancel(ctx)
	//nolint: gosec,noctx
	cmd := exec.Command(defaultShellPath, shellArgs...)
	cmd.Env = shellEnv
	// xpty gives a unix pty or a Windows ConPTY behind one interface.
	f, err := xpty.NewPty(80, 24)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	// xpty.Start doesn't give the child a controlling terminal, so interactive shells lose job control on unix
	setControllingTTY(cmd)
	if err := f.Start(cmd); err != nil {
		cancel()
		utils.UncheckedError(f.Close())
		return nil, nil, nil, err
	}

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
				svc.logger.CErrorw(ctx, "invalid window-change message; expected cols to be float64", "value", data["cols"])
				return
			}
			rows, ok := data["rows"].(float64)
			if !ok {
				svc.logger.CErrorw(ctx, "invalid window-change message; expected rows to be float64", "value", data["rows"])
				return
			}
			sizeLock.Lock()
			defer sizeLock.Unlock()
			if time.Since(lastSet) < 200*time.Millisecond {
				svc.logger.CDebug(ctx, "too many resizes")
				return
			}
			lastSet = time.Now()
			if err := f.Resize(int(cols), int(rows)); err != nil {
				svc.logger.CErrorw(ctx, "error setting pty window size", "error", err)
			}
		default:
			svc.logger.CDebugw(ctx, "will not process OOB data")
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

	svc.activeBackgroundWorkers.Add(2)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		<-ctx.Done()
	})
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		if err := xpty.WaitProcess(ctxCancel, cmd); err != nil {
			svc.logger.CDebugw(ctx, "error waiting for shell process", "error", err)
		}
		if err := f.Close(); err != nil {
			svc.logger.CDebugw(ctx, "error closing pty", "error", err)
		}
	})

	input := make(chan string)
	oobInput := make(chan map[string]interface{})
	output := make(chan shell.Output)

	utils.PanicCapturingGo(func() {
		defer close(output)
		var data [4096]byte
		var chunker utf8Chunker
		send := func(s string) bool {
			if s == "" {
				return true
			}
			select {
			case <-ctx.Done():
				return false
			case output <- shell.Output{Output: s}:
				return true
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := f.Read(data[:])
			if err != nil {
				// flush any held-back trailing rune before signaling EOF so it isn't dropped
				send(chunker.flush())
				if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					svc.logger.CErrorw(ctx, "error reading output", "error", err)
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
			if !send(chunker.push(data[:n])) {
				return
			}
		}
	})

	utils.PanicCapturingGo(func() {
		for {
			select {
			case inputData, ok := <-input:
				if ok {
					if _, err := f.Write([]byte(inputData)); err != nil {
						svc.logger.CErrorw(ctx, "error writing data", "error", err)
						return
					}
				} else {
					if _, err := f.Write([]byte{4}); err != nil {
						svc.logger.CErrorw(ctx, "error writing EOT", "error", err)
						return
					}
					return
				}
			case <-ctx.Done():
				if _, err := f.Write([]byte{4}); err != nil {
					svc.logger.CErrorw(ctx, "error writing EOT", "error", err)
					return
				}
			}
		}
	})

	utils.PanicCapturingGo(func() {
		for {
			select {
			case oobInputData, ok := <-oobInput:
				if !ok {
					return
				}
				procOOB(oobInputData)
			case <-ctxCancel.Done():
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
	svc.activeBackgroundWorkers.Wait()
	return nil
}
