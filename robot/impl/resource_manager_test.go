//go:build !no_media

package robotimpl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/google/go-cmp/cmp"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.mongodb.org/mongo-driver/bson/primitive"
	armpb "go.viam.com/api/component/arm/v1"
	basepb "go.viam.com/api/component/base/v1"
	boardpb "go.viam.com/api/component/board/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	gripperpb "go.viam.com/api/component/gripper/v1"
	motionpb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/protobuf/testing/protocmp"

	"go.viam.com/rdk/components/arm"
	fakearm "go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/camera"
	fakecamera "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/components/gripper"
	fakegripper "go.viam.com/rdk/components/gripper/fake"
	"go.viam.com/rdk/components/input"
	fakeinput "go.viam.com/rdk/components/input/fake"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	fakeservo "go.viam.com/rdk/components/servo/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/session"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
)

func setupInjectRobot(logger golog.Logger) *inject.Robot {
	injectRobot := &inject.Robot{}
	armNames := []resource.Name{
		arm.Named("arm1"),
		arm.Named("arm2"),
	}
	baseNames := []resource.Name{
		base.Named("base1"),
		base.Named("base2"),
	}
	boardNames := []resource.Name{
		board.Named("board1"),
		board.Named("board2"),
	}
	cameraNames := []resource.Name{
		camera.Named("camera1"),
		camera.Named("camera2"),
	}
	gripperNames := []resource.Name{
		gripper.Named("gripper1"),
		gripper.Named("gripper2"),
	}
	inputNames := []resource.Name{
		input.Named("inputController1"),
		input.Named("inputController2"),
	}
	motorNames := []resource.Name{
		motor.Named("motor1"),
		motor.Named("motor2"),
	}
	servoNames := []resource.Name{
		servo.Named("servo1"),
		servo.Named("servo2"),
	}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{"remote1%s", "remote2"}
	}

	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)
	}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		if _, ok := utils.NewStringSet(injectRobot.RemoteNames()...)[name]; !ok {
			return nil, false
		}
		return &dummyRobot{}, true
	}

	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.API {
				case arm.API:
					return &fakearm.Arm{Named: name.AsNamed()}, nil
				case base.API:
					return &fakebase.Base{Named: name.AsNamed()}, nil
				case board.API:
					fakeBoard, err := fakeboard.NewBoard(context.Background(), resource.Config{
						Name: name.String(),
						ConvertedAttributes: &fakeboard.Config{
							AnalogReaders: []board.AnalogReaderConfig{
								{Name: "analog1"},
								{Name: "analog2"},
							},
							DigitalInterrupts: []board.DigitalInterruptConfig{
								{Name: "digital1"},
								{Name: "digital2"},
							},
						},
					}, logger)
					if err != nil {
						panic(err)
					}
					return fakeBoard, nil
				case camera.API:
					conf := resource.NewEmptyConfig(name, resource.DefaultModelFamily.WithModel("fake"))
					conf.ConvertedAttributes = &fakecamera.Config{}
					return fakecamera.NewCamera(context.Background(), conf, logger)
				case gripper.API:
					return &fakegripper.Gripper{Named: name.AsNamed()}, nil
				case input.API:
					return &fakeinput.InputController{Named: name.AsNamed()}, nil
				case motor.API:
					return &fakemotor.Motor{Named: name.AsNamed()}, nil
				case servo.API:
					return &fakeservo.Servo{Named: name.AsNamed()}, nil
				}
				if rName.API.IsService() {
					return rdktestutils.NewUnimplementedResource(name), nil
				}
			}
		}
		return nil, resource.NewNotFoundError(name)
	}

	return injectRobot
}

func TestManagerForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}

	test.That(t, manager.RemoteNames(), test.ShouldBeEmpty)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(manager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("base1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("board1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(camera.Named("camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("camera1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(gripper.Named("gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("gripper1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("motor1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(servo.Named("servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("servo_what"))
	test.That(t, err, test.ShouldBeError)
}

func TestManagerMergeNamesWithRemotes(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote2"},
	)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddRemotes(armNames, "remote1", "remote2")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddRemotes(baseNames, "remote1", "remote2")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddRemotes(boardNames, "remote1", "remote2")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddRemotes(cameraNames, "remote1", "remote2")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddRemotes(gripperNames, "remote1", "remote2")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddRemotes(inputNames, "remote1", "remote2")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddRemotes(motorNames, "remote1", "remote2")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddRemotes(servoNames, "remote1", "remote2")...)

	test.That(
		t,
		utils.NewStringSet(manager.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(manager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1:arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote2:arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("what:arm1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("remote1:base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("remote2:base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("what:base1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("remote1:board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("remote2:board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("what:board1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(camera.Named("camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("remote1:camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("remote2:camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("what:camera1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(gripper.Named("gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("remote1:gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("remote2:gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("what:gripper1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("remote1:motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("remote2:motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("what:motor1"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(servo.Named("servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("remote1:servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("remote2:servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("what:servo1"))
	test.That(t, err, test.ShouldBeError)
}

func TestManagerResourceRemoteName(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	injectRobot.ResourceNamesFunc = func() []resource.Name { return armNames }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return rdktestutils.NewUnimplementedResource(name), nil
	}
	injectRobot.LoggerFunc = func() golog.Logger { return logger }

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()

	injectRemote := &inject.Robot{}
	injectRemote.ResourceNamesFunc = func() []resource.Name { return rdktestutils.AddSuffixes(armNames, "") }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRemote.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return rdktestutils.NewUnimplementedResource(name), nil
	}
	injectRemote.LoggerFunc = func() golog.Logger { return logger }
	manager.addRemote(
		context.Background(),
		newDummyRobot(injectRemote),
		nil,
		config.Remote{Name: "remote1"},
	)

	manager.updateRemotesResourceNames(context.Background())

	res := manager.remoteResourceNames(fromRemoteNameToRemoteNodeName("remote1"))

	test.That(
		t,
		rdktestutils.NewResourceNameSet(res...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet([]resource.Name{arm.Named("remote1:arm1"), arm.Named("remote1:arm2")}...),
	)
}

func TestManagerWithSameNameInRemoteNoPrefix(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote2"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1:arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerWithSameNameInBaseAndRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1:arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerAdd(t *testing.T) {
	logger := golog.NewTestLogger(t)
	manager := newResourceManager(resourceManagerOptions{}, logger)

	injectArm := &inject.Arm{}
	cfg := &resource.Config{API: arm.API, Name: "arm1"}
	rName := cfg.ResourceName()
	manager.resources.AddNode(rName, resource.NewConfiguredGraphNode(*cfg, injectArm, cfg.Model))
	arm1, err := manager.ResourceByName(rName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1, test.ShouldEqual, injectArm)

	injectBoard := &inject.Board{}
	injectBoard.SPINamesFunc = func() []string {
		return []string{"spi1"}
	}
	injectBoard.I2CNamesFunc = func() []string {
		return []string{"i2c1"}
	}
	injectBoard.AnalogReaderNamesFunc = func() []string {
		return []string{"analog1"}
	}
	injectBoard.DigitalInterruptNamesFunc = func() []string {
		return []string{"digital1"}
	}
	injectBoard.SPIByNameFunc = func(name string) (board.SPI, bool) {
		return &inject.SPI{}, true
	}
	injectBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		return &inject.I2C{}, true
	}
	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return &fakeboard.AnalogReader{}, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return &board.BasicDigitalInterrupt{}, true
	}

	cfg = &resource.Config{
		API:  board.API,
		Name: "board1",
	}
	rName = cfg.ResourceName()
	manager.resources.AddNode(rName, resource.NewConfiguredGraphNode(*cfg, injectBoard, cfg.Model))
	board1, err := manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldEqual, injectBoard)
	resource1, err := manager.ResourceByName(rName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resource1, test.ShouldEqual, injectBoard)

	injectMotionService := &inject.MotionService{}
	injectMotionService.MoveFunc = func(
		ctx context.Context,
		componentName resource.Name,
		grabPose *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *motionpb.Constraints,
		extra map[string]interface{},
	) (bool, error) {
		return false, nil
	}
	objectMResName := motion.Named("motion1")
	manager.resources.AddNode(objectMResName, resource.NewConfiguredGraphNode(resource.Config{}, injectMotionService, unknownModel))
	motionService, err := manager.ResourceByName(objectMResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motionService, test.ShouldEqual, injectMotionService)

	injectVisionService := &inject.VisionService{}
	injectVisionService.GetObjectPointCloudsFunc = func(
		ctx context.Context,
		cameraName string,
		extra map[string]interface{},
	) ([]*viz.Object, error) {
		return []*viz.Object{viz.NewEmptyObject()}, nil
	}
	objectSegResName := vision.Named(resource.DefaultServiceName)
	manager.resources.AddNode(objectSegResName, resource.NewConfiguredGraphNode(resource.Config{}, injectVisionService, unknownModel))
	objectSegmentationService, err := manager.ResourceByName(objectSegResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objectSegmentationService, test.ShouldEqual, injectVisionService)
}

func TestManagerNewComponent(t *testing.T) {
	fakeModel := resource.DefaultModelFamily.WithModel("fake")
	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:      "arm1",
				Model:     fakeModel,
				API:       arm.API,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "arm2",
				Model:     fakeModel,
				API:       arm.API,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "arm3",
				Model:     fakeModel,
				API:       arm.API,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "base1",
				Model:     fakeModel,
				API:       base.API,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "base2",
				Model:     fakeModel,
				API:       base.API,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "base3",
				Model:     fakeModel,
				API:       base.API,
				DependsOn: []string{"board3"},
			},
			{
				Name:                "board1",
				Model:               fakeModel,
				API:                 board.API,
				ConvertedAttributes: &fakeboard.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board2",
				Model:               fakeModel,
				API:                 board.API,
				ConvertedAttributes: &fakeboard.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board3",
				Model:               fakeModel,
				API:                 board.API,
				ConvertedAttributes: &fakeboard.Config{},
				DependsOn:           []string{},
			},
			{
				Name:      "camera1",
				Model:     fakeModel,
				API:       camera.API,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "camera2",
				Model:     fakeModel,
				API:       camera.API,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "camera3",
				Model:     fakeModel,
				API:       camera.API,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "gripper1",
				Model:     fakeModel,
				API:       gripper.API,
				DependsOn: []string{"arm1", "camera1"},
			},
			{
				Name:      "gripper2",
				Model:     fakeModel,
				API:       gripper.API,
				DependsOn: []string{"arm2", "camera2"},
			},
			{
				Name:      "gripper3",
				Model:     fakeModel,
				API:       gripper.API,
				DependsOn: []string{"arm3", "camera3"},
			},
			{
				Name:                "inputController1",
				Model:               fakeModel,
				API:                 input.API,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "inputController2",
				Model:               fakeModel,
				API:                 input.API,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "inputController3",
				Model:               fakeModel,
				API:                 input.API,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:                "motor1",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "motor2",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "motor3",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:      "sensor1",
				Model:     fakeModel,
				API:       sensor.API,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "sensor2",
				Model:     fakeModel,
				API:       sensor.API,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "sensor3",
				Model:     fakeModel,
				API:       sensor.API,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "servo1",
				Model:     fakeModel,
				API:       servo.API,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "servo2",
				Model:     fakeModel,
				API:       servo.API,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "servo3",
				Model:     fakeModel,
				API:       servo.API,
				DependsOn: []string{"board3"},
			},
		},
	}
	logger := golog.NewTestLogger(t)
	robotForRemote := &localRobot{
		manager: newResourceManager(resourceManagerOptions{}, logger),
	}
	diff, err := config.DiffConfigs(config.Config{}, *cfg, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robotForRemote.manager.updateResources(context.Background(), diff), test.ShouldBeNil)
	test.That(t, robotForRemote.manager.resources.ResolveDependencies(logger), test.ShouldBeNil)

	diff = &config.Diff{
		Added: &config.Config{},
		Modified: &config.ModifiedConfigDiff{
			Components: []resource.Config{},
		},
	}

	diff.Modified.Components = append(diff.Modified.Components, resource.Config{
		Name:                "board3",
		Model:               fakeModel,
		API:                 board.API,
		ConvertedAttributes: &fakeboard.Config{},
		DependsOn:           []string{"arm3"},
	})
	test.That(t, robotForRemote.manager.updateResources(context.Background(), diff), test.ShouldBeNil)
	err = robotForRemote.manager.resources.ResolveDependencies(logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "circular dependency")
	test.That(t, err.Error(), test.ShouldContainSubstring, "arm3")
	test.That(t, err.Error(), test.ShouldContainSubstring, "board3")
}

func managerForTest(ctx context.Context, t *testing.T, l golog.Logger) *resourceManager {
	t.Helper()
	injectRobot := setupInjectRobot(l)
	manager := managerForDummyRobot(injectRobot)

	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(l)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(setupInjectRobot(l)),
		nil,
		config.Remote{Name: "remote2"},
	)
	_, err := manager.processManager.AddProcess(ctx, &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.processManager.AddProcess(ctx, &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)
	return manager
}

func TestManagerMarkRemoved(t *testing.T) {
	logger := golog.NewTestLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	manager := managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	checkEmpty := func(
		procMan pexec.ProcessManager,
		resourcesToCloseBeforeComplete []resource.Resource,
		names map[resource.Name]struct{},
	) {
		t.Helper()
		test.That(t, names, test.ShouldBeEmpty)
		test.That(t, resourcesToCloseBeforeComplete, test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(procMan.ProcessIDs()...), test.ShouldBeEmpty)
	}

	processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames := manager.markRemoved(ctx, &config.Config{}, logger)
	checkEmpty(processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames)

	processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames = manager.markRemoved(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "what",
			},
		},
		Components: []resource.Config{
			{
				Name: "what1",
				API:  arm.API,
			},
			{
				Name: "what5",
				API:  base.API,
			},
			{
				Name: "what3",
				API:  board.API,
			},
			{
				Name: "what4",
				API:  camera.API,
			},
			{
				Name: "what5",
				API:  gripper.API,
			},
			{
				Name: "what6",
				API:  motor.API,
			},
			{
				Name: "what7",
				API:  sensor.API,
			},
			{
				Name: "what8",
				API:  servo.API,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "what",
				Name: "echo",
			},
		},
	}, logger)
	checkEmpty(processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames)

	processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames = manager.markRemoved(ctx, &config.Config{
		Components: []resource.Config{
			{
				Name: "what1",
			},
		},
	}, logger)
	checkEmpty(processesToRemove, resourcesToCloseBeforeComplete, markedResourceNames)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	processesToRemove, _, markedResourceNames = manager.markRemoved(ctx, &config.Config{
		Components: []resource.Config{
			{
				Name: "arm2",
				API:  arm.API,
			},
			{
				Name: "base2",
				API:  base.API,
			},
			{
				Name: "board2",
				API:  board.API,
			},
			{
				Name: "camera2",
				API:  camera.API,
			},
			{
				Name: "gripper2",
				API:  gripper.API,
			},
			{
				Name: "inputController2",
				API:  input.API,
			},
			{
				Name: "motor2",
				API:  motor.API,
			},
			{
				Name: "sensor2",
				API:  sensor.API,
			},

			{
				Name: "servo2",
				API:  servo.API,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)

	armNames := []resource.Name{arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base2")}
	boardNames := []resource.Name{board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo2")}

	test.That(
		t,
		markedResourceNames,
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(processesToRemove.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	processesToRemove, _, markedResourceNames = manager.markRemoved(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote2",
			},
		},
		Components: []resource.Config{
			{
				Name: "arm2",
				API:  arm.API,
			},
			{
				Name: "base2",
				API:  base.API,
			},
			{
				Name: "board2",
				API:  board.API,
			},
			{
				Name: "camera2",
				API:  camera.API,
			},
			{
				Name: "gripper2",
				API:  gripper.API,
			},
			{
				Name: "inputController2",
				API:  input.API,
			},
			{
				Name: "motor2",
				API:  motor.API,
			},
			{
				Name: "sensor2",
				API:  sensor.API,
			},
			{
				Name: "servo2",
				API:  servo.API,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("remote2:arm1"), arm.Named("remote2:arm2")}
	baseNames = []resource.Name{
		base.Named("base2"),
		base.Named("remote2:base1"),
		base.Named("remote2:base2"),
	}
	boardNames = []resource.Name{
		board.Named("board2"),
		board.Named("remote2:board1"),
		board.Named("remote2:board2"),
	}
	cameraNames = []resource.Name{
		camera.Named("camera2"),
		camera.Named("remote2:camera1"),
		camera.Named("remote2:camera2"),
	}
	gripperNames = []resource.Name{
		gripper.Named("gripper2"),
		gripper.Named("remote2:gripper1"),
		gripper.Named("remote2:gripper2"),
	}
	inputNames = []resource.Name{
		input.Named("inputController2"),
		input.Named("remote2:inputController1"),
		input.Named("remote2:inputController2"),
	}
	motorNames = []resource.Name{
		motor.Named("motor2"),
		motor.Named("remote2:motor1"),
		motor.Named("remote2:motor2"),
	}
	servoNames = []resource.Name{
		servo.Named("servo2"),
		servo.Named("remote2:servo1"),
		servo.Named("remote2:servo2"),
	}

	test.That(
		t,
		markedResourceNames,
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
			[]resource.Name{fromRemoteNameToRemoteNodeName("remote2")},
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(processesToRemove.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	processesToRemove, _, markedResourceNames = manager.markRemoved(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote1",
			},
			{
				Name: "remote2",
			},
			{
				Name: "remote3",
			},
		},
		Components: []resource.Config{
			{
				Name: "arm1",
				API:  arm.API,
			},
			{
				Name: "arm2",
				API:  arm.API,
			},
			{
				Name: "arm3",
				API:  arm.API,
			},
			{
				Name: "base1",
				API:  base.API,
			},
			{
				Name: "base2",
				API:  base.API,
			},
			{
				Name: "base3",
				API:  base.API,
			},
			{
				Name: "board1",
				API:  board.API,
			},
			{
				Name: "board2",
				API:  board.API,
			},
			{
				Name: "board3",
				API:  board.API,
			},
			{
				Name: "camera1",
				API:  camera.API,
			},
			{
				Name: "camera2",
				API:  camera.API,
			},
			{
				Name: "camera3",
				API:  camera.API,
			},
			{
				Name: "gripper1",
				API:  gripper.API,
			},
			{
				Name: "gripper2",
				API:  gripper.API,
			},
			{
				Name: "gripper3",
				API:  gripper.API,
			},
			{
				Name: "inputController1",
				API:  input.API,
			},
			{
				Name: "inputController2",
				API:  input.API,
			},
			{
				Name: "inputController3",
				API:  input.API,
			},
			{
				Name: "motor1",
				API:  motor.API,
			},
			{
				Name: "motor2",
				API:  motor.API,
			},
			{
				Name: "motor3",
				API:  motor.API,
			},
			{
				Name: "sensor1",
				API:  sensor.API,
			},
			{
				Name: "sensor2",
				API:  sensor.API,
			},
			{
				Name: "sensor3",
				API:  sensor.API,
			},
			{
				Name: "servo1",
				API:  servo.API,
			},
			{
				Name: "servo2",
				API:  servo.API,
			},
			{
				Name: "servo3",
				API:  servo.API,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo", // does not matter
			},
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
			{
				ID:   "3",
				Name: "echo", // does not matter
			},
		},
	}, logger)

	armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddRemotes(armNames, "remote1", "remote2")...)
	baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddRemotes(baseNames, "remote1", "remote2")...)
	boardNames = []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddRemotes(boardNames, "remote1", "remote2")...)
	cameraNames = []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddRemotes(cameraNames, "remote1", "remote2")...)
	gripperNames = []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddRemotes(gripperNames, "remote1", "remote2")...)
	inputNames = []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddRemotes(inputNames, "remote1", "remote2")...)
	motorNames = []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddRemotes(motorNames, "remote1", "remote2")...)
	servoNames = []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddRemotes(servoNames, "remote1", "remote2")...)

	test.That(
		t,
		markedResourceNames,
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
			[]resource.Name{
				fromRemoteNameToRemoteNodeName("remote1"),
				fromRemoteNameToRemoteNodeName("remote2"),
			},
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(processesToRemove.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)
	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()
}

func TestConfigRemoteAllowInsecureCreds(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	altName := primitive.NewObjectID().Hex()
	cert, certFile, keyFile, certPool, err := testutils.GenerateSelfSignedCertificate("somename", altName)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		os.Remove(certFile)
		os.Remove(keyFile)
	})

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	options.Network.TLSConfig = &tls.Config{
		RootCAs:      certPool,
		ClientCAs:    certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	}
	options.Auth.TLSAuthEntities = leaf.DNSNames
	options.Managed = true
	options.FQDN = altName
	locationSecret := "locsosecret"

	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rutils.CredentialsTypeRobotLocationSecret,
			Config: rutils.AttributeMap{
				"secret": locationSecret,
			},
		},
	}

	options.BakedAuthEntity = "blah"
	options.BakedAuthCreds = rpc.Credentials{Type: "blah"}

	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remoteTLSConfig := options.Network.TLSConfig.Clone()
	remoteTLSConfig.Certificates = nil
	remoteTLSConfig.ServerName = "somename"
	remote := config.Remote{
		Name:    "foo",
		Address: addr,
		Auth: config.RemoteAuth{
			Managed: true,
		},
	}
	manager := newResourceManager(resourceManagerOptions{
		tlsConfig: remoteTLSConfig,
	}, logger)

	_, err = manager.processRemote(context.Background(), remote)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remote.Auth.Entity = "wrong"
	_, err = manager.processRemote(context.Background(), remote)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remote.Auth.Entity = options.FQDN
	_, err = manager.processRemote(context.Background(), remote)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")
}

func TestConfigUntrustedEnv(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	manager := newResourceManager(resourceManagerOptions{
		untrustedEnv: true,
	}, logger)
	test.That(t, manager.processManager, test.ShouldEqual, pexec.NoopProcessManager)

	t.Run("disable processes", func(t *testing.T) {
		err := manager.updateResources(ctx, &config.Diff{
			Added: &config.Config{
				Processes: []pexec.ProcessConfig{{ID: "id1", Name: "echo"}},
			},
			Modified: &config.ModifiedConfigDiff{
				Processes: []pexec.ProcessConfig{{ID: "id2", Name: "echo"}},
			},
		})
		test.That(t, errors.Is(err, errProcessesDisabled), test.ShouldBeTrue)

		processesToClose, _, _ := manager.markRemoved(ctx, &config.Config{
			Processes: []pexec.ProcessConfig{{ID: "id1", Name: "echo"}},
		}, logger)
		test.That(t, processesToClose.ProcessIDs(), test.ShouldBeEmpty)
	})

	t.Run("disable shell service", func(t *testing.T) {
		err := manager.updateResources(ctx, &config.Diff{
			Added: &config.Config{
				Services: []resource.Config{{
					Name: "shell-service",
					API:  shell.API,
				}},
			},
			Modified: &config.ModifiedConfigDiff{
				Services: []resource.Config{{
					Name: "shell-service",
					API:  shell.API,
				}},
			},
		})
		test.That(t, errors.Is(err, errShellServiceDisabled), test.ShouldBeTrue)

		_, resourcesToCloseBeforeComplete, markedResourceNames := manager.markRemoved(ctx, &config.Config{
			Services: []resource.Config{{
				Name: "shell-service",
				API:  shell.API,
			}},
		}, logger)
		test.That(t, resourcesToCloseBeforeComplete, test.ShouldBeEmpty)
		test.That(t, markedResourceNames, test.ShouldBeEmpty)
	})
}

type fakeProcess struct {
	id string
}

func (fp *fakeProcess) ID() string {
	return fp.id
}

func (fp *fakeProcess) Start(ctx context.Context) error {
	return nil
}

func (fp *fakeProcess) Stop() error {
	return nil
}

func TestManagerResourceRPCAPIs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{
			arm.Named("arm1"),
			arm.Named("arm2"),
			base.Named("base1"),
			base.Named("base2"),
		}
	}
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.API {
				case arm.API:
					return &fakearm.Arm{Named: name.AsNamed()}, nil
				case base.API:
					return &fakebase.Base{Named: name.AsNamed()}, nil
				}
			}
		}
		return nil, resource.NewNotFoundError(name)
	}

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()

	api1 := resource.APINamespace("acme").WithComponentType("huwat")
	api2 := resource.APINamespace("acme").WithComponentType("wat")

	resName1 := resource.NewName(api1, "thing1")
	resName2 := resource.NewName(api2, "thing2")

	injectRobotRemote1 := &inject.Robot{}
	injectRobotRemote1.LoggerFunc = func() golog.Logger {
		return logger
	}
	injectRobotRemote1.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{
			resName1,
			resName2,
		}
	}
	injectRobotRemote1.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRobotRemote1.ResourceNames() {
			if rName == name {
				return grpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, resource.NewNotFoundError(name)
	}

	armDesc, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	baseDesc, err := grpcreflect.LoadServiceDescriptor(&basepb.BaseService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	boardDesc, err := grpcreflect.LoadServiceDescriptor(&boardpb.BoardService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	cameraDesc, err := grpcreflect.LoadServiceDescriptor(&camerapb.CameraService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	injectRobotRemote1.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
		return []resource.RPCAPI{
			{
				API:  api1,
				Desc: boardDesc,
			},
			{
				API:  api2,
				Desc: cameraDesc,
			},
		}
	}

	manager.addRemote(
		context.Background(),
		newDummyRobot(injectRobotRemote1),
		nil,
		config.Remote{Name: "remote1"},
	)

	injectRobotRemote2 := &inject.Robot{}
	injectRobotRemote2.LoggerFunc = func() golog.Logger {
		return logger
	}
	injectRobotRemote2.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{
			resName1,
			resName2,
		}
	}
	injectRobotRemote2.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRobotRemote2.ResourceNames() {
			if rName == name {
				return grpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, resource.NewNotFoundError(name)
	}

	gripperDesc, err := grpcreflect.LoadServiceDescriptor(&gripperpb.GripperService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	injectRobotRemote2.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
		return []resource.RPCAPI{
			{
				API:  api1,
				Desc: boardDesc,
			},
			{
				API:  api2,
				Desc: gripperDesc,
			},
		}
	}

	manager.addRemote(
		context.Background(),
		newDummyRobot(injectRobotRemote2),
		nil,
		config.Remote{Name: "remote2"},
	)

	apis := manager.ResourceRPCAPIs()
	test.That(t, apis, test.ShouldHaveLength, 4)

	apisM := make(map[resource.API]*desc.ServiceDescriptor, len(apis))
	for _, api := range apis {
		apisM[api.API] = api.Desc
	}

	test.That(t, apisM, test.ShouldContainKey, arm.API)
	test.That(t, cmp.Equal(apisM[arm.API].AsProto(), armDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, apisM, test.ShouldContainKey, base.API)
	test.That(t, cmp.Equal(apisM[base.API].AsProto(), baseDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, apisM, test.ShouldContainKey, api1)
	test.That(t, cmp.Equal(apisM[api1].AsProto(), boardDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, apisM, test.ShouldContainKey, api2)
	// one of these will be true due to a clash
	test.That(t,
		cmp.Equal(
			apisM[api2].AsProto(), cameraDesc.AsProto(), protocmp.Transform()) ||
			cmp.Equal(apisM[api2].AsProto(), gripperDesc.AsProto(), protocmp.Transform()),
		test.ShouldBeTrue)
}

func TestManagerEmptyResourceDesc(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}
	api := resource.APINamespaceRDK.WithComponentType("mockDesc")
	resource.RegisterAPI(
		api,
		resource.APIRegistration[resource.Resource]{},
	)
	defer func() {
		resource.DeregisterAPI(api)
	}()

	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{resource.NewName(api, "mock1")}
	}
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return rdktestutils.NewUnimplementedResource(name), nil
	}

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	}()

	apis := manager.ResourceRPCAPIs()
	test.That(t, apis, test.ShouldHaveLength, 0)
}

func TestReconfigure(t *testing.T) {
	const subtypeName = "testSubType"

	api := resource.APINamespaceRDK.WithServiceType(subtypeName)

	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)

	resource.RegisterAPI(api, resource.APIRegistration[resource.Resource]{})
	defer func() {
		resource.DeregisterAPI(api)
	}()

	resource.Register(api, resource.DefaultServiceModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return &mock{
				Named: conf.ResourceName().AsNamed(),
			}, nil
		},
	})
	defer func() {
		resource.Deregister(api, resource.DefaultServiceModel)
	}()

	manager := managerForDummyRobot(r)
	defer func() {
		test.That(t, manager.Close(ctx), test.ShouldBeNil)
	}()

	svc1 := resource.Config{
		Name:  "somesvc",
		Model: resource.DefaultServiceModel,
		API:   api,
	}

	local, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)
	newService, newlyBuilt, err := manager.processResource(ctx, svc1, resource.NewUninitializedNode(), local)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newlyBuilt, test.ShouldBeTrue)
	svcNode := resource.NewConfiguredGraphNode(svc1, newService, svc1.Model)
	manager.resources.AddNode(svc1.ResourceName(), svcNode)
	newService, newlyBuilt, err = manager.processResource(ctx, svc1, svcNode, local)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newlyBuilt, test.ShouldBeFalse)

	mockRe, ok := newService.(*mock)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, mockRe, test.ShouldNotBeNil)
	test.That(t, mockRe.reconfigCount, test.ShouldEqual, 1)

	defer func() {
		test.That(t, local.Close(ctx), test.ShouldBeNil)
	}()
}

func TestResourceCreationPanic(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	r, err := New(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)

	manager := managerForDummyRobot(r)
	defer func() {
		test.That(t, manager.Close(ctx), test.ShouldBeNil)
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	}()

	t.Run("component", func(t *testing.T) {
		subtypeName := "testComponentAPI"
		api := resource.APINamespaceRDK.WithComponentType(subtypeName)
		model := resource.DefaultModelFamily.WithModel("test")

		resource.RegisterComponent(api, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(ctx context.Context, deps resource.Dependencies, c resource.Config, logger golog.Logger) (resource.Resource, error) {
				panic("hello")
			},
		})
		defer func() {
			resource.Deregister(api, model)
		}()

		svc1 := resource.Config{
			Name:  "test",
			Model: model,
			API:   api,
		}

		local, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		_, _, err = manager.processResource(ctx, svc1, resource.NewUninitializedNode(), local)
		test.That(t, err.Error(), test.ShouldContainSubstring, "hello")
	})

	t.Run("service", func(t *testing.T) {
		subtypeName := "testServiceAPI"
		api := resource.APINamespaceRDK.WithServiceType(subtypeName)

		resource.Register(api, resource.DefaultServiceModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				c resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				panic("hello")
			},
		})
		defer func() {
			resource.Deregister(api, resource.DefaultServiceModel)
		}()

		resource.RegisterAPI(api, resource.APIRegistration[resource.Resource]{})
		defer func() {
			resource.DeregisterAPI(api)
		}()

		svc1 := resource.Config{
			Name:  "",
			Model: resource.DefaultServiceModel,
			API:   api,
		}

		local, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		_, _, err = manager.processResource(ctx, svc1, resource.NewUninitializedNode(), local)
		test.That(t, err.Error(), test.ShouldContainSubstring, "hello")
	})
}

type mock struct {
	resource.Named
	resource.TriviallyCloseable
	reconfigCount int
}

func (m *mock) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	m.reconfigCount++
	return nil
}

// A dummyRobot implements wraps an robot.Robot. It's only use for testing purposes.
type dummyRobot struct {
	resource.Named
	mu         sync.Mutex
	robot      robot.Robot
	manager    *resourceManager
	modmanager modmaninterface.ModuleManager
}

// newDummyRobot returns a new dummy robot wrapping a given robot.Robot
// and its configuration.
func newDummyRobot(robot robot.Robot) *dummyRobot {
	remoteManager := managerForDummyRobot(robot)
	remote := &dummyRobot{
		Named:   resource.NewName(client.RemoteAPI, "something").AsNamed(),
		robot:   robot,
		manager: remoteManager,
	}
	return remote
}

func (rr *dummyRobot) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	return errors.New("unsupported")
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (rr *dummyRobot) DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error) {
	return rr.robot.DiscoverComponents(ctx, qs)
}

func (rr *dummyRobot) RemoteNames() []string {
	return nil
}

func (rr *dummyRobot) ResourceNames() []resource.Name {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	names := rr.manager.ResourceNames()
	newNames := make([]resource.Name, 0, len(names))
	newNames = append(newNames, names...)
	return newNames
}

func (rr *dummyRobot) ResourceRPCAPIs() []resource.RPCAPI {
	return rr.robot.ResourceRPCAPIs()
}

func (rr *dummyRobot) RemoteByName(name string) (robot.Robot, bool) {
	return nil, false
}

func (rr *dummyRobot) ResourceByName(name resource.Name) (resource.Resource, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.manager.ResourceByName(name)
}

// FrameSystemConfig returns a remote robot's FrameSystem Config.
func (rr *dummyRobot) FrameSystemConfig(ctx context.Context) (*framesystem.Config, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) TransformPointCloud(ctx context.Context, srcpc pointcloud.PointCloud, srcName, dstName string,
) (pointcloud.PointCloud, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) Status(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) ProcessManager() pexec.ProcessManager {
	panic("change to return nil")
}

func (rr *dummyRobot) OperationManager() *operation.Manager {
	panic("change to return nil")
}

func (rr *dummyRobot) ModuleManager() modmaninterface.ModuleManager {
	return rr.modmanager
}

func (rr *dummyRobot) SessionManager() session.Manager {
	panic("change to return nil")
}

func (rr *dummyRobot) PackageManager() packages.Manager {
	panic("change to return nil")
}

func (rr *dummyRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *dummyRobot) Close(ctx context.Context) error {
	return rr.robot.Close(ctx)
}

func (rr *dummyRobot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	return rr.robot.StopAll(ctx, extra)
}

// managerForDummyRobot integrates all parts from a given robot
// except for its remotes.
func managerForDummyRobot(robot robot.Robot) *resourceManager {
	manager := newResourceManager(resourceManagerOptions{}, robot.Logger().Named("manager"))

	// start a dummy module manager so calls to moduleManager.Provides() do not
	// panic.
	manager.startModuleManager("", nil, false, robot.Logger())

	for _, name := range robot.ResourceNames() {
		res, err := robot.ResourceByName(name)
		if err != nil {
			robot.Logger().Debugw("error getting resource", "resource", name, "error", err)
			continue
		}
		gNode := resource.NewConfiguredGraphNode(resource.Config{}, res, unknownModel)
		manager.resources.AddNode(name, gNode)
	}
	return manager
}
