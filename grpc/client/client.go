// Package client contains a gRPC based robot.Robot client.
package client

import (
	"context"
	"runtime/debug"
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
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// errUnimplemented is used for any unimplemented methods that should
// eventually be implemented server side or faked client side.
var errUnimplemented = errors.New("unimplemented")

// RobotClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type RobotClient struct {
	address string
	conn    rpc.ClientConn
	client  pb.RobotServiceClient

	namesMu       *sync.RWMutex
	resourceNames []resource.Name

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

	conn, err := grpc.Dial(ctx, address, logger, rOpts.dialOptions...)
	if err != nil {
		return nil, err
	}

	client := pb.NewRobotServiceClient(conn)
	closeCtx, cancel := context.WithCancel(context.Background())

	rc := &RobotClient{
		address:                 address,
		conn:                    conn,
		client:                  client,
		cancelBackgroundWorkers: cancel,
		namesMu:                 &sync.RWMutex{},
		activeBackgroundWorkers: &sync.WaitGroup{},
		logger:                  logger,
		closeContext:            closeCtx,
	}

	// refresh once to hydrate the robot.
	if err := rc.Refresh(ctx); err != nil {
		return nil, multierr.Combine(err, conn.Close())
	}

	if rOpts.refreshEvery != 0 {
		rc.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(func() {
			rc.RefreshEvery(closeCtx, rOpts.refreshEvery)
		}, rc.activeBackgroundWorkers.Done)
	}

	return rc, nil
}

// Close cleanly closes the underlying connections and stops the refresh goroutine
// if it is running.
func (rc *RobotClient) Close(ctx context.Context) error {
	rc.cancelBackgroundWorkers()
	rc.activeBackgroundWorkers.Wait()

	return rc.conn.Close()
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
	c := registry.ResourceSubtypeLookup(name.Subtype)
	if c == nil || c.RPCClient == nil {
		// registration doesn't exist
		return nil, errors.New("resource client registration doesn't exist")
	}
	// pass in conn
	resourceClient := c.RPCClient(rc.closeContext, rc.conn, name.Name, rc.Logger())
	return resourceClient, nil
}

func (rc *RobotClient) resources(ctx context.Context) ([]resource.Name, error) {
	resp, err := rc.client.ResourceNames(ctx, &pb.ResourceNamesRequest{})
	if err != nil {
		return nil, err
	}

	resources := make([]resource.Name, 0, len(resp.Resources))

	for _, name := range resp.Resources {
		newName := protoutils.ResourceNameFromProto(name)
		resources = append(resources, newName)
	}
	return resources, nil
}

// Refresh manually updates the underlying parts of the robot based
// on its metadata response.
func (rc *RobotClient) Refresh(ctx context.Context) (err error) {
	rc.namesMu.Lock()
	defer rc.namesMu.Unlock()

	// call metadata service.
	names, err := rc.resources(ctx)
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
	rc.namesMu.RLock()
	defer rc.namesMu.RUnlock()
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
