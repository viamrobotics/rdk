// Package main is a module with a built-in "counter" component model, that will simply track numbers.
// It uses the rdk:component:generic interface for simplicity.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var myModel = resource.NewModel("acme", "demo", "mycounter")

func main() {
	// Start the hello printer subprocess
	scriptPath, err := filepath.Abs("hello_printer.sh")
	if err != nil {
		fmt.Printf("Error getting script path: %v\n", err)
	} else {
		cmd := exec.Command("/bin/bash", scriptPath)
		stdout, err := cmd.StdoutPipe()
		// Get the stdout pipe
		if err != nil {
			panic(err)
		}
		// Start the subprocess in the background
		go func() {
			// Start the command
			if err := cmd.Start(); err != nil {
				panic(err)
			}

			// Use TeeReader to read from stdout and write to both buffer and os.Stdout
			var buf bytes.Buffer
			teeReader := io.TeeReader(stdout, &buf)

			// Copy everything to stdout (this will also populate the buffer)
			io.Copy(os.Stdout, teeReader)

			fmt.Printf("Started hello printer subprocess with PID: %d\n", cmd.Process.Pid)

			// Wait for the process to finish (it won't since it's an infinite loop)
			cmd.Wait()
		}()
	}

	// We first put our component's constructor in the registry, then tell the module to load it
	// Note that all resources must be added before the module is started.
	resource.RegisterComponent(generic.API, myModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: newCounter,
	})

	// Next, we run a module which will have a singl model.
	module.ModularMain(resource.APIModel{generic.API, myModel})
}

// newCounter is used to create a new instance of our specific model. It is called for each component in the robot's config with this model.
func newCounter(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	return &counter{
		Named: conf.ResourceName().AsNamed(),
	}, nil
}

// counter is the representation of this model. It holds only a "total" count.
type counter struct {
	resource.Named
	resource.TriviallyCloseable
	total int64
}

func (c *counter) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	atomic.StoreInt64(&c.total, 0)
	return nil
}

// DoCommand is the only method of this component. It looks up the "real" command from the map it's passed.
// Because of this, any arbitrary commands can be received, and any data returned.
func (c *counter) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	// We look for a map key called "command"
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	// If it's "get" we return the current total.
	if cmd == "get" {
		return map[string]interface{}{"total": atomic.LoadInt64(&c.total)}, nil
	}

	// If it's "add" we atomically add a second key "value" to the total.
	if cmd == "add" {
		_, ok := req["value"]
		if !ok {
			return nil, errors.New("value must exist")
		}
		val, ok := req["value"].(float64)
		if !ok {
			return nil, errors.New("value must be a number")
		}
		atomic.AddInt64(&c.total, int64(val))
		// We return the new total after the addition.
		return map[string]interface{}{"total": atomic.LoadInt64(&c.total)}, nil
	}
	// The command must've been something else.
	return nil, fmt.Errorf("unknown command string %s", cmd)
}
