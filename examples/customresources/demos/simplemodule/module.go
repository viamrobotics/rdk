// Package main is a module with a built-in "counter" component model, that will simply track numbers.
// It uses the rdk:component:generic interface for simplicity.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
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
	scriptPath, err := filepath.Abs("hello_printer.sh")
	if err != nil {
		return nil, fmt.Errorf("error getting script path: %v\n", err)
	}
	subCmd := exec.Command("/bin/bash", scriptPath)
	subStdout, err := subCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating STDOUT pipe for process: %v", err.Error())
	}

	go func() {
		// Start seemingly needs to happen in a goroutine for the process to be leaked.
		if err := subCmd.Start(); err != nil {
			log.Printf("Error starting command for process: %v", err.Error())
			return
		}
		log.Printf("Started hello_printer.sh subprocess with PID %d\n", subCmd.Process.Pid)

		go func() {
			// Async `Wait` as it is blocking.
			if err := subCmd.Wait(); err != nil {
				log.Printf("Error waiting on command for process: %v", err.Error())
				return
			}
			log.Printf("hello_printer.sh subprocess with PID %d has been successfully waited upon\n", subCmd.Process.Pid)
		}()

		go func() {
			// Async `Copy` between the subcommand's STDOUT and "our" STDOUT. The hope is that
			// this leaked writing will force `viam-server`'s `cmd.Wait` for this module to hang
			// even after `simplemodule` is dead, as it will still be trying to copy from the
			// STDOUT pipe it set out for `simplemodule`, since `hello_printer.sh` is still
			// writing to it.
			log.Printf("Starting to copy between hello_printer.sh's STDOUT and 'our' STDOUT\n")
			if _, err := io.Copy(os.Stdout, subStdout); err != nil {
				log.Printf("Error copying between STDOUTs: %v", err.Error())
				return
			}
			log.Printf("Stopped copying between hello_printer.sh's STDOUT and 'our' STDOUT\n")
		}()
	}()

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
