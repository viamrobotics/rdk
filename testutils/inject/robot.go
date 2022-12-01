// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// Robot is an injected robot.
type Robot struct {
	robot.LocalRobot
	Mu                      sync.RWMutex // Ugly, has to be manually locked if a test means to swap funcs on an in-use robot.
	DiscoverComponentsFunc  func(ctx context.Context, keys []discovery.Query) ([]discovery.Discovery, error)
	RemoteByNameFunc        func(name string) (robot.Robot, bool)
	ResourceByNameFunc      func(name resource.Name) (interface{}, error)
	RemoteNamesFunc         func() []string
	ResourceNamesFunc       func() []resource.Name
	ResourceRPCSubtypesFunc func() []resource.RPCSubtype
	ProcessManagerFunc      func() pexec.ProcessManager
	ConfigFunc              func(ctx context.Context) (*config.Config, error)
	LoggerFunc              func() golog.Logger
	CloseFunc               func(ctx context.Context) error
	StopAllFunc             func(ctx context.Context, extra map[resource.Name]map[string]interface{}) error
	RefreshFunc             func(ctx context.Context) error
	FrameSystemConfigFunc   func(ctx context.Context, additionalTransforms []*referenceframe.PoseInFrame) (framesystemparts.Parts, error)
	TransformPoseFunc       func(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*referenceframe.PoseInFrame,
	) (*referenceframe.PoseInFrame, error)
	StatusFunc func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error)

	ops     *operation.Manager
	opsLock sync.Mutex
}

// MockResourcesFromMap mocks ResourceNames and ResourceByName based on a resource map.
func (r *Robot) MockResourcesFromMap(rs map[resource.Name]interface{}) {
	r.ResourceNamesFunc = func() []resource.Name {
		result := []resource.Name{}
		for name := range rs {
			result = append(result, name)
		}
		return result
	}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		result, ok := rs[name]
		if ok {
			return result, nil
		}
		return r.ResourceByName(name)
	}
}

// RemoteByName calls the injected RemoteByName or the real version.
func (r *Robot) RemoteByName(name string) (robot.Robot, bool) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.RemoteByNameFunc == nil {
		return r.LocalRobot.RemoteByName(name)
	}
	return r.RemoteByNameFunc(name)
}

// ResourceByName calls the injected ResourceByName or the real version.
func (r *Robot) ResourceByName(name resource.Name) (interface{}, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.ResourceByNameFunc == nil {
		return r.LocalRobot.ResourceByName(name)
	}
	return r.ResourceByNameFunc(name)
}

// RemoteNames calls the injected RemoteNames or the real version.
func (r *Robot) RemoteNames() []string {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.RemoteNamesFunc == nil {
		return r.LocalRobot.RemoteNames()
	}
	return r.RemoteNamesFunc()
}

// ResourceNames calls the injected ResourceNames or the real version.
func (r *Robot) ResourceNames() []resource.Name {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.ResourceNamesFunc == nil {
		return r.LocalRobot.ResourceNames()
	}
	return r.ResourceNamesFunc()
}

// ResourceRPCSubtypes returns a list of all known resource RPC subtypes.
func (r *Robot) ResourceRPCSubtypes() []resource.RPCSubtype {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.ResourceRPCSubtypesFunc == nil {
		return r.LocalRobot.ResourceRPCSubtypes()
	}
	return r.ResourceRPCSubtypesFunc()
}

// ProcessManager calls the injected ProcessManager or the real version.
func (r *Robot) ProcessManager() pexec.ProcessManager {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.ProcessManagerFunc == nil {
		return r.LocalRobot.ProcessManager()
	}
	return r.ProcessManagerFunc()
}

// OperationManager calls the injected OperationManager or the real version.
func (r *Robot) OperationManager() *operation.Manager {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	r.opsLock.Lock()
	defer r.opsLock.Unlock()

	if r.ops == nil {
		r.ops = operation.NewManager(r.Logger())
	}
	return r.ops
}

// Config calls the injected Config or the real version.
func (r *Robot) Config(ctx context.Context) (*config.Config, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.ConfigFunc == nil {
		return r.LocalRobot.Config(ctx)
	}
	return r.ConfigFunc(ctx)
}

// Logger calls the injected Logger or the real version.
func (r *Robot) Logger() golog.Logger {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.LoggerFunc == nil {
		return r.LocalRobot.Logger()
	}
	return r.LoggerFunc()
}

// Close calls the injected Close or the real version.
func (r *Robot) Close(ctx context.Context) error {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.CloseFunc == nil {
		return utils.TryClose(ctx, r.LocalRobot)
	}
	return r.CloseFunc(ctx)
}

// StopAll calls the injected StopAll or the real version.
func (r *Robot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.StopAllFunc == nil {
		return r.LocalRobot.StopAll(ctx, extra)
	}
	return r.StopAllFunc(ctx, extra)
}

// Refresh calls the injected Refresh or the real version.
func (r *Robot) Refresh(ctx context.Context) error {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.RefreshFunc == nil {
		if refresher, ok := r.LocalRobot.(robot.Refresher); ok {
			return refresher.Refresh(ctx)
		}
		return nil
	}
	return r.RefreshFunc(ctx)
}

// DiscoverComponents call the injected DiscoverComponents or the real one.
func (r *Robot) DiscoverComponents(ctx context.Context, keys []discovery.Query) ([]discovery.Discovery, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.DiscoverComponentsFunc == nil {
		return r.LocalRobot.DiscoverComponents(ctx, keys)
	}
	return r.DiscoverComponentsFunc(ctx, keys)
}

// FrameSystemConfig calls the injected FrameSystemConfig or the real version.
func (r *Robot) FrameSystemConfig(ctx context.Context, additionalTransforms []*referenceframe.PoseInFrame) (framesystemparts.Parts, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.FrameSystemConfigFunc == nil {
		return r.LocalRobot.FrameSystemConfig(ctx, additionalTransforms)
	}

	return r.FrameSystemConfigFunc(ctx, additionalTransforms)
}

// TransformPose calls the injected TransformPose or the real version.
func (r *Robot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.PoseInFrame,
) (*referenceframe.PoseInFrame, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.TransformPoseFunc == nil {
		return r.LocalRobot.TransformPose(ctx, pose, dst, additionalTransforms)
	}
	return r.TransformPoseFunc(ctx, pose, dst, additionalTransforms)
}

// Status call the injected Status or the real one.
func (r *Robot) Status(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	if r.StatusFunc == nil {
		return r.LocalRobot.Status(ctx, resourceNames)
	}
	return r.StatusFunc(ctx, resourceNames)
}
