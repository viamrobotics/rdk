// Package actuatorstop stops sets of local and remote actuators concurrently.
package actuatorstop

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
)

// RemoteTimeout bounds each Stop sent to an actuator on a remote machine. A lower value
// risks cancelling a stop that a slow but working link would still deliver.
const RemoteTimeout = 5 * time.Second

// Stop calls Stop on every actuator concurrently, passing each its entry in extra, and
// returns the errors by name. Local stops run on ctx with no bound, so a return means they all
// finished. Remote stops are bounded by RemoteTimeout even if they ignore ctx.
func Stop(
	ctx context.Context,
	local, remote map[resource.Name]resource.Actuator,
	extra map[resource.Name]map[string]interface{},
) map[resource.Name]error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs = map[resource.Name]error{}
	)
	start := func(name resource.Name, stop func() error) {
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			if err := stop(); err != nil {
				mu.Lock()
				errs[name] = err
				mu.Unlock()
			}
		})
	}
	for name, actuator := range local {
		start(name, func() error { return stopActuator(ctx, name, actuator, extra[name]) })
	}
	for name, actuator := range remote {
		start(name, func() error { return stopRemoteActuator(ctx, name, actuator, extra[name]) })
	}
	wg.Wait()
	return errs
}

func stopActuator(ctx context.Context, name resource.Name, actuator resource.Actuator, extra map[string]interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("panic stopping %q: %v", name, r)
		}
	}()
	return actuator.Stop(ctx, extra)
}

func stopRemoteActuator(ctx context.Context, name resource.Name, actuator resource.Actuator, extra map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, RemoteTimeout)
	defer cancel()
	errCh := make(chan error, 1)
	utils.PanicCapturingGo(func() {
		errCh <- stopActuator(ctx, name, actuator, extra)
	})
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
