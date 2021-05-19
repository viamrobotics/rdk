package robotimpl

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rexec"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
)

var errUnimplemented = errors.New("unimplemented")

// A remoteRobot implements wraps an robot.Robot. It
// assists in the un/prefixing of part names for RemoteRobots that
// are not aware they are integrated elsewhere.
// We intentionally do not promote the underlying robot.Robot
// so that any future changes are forced to consider un/prefixing
// of names.
type remoteRobot struct {
	mu    sync.Mutex
	robot robot.Robot
	conf  config.Remote
	parts *robotParts
}

// newRemoteRobot returns a new remote robot wrapping a given robot.Robot
// and its configuration.
func newRemoteRobot(robot robot.Robot, config config.Remote) *remoteRobot {
	// We pull the parts out here such that we correctly return nil for
	// when parts are accessed. This is because a networked robot client
	// may just return a non-nil wrapper for a part they may not exist.
	remoteParts := partsForRemoteRobot(robot)
	return &remoteRobot{
		robot: robot,
		conf:  config,
		parts: remoteParts,
	}
}

func (rr *remoteRobot) Refresh(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if err := rr.robot.Refresh(ctx); err != nil {
		return err
	}
	rr.parts = partsForRemoteRobot(rr.robot)
	return nil
}

// Reconfigure replaces this robot with the given robot.
func (rr *remoteRobot) Reconfigure(newRobot robot.Robot, diff *config.Diff) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	actual, ok := newRobot.(*remoteRobot)
	if !ok {
		panic(fmt.Errorf("expected new servo to be %T but got %T", actual, newRobot))
	}

	if err := rr.parts.Reconfigure(actual.parts, diff); err != nil {
		rlog.Logger.Errorw("error during reconfiguration but proceeding", "error", err)
	}
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

func (rr *remoteRobot) prefixNames(names []string) []string {
	if !rr.conf.Prefix {
		return names
	}
	newNames := make([]string, 0, len(names))
	for _, name := range names {
		newNames = append(newNames, rr.prefixName(name))
	}
	return newNames
}

// AddProvider should not be used or needed for a remote robot
func (rr *remoteRobot) AddProvider(p robot.Provider, c config.Component) {
}

// ProviderByName should not be used or needed for a remote robot
func (rr *remoteRobot) ProviderByName(name string) robot.Provider {
	return nil
}

func (rr *remoteRobot) RemoteNames() []string {
	return nil
}

func (rr *remoteRobot) ArmNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.ArmNames())
}

func (rr *remoteRobot) GripperNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.GripperNames())
}

func (rr *remoteRobot) CameraNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.CameraNames())
}

func (rr *remoteRobot) LidarNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.LidarNames())
}

func (rr *remoteRobot) BaseNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.BaseNames())
}

func (rr *remoteRobot) BoardNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.BoardNames())
}

func (rr *remoteRobot) SensorNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.SensorNames())
}

func (rr *remoteRobot) RemoteByName(name string) robot.Robot {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rr *remoteRobot) ArmByName(name string) arm.Arm {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.ArmByName(rr.unprefixName(name))
}

func (rr *remoteRobot) BaseByName(name string) base.Base {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.BaseByName(rr.unprefixName(name))
}

func (rr *remoteRobot) GripperByName(name string) gripper.Gripper {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.GripperByName(rr.unprefixName(name))
}

func (rr *remoteRobot) CameraByName(name string) camera.Camera {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.CameraByName(rr.unprefixName(name))
}

func (rr *remoteRobot) LidarByName(name string) lidar.Lidar {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.LidarByName(rr.unprefixName(name))
}

func (rr *remoteRobot) BoardByName(name string) board.Board {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.BoardByName(rr.unprefixName(name))
}

func (rr *remoteRobot) SensorByName(name string) sensor.Sensor {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.parts.SensorByName(rr.unprefixName(name))
}

func (rr *remoteRobot) ProcessManager() rexec.ProcessManager {
	return rexec.NoopProcessManager
}

func (rr *remoteRobot) Config(ctx context.Context) (*config.Config, error) {
	return rr.robot.Config(ctx)
}

func (rr *remoteRobot) Status(ctx context.Context) (*pb.Status, error) {
	status, err := rr.robot.Status(ctx)
	if err != nil {
		return nil, err
	}
	var rewrittenStatus pb.Status

	if len(status.Arms) != 0 {
		rewrittenStatus.Arms = make(map[string]*pb.ArmStatus, len(status.Arms))
		for k, v := range status.Arms {
			rewrittenStatus.Arms[rr.prefixName(k)] = v
		}
	}
	if len(status.Bases) != 0 {
		rewrittenStatus.Bases = make(map[string]bool, len(status.Bases))
		for k, v := range status.Bases {
			rewrittenStatus.Bases[rr.prefixName(k)] = v
		}
	}
	if len(status.Grippers) != 0 {
		rewrittenStatus.Grippers = make(map[string]bool, len(status.Grippers))
		for k, v := range status.Grippers {
			rewrittenStatus.Grippers[rr.prefixName(k)] = v
		}
	}
	if len(status.Boards) != 0 {
		rewrittenStatus.Boards = make(map[string]*pb.BoardStatus, len(status.Boards))
		for k, v := range status.Boards {
			rewrittenStatus.Boards[rr.prefixName(k)] = v
		}
	}
	if len(status.Cameras) != 0 {
		rewrittenStatus.Cameras = make(map[string]bool, len(status.Cameras))
		for k, v := range status.Cameras {
			rewrittenStatus.Cameras[rr.prefixName(k)] = v
		}
	}
	if len(status.Lidars) != 0 {
		rewrittenStatus.Lidars = make(map[string]bool, len(status.Lidars))
		for k, v := range status.Lidars {
			rewrittenStatus.Lidars[rr.prefixName(k)] = v
		}
	}
	if len(status.Sensors) != 0 {
		rewrittenStatus.Sensors = make(map[string]*pb.SensorStatus, len(status.Sensors))
		for k, v := range status.Sensors {
			rewrittenStatus.Sensors[rr.prefixName(k)] = v
		}
	}

	return &rewrittenStatus, nil
}

func (rr *remoteRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *remoteRobot) Close() error {
	return utils.TryClose(rr.robot)
}
