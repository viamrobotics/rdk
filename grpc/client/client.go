// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/metadata"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address        string
	conn           rpc.ClientConn
	client         pb.RobotServiceClient
	metadataClient metadata.Service
	dialOptions    []rpc.DialOption

	mu            *sync.RWMutex
	resourceNames []resource.Name

	connected  bool
	changeChan chan bool

	activeBackgroundWorkers *sync.WaitGroup
	cancelBackgroundWorkers func()
	logger                  golog.Logger

	closeContext context.Context
}

// New constructs a new RobotClient that is served at the given address. The given
// context can be used to cancel the operation.
func New(ctx context.Context, address string, logger golog.Logger, opts ...RobotClientOption) (*RobotClient, error) {
	var rOpts robotClientOpts
	for _, opt := range opts {
		opt.apply(&rOpts)
	}

	closeCtx, cancel := context.WithCancel(ctx)

	rc := &RobotClient{
		address:                 address,
		cancelBackgroundWorkers: cancel,
		mu:                      &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		closeContext:            closeCtx,
		dialOptions:             rOpts.dialOptions,
	}
	if err := rc.connect(ctx); err != nil {
		return nil, err
	}

	// refresh once to hydrate the robot.
	if err := rc.Refresh(ctx); err != nil {
		return nil, multierr.Combine(err, rc.conn.Close())
	}

	if rOpts.refreshEvery != 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, rOpts.refreshEvery)
		}, rc.activeBackgroundWorkers.Done)
	}

	if rOpts.checkConnectedEvery != 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.checkConnection(closeCtx, rOpts.checkConnectedEvery, rOpts.reconnectEvery)
		}, rc.activeBackgroundWorkers.Done)
	}

	return rc, nil
}

// Connected exposes whether a robot client is connected to the remote.
func (rc *RobotClient) Connected() bool {
	return rc.connected
}

// Changed watches for whether the remote has changed.
func (rc *RobotClient) Changed() <-chan bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.changeChan == nil {
		rc.changeChan = make(chan bool)
	}
	return rc.changeChan
}

func (rc *RobotClient) connect(ctx context.Context) error {
	if rc.conn != nil {
		if err := rc.conn.Close(); err != nil {
			return err
		}
	}
	conn, err := grpc.Dial(ctx, rc.address, rc.logger, rc.dialOptions...)
	if err != nil {
		return err
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	client := pb.NewRobotServiceClient(conn)
	metadataClient := metadata.NewClientFromConn(ctx, conn, "", rc.logger)
	rc.conn = conn
	rc.client = client
	rc.metadataClient = metadataClient
	rc.connected = true
	if rc.changeChan != nil {
		rc.changeChan <- true
	}
	return nil
}

// checkConnection either checks if the client is still connected, or attempts to reconnect to the remote.
func (rc *RobotClient) checkConnection(ctx context.Context, checkEvery time.Duration, reconnectEvery time.Duration) {
	for {
		var waitTime time.Duration
		if rc.Connected() {
			waitTime = checkEvery
		} else {
			if reconnectEvery != 0 {
				waitTime = reconnectEvery
			} else {
				// if reconnectEvery is unset, we will not attempt to reconnect
				return
			}
		}
		if !utils.SelectContextOrWait(ctx, waitTime) {
			return
		}
		if !rc.Connected() {
			rc.Logger().Debugf("trying to reconnect to remote at address %v", rc.address)
			if err := rc.connect(ctx); err != nil {
				rc.Logger().Debugw(fmt.Sprintf("failed to reconnect remote at address %v", rc.address), "error", err, "address", rc.address)
				continue
			}
			rc.Logger().Debugf("successfully reconnected remote at address %v", rc.address)
		} else {
			check := func() error {
				timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				if _, err := rc.metadataClient.Resources(timeoutCtx); err != nil {
					return err
				}
				return nil
			}
			var outerError error
			for attempt := 0; attempt < 3; attempt++ {
				err := check()
				if err != nil {
					outerError = err
					// if pipe is closed, we know for sure we lost connection
					if strings.Contains(err.Error(), "read/write on closed pipe") {
						break
					} else {
						// otherwise retry
						continue
					}
				} else {
					outerError = nil
					break
				}
			}
			if outerError != nil {
				rc.Logger().Errorw(
					fmt.Sprintf("lost connection to remote at address %v, retrying in %v sec", rc.address, reconnectEvery.Seconds()),
					"error", outerError,
					"address", rc.address,
				)
				rc.mu.Lock()
				rc.connected = false
				if rc.changeChan != nil {
					rc.changeChan <- true
				}
				rc.mu.Unlock()
			}
		}
	}
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()
	if rc.changeChan != nil {
		close(rc.changeChan)
		rc.changeChan = nil
	}
	return rc.conn.Close()
}

func (rc *RobotClient) checkConnected() error {
	if !rc.Connected() {
		return errors.Errorf("not connected to remote robot at %s", rc.address)
	}
	return nil
}

// RefreshEvery refreshes the robot on the interval given by every until the
// given context is done.
func (rc *RobotClient) RefreshEvery(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
			return
		}
		if err := rc.Refresh(ctx); err != nil {
			// we want to keep refreshing and hopefully the ticker is not
			// too fast so that we do not thrash.
			rc.Logger().Errorw("failed to refresh status", "error", err)
		}
	}
}

// RemoteByName returns a remote robot by name. It is assumed to exist on the
// other end. Right now this method is unimplemented.
func (rc *RobotClient) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

// ResourceByName returns resource by name.
func (rc *RobotClient) ResourceByName(name resource.Name) (interface{}, error) {
	if err := rc.checkConnected(); err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(name.Subtype)
	if c == nil || c.RPCClient == nil {
		// registration doesn't exist
		return nil, errors.New("resource client registration doesn't exist")
	}
	// pass in conn
	resourceClient := c.RPCClient(rc.closeContext, rc.conn, name.Name, rc.Logger())
	return resourceClient, nil
}

// Refresh manually updates the underlying parts of the robot based
// on its metadata response.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if err := rc.checkConnected(); err != nil {
		return err
	}

	// call metadata service.
	names, err := rc.metadataClient.Resources(ctx)
	// only return if it is not unimplemented - means a bigger error came up
	if err != nil && grpcstatus.Code(err) != codes.Unimplemented {
		return err
	}
	if err == nil {
		rc.resourceNames = make([]resource.Name, 0, len(names))
		rc.resourceNames = append(rc.resourceNames, names...)
	}
	return nil
}

// RemoteNames returns the names of all known remotes.
func (rc *RobotClient) RemoteNames() []string {
	return nil
}

// ProcessManager returns a useless process manager for the sake of
// satisfying the robot.Robot interface. Maybe it should not be part
// of the interface!
func (rc *RobotClient) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

// OperationManager returns nil.
func (rc *RobotClient) OperationManager() *operation.Manager {
	return nil
}

// ResourceNames returns all resource names.
func (rc *RobotClient) ResourceNames() []resource.Name {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if err := rc.checkConnected(); err != nil {
		rc.Logger().Errorw("failed to get remote resource names", "error", err)
		return []resource.Name{}
	}
	names := []resource.Name{}
	for _, v := range rc.resourceNames {
		names = append(
			names,
			resource.NewName(
				v.Namespace, v.ResourceType, v.ResourceSubtype, v.Name,
			),
		)
	}
	return names
}

// Logger returns the logger being used for this robot.
func (rc *RobotClient) Logger() golog.Logger {
	return rc.logger
}
