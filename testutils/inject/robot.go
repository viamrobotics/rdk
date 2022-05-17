// Package inject provides dependency injected structures for mocking interfaces.
package inject

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// Robot is an injected robot.
type Robot struct {
	robot.LocalRobot
	RemoteByNameFunc      func(name string) (robot.Robot, bool)
	ResourceByNameFunc    func(name resource.Name) (interface{}, error)
	RemoteNamesFunc       func() []string
	ResourceNamesFunc     func() []resource.Name
	ProcessManagerFunc    func() pexec.ProcessManager
	ConfigFunc            func(ctx context.Context) (*config.Config, error)
	LoggerFunc            func() golog.Logger
	CloseFunc             func(ctx context.Context) error
	RefreshFunc           func(ctx context.Context) error
	FrameSystemConfigFunc func(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error)
	TransformPoseFunc     func(
		ctx context.Context,
		pose *referenceframe.PoseInFrame,
		dst string,
		additionalTransforms []*commonpb.Transform,
	) (*referenceframe.PoseInFrame, error)

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
	if r.RemoteByNameFunc == nil {
		return r.LocalRobot.RemoteByName(name)
	}
	return r.RemoteByNameFunc(name)
}

// ResourceByName calls the injected ResourceByName or the real version.
func (r *Robot) ResourceByName(name resource.Name) (interface{}, error) {
	if r.ResourceByNameFunc == nil {
		return r.LocalRobot.ResourceByName(name)
	}
	return r.ResourceByNameFunc(name)
}

// RemoteNames calls the injected RemoteNames or the real version.
func (r *Robot) RemoteNames() []string {
	if r.RemoteNamesFunc == nil {
		return r.LocalRobot.RemoteNames()
	}
	return r.RemoteNamesFunc()
}

// ResourceNames calls the injected ResourceNames or the real version.
func (r *Robot) ResourceNames() []resource.Name {
	if r.ResourceNamesFunc == nil {
		return r.LocalRobot.ResourceNames()
	}
	return r.ResourceNamesFunc()
}

// ProcessManager calls the injected ProcessManager or the real version.
func (r *Robot) ProcessManager() pexec.ProcessManager {
	if r.ProcessManagerFunc == nil {
		return r.LocalRobot.ProcessManager()
	}
	return r.ProcessManagerFunc()
}

// OperationManager calls the injected OperationManager or the real version.
func (r *Robot) OperationManager() *operation.Manager {
	r.opsLock.Lock()
	defer r.opsLock.Unlock()

	if r.ops == nil {
		r.ops = operation.NewManager()
	}
	return r.ops
}

// Config calls the injected Config or the real version.
func (r *Robot) Config(ctx context.Context) (*config.Config, error) {
	if r.ConfigFunc == nil {
		return r.LocalRobot.Config(ctx)
	}
	return r.ConfigFunc(ctx)
}

// Logger calls the injected Logger or the real version.
func (r *Robot) Logger() golog.Logger {
	if r.LoggerFunc == nil {
		return r.LocalRobot.Logger()
	}
	return r.LoggerFunc()
}

// Close calls the injected Close or the real version.
func (r *Robot) Close(ctx context.Context) error {
	if r.CloseFunc == nil {
		return utils.TryClose(ctx, r.LocalRobot)
	}
	return r.CloseFunc(ctx)
}

// Refresh calls the injected Refresh or the real version.
func (r *Robot) Refresh(ctx context.Context) error {
	if r.RefreshFunc == nil {
		if refresher, ok := r.LocalRobot.(robot.Refresher); ok {
			return refresher.Refresh(ctx)
		}
		return nil
	}
	return r.RefreshFunc(ctx)
}

// RemoteRobot is an injected remote robot.
type RemoteRobot struct {
	Robot

	Disconnected bool
	ChangeChan   chan bool
}

// Changed returns a channel that returns true when the remote has changed.
func (r *RemoteRobot) Changed() <-chan bool {
	if r.ChangeChan == nil {
		r.ChangeChan = make(chan bool)
	}
	return r.ChangeChan
}

// Connected returns whether the injected robot is connected or not.
func (r *RemoteRobot) Connected() bool {
	return !r.Disconnected
}

func (r *Robot) FrameSystemConfig(ctx context.Context, additionalTransforms []*commonpb.Transform) (framesystemparts.Parts, error) {
	if r.FrameSystemConfigFunc == nil {
		return r.LocalRobot.FrameSystemConfig(ctx, additionalTransforms)
	}

	return r.FrameSystemConfigFunc(ctx, additionalTransforms)
}

func (r *Robot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	if r.TransformPoseFunc == nil {
		return r.LocalRobot.TransformPose(ctx, pose, dst, additionalTransforms)
	}
	return r.TransformPoseFunc(ctx, pose, dst, additionalTransforms)
}
