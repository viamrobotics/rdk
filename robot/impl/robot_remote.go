package robotimpl

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

var errUnimplemented = errors.New("unimplemented")

func remoteDialOptions(config config.Remote, opts resourceManagerOptions) []rpc.DialOption {
	var dialOpts []rpc.DialOption
	if opts.debug {
		dialOpts = append(dialOpts, rpc.WithDialDebug())
	}
	if config.Insecure {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}
	if opts.allowInsecureCreds {
		dialOpts = append(dialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}
	if opts.tlsConfig != nil {
		dialOpts = append(dialOpts, rpc.WithTLSConfig(opts.tlsConfig))
	}
	if config.Auth.Credentials != nil {
		if config.Auth.Entity == "" {
			dialOpts = append(dialOpts, rpc.WithCredentials(*config.Auth.Credentials))
		} else {
			dialOpts = append(dialOpts, rpc.WithEntityCredentials(config.Auth.Entity, *config.Auth.Credentials))
		}
	} else {
		// explicitly unset credentials so they are not fed to remotes unintentionally.
		dialOpts = append(dialOpts, rpc.WithEntityCredentials("", rpc.Credentials{}))
	}

	if config.Auth.ExternalAuthAddress != "" {
		dialOpts = append(dialOpts, rpc.WithExternalAuth(
			config.Auth.ExternalAuthAddress,
			config.Auth.ExternalAuthToEntity,
		))
	}

	if config.Auth.ExternalAuthInsecure {
		dialOpts = append(dialOpts, rpc.WithExternalAuthInsecure())
	}

	if config.Auth.SignalingServerAddress != "" {
		wrtcOpts := rpc.DialWebRTCOptions{
			Config:                 &rpc.DefaultWebRTCConfiguration,
			SignalingServerAddress: config.Auth.SignalingServerAddress,
			SignalingAuthEntity:    config.Auth.SignalingAuthEntity,
		}
		if config.Auth.SignalingCreds != nil {
			wrtcOpts.SignalingCreds = *config.Auth.SignalingCreds
		}
		dialOpts = append(dialOpts, rpc.WithWebRTCOptions(wrtcOpts))

		if config.Auth.Managed {
			// managed robots use TLS authN/Z
			dialOpts = append(dialOpts, rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{
				RemoveAuthCredentials: true,
			}))
		}
	}
	return dialOpts
}

func dialRemote(ctx context.Context, config config.Remote, logger golog.Logger, dialOpts ...rpc.DialOption) (robot.RemoteRobot, error) {
	var outerError error
	for attempt := 0; attempt < 3; attempt++ {
		robotClient, err := client.New(
			ctx,
			config.Address,
			logger,
			client.WithDialOptions(dialOpts...),
			client.WithCheckConnectedEvery(config.ConnectionCheckInterval),
			client.WithReconnectEvery(config.ReconnectInterval),
		)
		if err != nil {
			outerError = err
			continue
		}
		return robotClient, nil
	}
	return nil, outerError
}

type changeable interface {
	// Changed watches for whether the remote has changed.
	Changed() <-chan bool
}

// A remoteRobot implements wraps an robot.Robot. It
// assists in the un/prefixing of part names for RemoteRobots that
// are not aware they are integrated elsewhere.
// We intentionally do not promote the underlying robot.Robot
// so that any future changes are forced to consider un/prefixing
// of names.
type remoteRobot struct {
	mu      sync.Mutex
	robot   robot.RemoteRobot
	conf    config.Remote
	manager *resourceManager

	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers func()
}

// newRemoteRobot returns a new remote robot wrapping a given robot.Robot
// and its configuration.
func newRemoteRobot(ctx context.Context, robot robot.RemoteRobot, config config.Remote) *remoteRobot {
	// We pull the manager out here such that we correctly return nil for
	// when parts are accessed. This is because a networked robot client
	// may just return a non-nil wrapper for a part they may not exist.
	remoteManager := managerForRemoteRobot(robot)

	remote := &remoteRobot{
		robot:   robot,
		conf:    config,
		manager: remoteManager,
	}

	remote.startWatcher(ctx)
	return remote
}

func (rr *remoteRobot) startWatcher(ctx context.Context) {
	rr.activeBackgroundWorkers.Add(1)
	cancelCtx, cancel := context.WithCancel(ctx)
	rr.cancelBackgroundWorkers = cancel
	changed, ok := rr.robot.(changeable)
	if !ok {
		return
	}
	utils.ManagedGo(func() {
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
			}
			select {
			case <-cancelCtx.Done():
				return
			case <-changed.Changed():
				rr.mu.Lock()
				if rr.robot.Connected() {
					newManager := managerForRemoteRobot(rr.robot)
					rr.manager.replaceForRemote(cancelCtx, newManager)
				}
				rr.mu.Unlock()
			}
		}
	}, func() {
		rr.activeBackgroundWorkers.Done()
	})
}

func (rr *remoteRobot) checkConnected() error {
	if !rr.robot.Connected() {
		return errors.Errorf("not connected to remote robot %q at %s", rr.conf.Name, rr.conf.Address)
	}
	return nil
}

func (rr *remoteRobot) Refresh(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if err := rr.checkConnected(); err != nil {
		return err
	}
	refresher, ok := rr.robot.(robot.Refresher)
	if !ok {
		return nil
	}
	if err := refresher.Refresh(ctx); err != nil {
		return err
	}
	rr.manager = managerForRemoteRobot(rr.robot)
	return nil
}

// replace replaces this robot with the given robot. We can do a full
// replacement here since we will always get a full view of the parts,
// not one partially created from a diff.
func (rr *remoteRobot) replace(ctx context.Context, newRobot robot.Robot) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	actual, ok := newRobot.(*remoteRobot)
	if !ok {
		panic(fmt.Errorf("expected new remote to be %T but got %T", actual, newRobot))
	}

	rr.manager.replaceForRemote(ctx, actual.manager)
	if err := rr.Close(ctx); err != nil {
		rr.Logger().Errorw("error closing replaced remote robot client", "error", err)
	}
	rr.robot = actual.robot
	rr.conf = actual.conf
	rr.startWatcher(ctx)
}

func (rr *remoteRobot) prefixName(name string) string {
	if rr.conf.Prefix {
		return fmt.Sprintf("%s.%s", rr.conf.Name, name)
	}
	return name
}

func (rr *remoteRobot) unprefixName(name string) string {
	if rr.conf.Prefix {
		return strings.TrimPrefix(name, rr.conf.Name+".")
	}
	return name
}

func (rr *remoteRobot) prefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.prefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) unprefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.unprefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) RemoteNames() []string {
	return nil
}

func (rr *remoteRobot) ResourceNames() []resource.Name {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if err := rr.checkConnected(); err != nil {
		rr.Logger().Errorw("failed to get remote resource names", "error", err)
		return []resource.Name{}
	}
	newNames := make([]resource.Name, 0, len(rr.manager.ResourceNames()))
	for _, name := range rr.manager.ResourceNames() {
		name := rr.prefixResourceName(name)
		newNames = append(newNames, name)
	}
	return newNames
}

func (rr *remoteRobot) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rr *remoteRobot) ResourceByName(name resource.Name) (interface{}, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if err := rr.checkConnected(); err != nil {
		return nil, err
	}
	newName := rr.unprefixResourceName(name)
	return rr.manager.ResourceByName(newName)
}

func (rr *remoteRobot) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

func (rr *remoteRobot) OperationManager() *operation.Manager {
	return rr.robot.OperationManager()
}

func (rr *remoteRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *remoteRobot) Close(ctx context.Context) error {
	if rr.cancelBackgroundWorkers != nil {
		rr.cancelBackgroundWorkers()
	}
	rr.activeBackgroundWorkers.Wait()
	return utils.TryClose(ctx, rr.robot)
}

// managerForRemoteRobot integrates all parts from a given robot
// except for its remotes. This is for a remote robot to integrate
// which should be unaware of remotes.
// Be sure to update this function if resourceManager grows.
func managerForRemoteRobot(robot robot.Robot) *resourceManager {
	manager := newResourceManager(resourceManagerOptions{}, robot.Logger().Named("manager"))

	for _, name := range robot.ResourceNames() {
		part, err := robot.ResourceByName(name)
		if err != nil {
			robot.Logger().Debugw("error getting resource", "resource", name, "error", err)
			continue
		}
		manager.addResource(name, part)
	}
	return manager
}

// replaceForRemote replaces these parts with the given parts coming from a remote.
func (manager *resourceManager) replaceForRemote(ctx context.Context, newManager *resourceManager) {
	oldResources := resource.NewGraph()

	if len(manager.resources.Nodes) != 0 {
		for name := range manager.resources.Nodes {
			oldResources.AddNode(name, struct{}{})
		}
	}

	for name, newR := range newManager.resources.Nodes {
		old, ok := manager.resources.Nodes[name]
		if ok {
			oldResources.Remove(name)
			oldPart, oldIsReconfigurable := old.(resource.Reconfigurable)
			newPart, newIsReconfigurable := newR.(resource.Reconfigurable)

			switch {
			case oldIsReconfigurable != newIsReconfigurable:
				// this is an indicator of a serious constructor problem
				// for the resource subtype.
				if oldIsReconfigurable {
					panic(fmt.Errorf(
						"old type %T is reconfigurable whereas new type %T is not",
						old, newR))
				}
				panic(fmt.Errorf(
					"new type %T is reconfigurable whereas old type %T is not",
					newR, old))
			case oldIsReconfigurable && newIsReconfigurable:
				// if we are dealing with a reconfigurable resource
				// use the new resource to reconfigure the old one.
				if err := oldPart.Reconfigure(ctx, newPart); err != nil {
					panic(err)
				}
				continue
			case !oldIsReconfigurable && !newIsReconfigurable:
				// if we are not dealing with a reconfigurable resource
				// we want to close the old resource and replace it with the
				// new.
				if err := utils.TryClose(ctx, old); err != nil {
					panic(err)
				}
			}
		}

		manager.resources.Nodes[name] = newR
	}

	for name := range oldResources.Nodes {
		manager.resources.Remove(name)
	}
}
