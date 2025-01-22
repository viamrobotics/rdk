package robotimpl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.mongodb.org/mongo-driver/bson/primitive"
	armpb "go.viam.com/api/component/arm/v1"
	basepb "go.viam.com/api/component/base/v1"
	boardpb "go.viam.com/api/component/board/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	gripperpb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"gonum.org/v1/gonum/stat/combin"
	"google.golang.org/protobuf/testing/protocmp"

	"go.viam.com/rdk/cloud"
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
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	weboptions "go.viam.com/rdk/robot/web/options"
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

func setupInjectRobot(logger logging.Logger) *inject.Robot {
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
	injectRobot.LoggerFunc = func() logging.Logger {
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
					return fakecamera.NewCamera(context.Background(), resource.Dependencies{}, conf, logger)
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
	logger := logging.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(t, injectRobot)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}

	test.That(t, manager.RemoteNames(), test.ShouldBeEmpty)
	rdktestutils.VerifySameResourceNames(
		t,
		manager.ResourceNames(),
		rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		),
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
	logger := logging.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(t, injectRobot)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(logger)),
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

	rdktestutils.VerifySameElements(t, manager.RemoteNames(), []string{"remote1", "remote2"})
	rdktestutils.VerifySameResourceNames(
		t,
		manager.ResourceNames(),
		rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		),
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
	logger := logging.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	injectRobot.ResourceNamesFunc = func() []resource.Name { return armNames }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return rdktestutils.NewUnimplementedResource(name), nil
	}
	injectRobot.LoggerFunc = func() logging.Logger { return logger }

	manager := managerForDummyRobot(t, injectRobot)

	injectRemote := &inject.Robot{}
	injectRemote.ResourceNamesFunc = func() []resource.Name { return rdktestutils.AddSuffixes(armNames, "") }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRemote.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return rdktestutils.NewUnimplementedResource(name), nil
	}
	injectRemote.LoggerFunc = func() logging.Logger { return logger }
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, injectRemote),
		nil,
		config.Remote{Name: "remote1"},
	)

	manager.updateRemotesResourceNames(context.Background())

	rdktestutils.VerifySameResourceNames(
		t,
		manager.remoteResourceNames(fromRemoteNameToRemoteNodeName("remote1")),
		[]resource.Name{arm.Named("remote1:arm1"), arm.Named("remote1:arm2")},
	)
}

func TestManagerWithSameNameInRemoteNoPrefix(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(t, injectRobot)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote2"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1:arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerWithSameNameInBaseAndRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(t, injectRobot)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(logger)),
		nil,
		config.Remote{Name: "remote1"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1:arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerAdd(t *testing.T) {
	logger := logging.NewTestLogger(t)
	manager := newResourceManager(resourceManagerOptions{}, logger)

	injectArm := &inject.Arm{}
	cfg := &resource.Config{API: arm.API, Name: "arm1"}
	rName := cfg.ResourceName()
	manager.resources.AddNode(rName, resource.NewConfiguredGraphNode(*cfg, injectArm, cfg.Model))
	arm1, err := manager.ResourceByName(rName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1, test.ShouldEqual, injectArm)

	injectBoard := &inject.Board{}
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
	logger := logging.NewTestLogger(t)
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

func managerForTest(ctx context.Context, t *testing.T, l logging.Logger) *resourceManager {
	t.Helper()
	injectRobot := setupInjectRobot(l)
	manager := managerForDummyRobot(t, injectRobot)

	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(l)),
		nil,
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(
		context.Background(),
		newDummyRobot(t, setupInjectRobot(l)),
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
	logger := logging.NewTestLogger(t)

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
		test.That(t, procMan.ProcessIDs(), test.ShouldBeEmpty)
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
	rdktestutils.VerifySameElements(t, processesToRemove.ProcessIDs(), []string{"2"})

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
	rdktestutils.VerifySameElements(t, processesToRemove.ProcessIDs(), []string{"2"})

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
	rdktestutils.VerifySameElements(t, processesToRemove.ProcessIDs(), []string{"1", "2"})
	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()
}

func TestConfigRemoteAllowInsecureCreds(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r := setupLocalRobot(t, ctx, cfg, logger)

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

	gNode := resource.NewUninitializedNode()
	gNode.InitializeLogger(logger, "remote")
	_, err = manager.processRemote(context.Background(), remote, gNode)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remote.Auth.Entity = "wrong"
	_, err = manager.processRemote(context.Background(), remote, gNode)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remote.Auth.Entity = options.FQDN
	_, err = manager.processRemote(context.Background(), remote, gNode)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")
}

func TestConfigUntrustedEnv(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
func (fp *fakeProcess) KillGroup() {}

func (fp *fakeProcess) Status() error {
	return nil
}

func (fp *fakeProcess) UnixPid() (int, error) {
	return 0, errors.New("unimplemented")
}

func TestManagerResourceRPCAPIs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	injectRobot.LoggerFunc = func() logging.Logger {
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

	manager := managerForDummyRobot(t, injectRobot)

	api1 := resource.APINamespace("acme").WithComponentType("huwat")
	api2 := resource.APINamespace("acme").WithComponentType("wat")

	resName1 := resource.NewName(api1, "thing1")
	resName2 := resource.NewName(api2, "thing2")

	injectRobotRemote1 := &inject.Robot{}
	injectRobotRemote1.LoggerFunc = func() logging.Logger {
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
		newDummyRobot(t, injectRobotRemote1),
		nil,
		config.Remote{Name: "remote1"},
	)

	injectRobotRemote2 := &inject.Robot{}
	injectRobotRemote2.LoggerFunc = func() logging.Logger {
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
		newDummyRobot(t, injectRobotRemote2),
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
	logger := logging.NewTestLogger(t)
	injectRobot := &inject.Robot{}
	injectRobot.LoggerFunc = func() logging.Logger {
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

	manager := managerForDummyRobot(t, injectRobot)

	apis := manager.ResourceRPCAPIs()
	test.That(t, apis, test.ShouldHaveLength, 0)
}

func TestReconfigure(t *testing.T) {
	const subtypeName = "testSubType"

	api := resource.APINamespaceRDK.WithServiceType(subtypeName)

	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r := setupLocalRobot(t, ctx, cfg, logger)

	resource.RegisterAPI(api, resource.APIRegistration[resource.Resource]{})
	defer func() {
		resource.DeregisterAPI(api)
	}()

	resource.Register(api, resource.DefaultServiceModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return &mock{
				Named: conf.ResourceName().AsNamed(),
			}, nil
		},
	})
	defer func() {
		resource.Deregister(api, resource.DefaultServiceModel)
	}()

	manager := managerForDummyRobot(t, r)

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

func TestRemoteConnClosedOnReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	ctx := context.Background()

	fakeMotor := resource.Config{
		Name:                "motor",
		Model:               resource.DefaultModelFamily.WithModel("fake"),
		API:                 motor.API,
		ConvertedAttributes: &fakemotor.Config{},
	}

	fakeArm := resource.Config{
		Name:                "arm",
		Model:               resource.DefaultModelFamily.WithModel("fake"),
		API:                 arm.API,
		ConvertedAttributes: &fakearm.Config{},
	}

	remoteCfg1 := &config.Config{
		Components: []resource.Config{fakeMotor},
	}

	remoteCfg2 := &config.Config{
		Components: []resource.Config{fakeArm},
	}

	t.Run("remotes with same exact resources", func(t *testing.T) {
		remote1 := setupLocalRobot(t, ctx, remoteCfg1, logger.Sublogger("remote1"))
		remote2 := setupLocalRobot(t, ctx, remoteCfg1, logger.Sublogger("remote2"))

		options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
		err := remote1.StartWeb(ctx, options1)
		test.That(t, err, test.ShouldBeNil)

		options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
		err = remote2.StartWeb(ctx, options2)
		test.That(t, err, test.ShouldBeNil)

		// Set up a local main robot which is connected to remote1
		remoteConf := config.Remote{
			Name:    "remote",
			Address: addr1,
		}

		mainRobotCfg := &config.Config{
			Remotes: []config.Remote{remoteConf},
		}

		// Make a copy of the main robot config as reconfigure will directly modify it
		mainCfgCopy := *mainRobotCfg
		mainRobot := setupLocalRobot(t, ctx, mainRobotCfg, logger.Sublogger("main"))

		// Grab motor of remote1 to check that it won't work after switching remotes
		motor1, err := motor.FromRobot(mainRobot, "motor")
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err := motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Change address in config copy to reference address of the remote2, make a copy of the config, and reconfigure
		mainCfgCopy.Remotes[0].Address = addr2
		mainCfgCopy2 := mainCfgCopy
		mainRobot.Reconfigure(ctx, &mainCfgCopy)

		// Verify that motor of remote1 no longer works
		_, _, err = motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)

		// Grab motor of remote2 to check that it won't work after switching remotes
		motor2, err := motor.FromRobot(mainRobot, "motor")
		test.That(t, err, test.ShouldBeNil)

		moving, speed, _ = motor2.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Change address back to remote1, and reconfigure
		mainCfgCopy2.Remotes[0].Address = addr1
		mainRobot.Reconfigure(ctx, &mainCfgCopy2)

		// Close second remote robot
		remote2.Close(ctx)

		// Verify that motor of remote2 no longer works
		_, _, err = motor2.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)

		// Check that we were able to grab the motor from remote1 through the main robot and successfully call IsPowered()
		motor1, err = motor.FromRobot(mainRobot, "motor")
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)
	})

	t.Run("remotes with different resources", func(t *testing.T) {
		remote1 := setupLocalRobot(t, ctx, remoteCfg2, logger.Sublogger("remote1"))
		remote2 := setupLocalRobot(t, ctx, remoteCfg1, logger.Sublogger("remote2"))

		options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
		err := remote1.StartWeb(ctx, options1)
		test.That(t, err, test.ShouldBeNil)

		options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
		err = remote2.StartWeb(ctx, options2)
		test.That(t, err, test.ShouldBeNil)

		// Set up a local main robot which is connected to remote1
		remoteConf := config.Remote{
			Name:    "remote",
			Address: addr1,
		}

		mainRobotCfg := &config.Config{
			Remotes: []config.Remote{remoteConf},
		}

		// Make a copy of the main robot config as reconfigure will directly modify it
		mainCfgCopy := *mainRobotCfg
		mainRobot := setupLocalRobot(t, ctx, mainRobotCfg, logger.Sublogger("main"))

		// Grab arm of remote1 to check that it won't work after switching remotes
		arm1, err := arm.FromRobot(mainRobot, "arm")
		test.That(t, err, test.ShouldBeNil)

		moving, err := arm1.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)

		// Change address in config copy to reference address of the remote2, make a copy of the config, and reconfigure
		mainCfgCopy.Remotes[0].Address = addr2
		mainCfgCopy2 := mainCfgCopy
		mainRobot.Reconfigure(ctx, &mainCfgCopy)

		// Verify that arm of remote1 no longer works
		_, err = arm1.IsMoving(ctx)
		test.That(t, err, test.ShouldNotBeNil)

		// Grab motor of remote2 to check that it won't work after switching remotes
		motor1, err := motor.FromRobot(mainRobot, "motor")
		test.That(t, err, test.ShouldBeNil)

		moving, speed, _ := motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Change address back to remote1, and reconfigure
		mainCfgCopy2.Remotes[0].Address = addr1
		mainRobot.Reconfigure(ctx, &mainCfgCopy2)

		// Close second remote robot
		remote2.Close(ctx)

		// Verify that motor of remote2 no longer works
		_, _, err = motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)

		// Check that we were able to grab the arm from remote1 through the main robot and successfully call IsMoving()
		arm1, err = arm.FromRobot(mainRobot, "arm")
		test.That(t, err, test.ShouldBeNil)

		moving, err = arm1.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
	})
}

func TestResourceCreationPanic(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	r := setupLocalRobot(t, ctx, &config.Config{}, logger)
	manager := managerForDummyRobot(t, r)

	t.Run("component", func(t *testing.T) {
		subtypeName := "testComponentAPI"
		api := resource.APINamespaceRDK.WithComponentType(subtypeName)
		model := resource.DefaultModelFamily.WithModel("test")

		resource.RegisterComponent(api, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, c resource.Config, logger logging.Logger,
			) (resource.Resource, error) {
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
		_, _, err := manager.processResource(ctx, svc1, resource.NewUninitializedNode(), local)
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
				logger logging.Logger,
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
		_, _, err := manager.processResource(ctx, svc1, resource.NewUninitializedNode(), local)
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

	offline bool
}

// newDummyRobot returns a new dummy robot wrapping a given robot.Robot
// and its configuration.
func newDummyRobot(t *testing.T, robot robot.Robot) *dummyRobot {
	t.Helper()

	remoteManager := managerForDummyRobot(t, robot)
	remote := &dummyRobot{
		Named:   resource.NewName(client.RemoteAPI, "something").AsNamed(),
		robot:   robot,
		manager: remoteManager,
	}
	return remote
}

func (rr *dummyRobot) SetOffline(offline bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.offline = offline
}

func (rr *dummyRobot) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return errors.New("offline")
	}
	return errors.New("unsupported")
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (rr *dummyRobot) DiscoverComponents(ctx context.Context, qs []resource.DiscoveryQuery) ([]resource.Discovery, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return nil, errors.New("offline")
	}
	return rr.robot.DiscoverComponents(ctx, qs)
}

func (rr *dummyRobot) RemoteNames() []string {
	return nil
}

func (rr *dummyRobot) ResourceNames() []resource.Name {
	// NOTE: The offline behavior here should resemble behavior in the robot client
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return nil
	}
	names := rr.manager.ResourceNames()
	newNames := make([]resource.Name, 0, len(names))
	newNames = append(newNames, names...)
	return newNames
}

func (rr *dummyRobot) ResourceRPCAPIs() []resource.RPCAPI {
	// NOTE: The offline behavior here should resemble behavior in the robot client
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return nil
	}
	return rr.robot.ResourceRPCAPIs()
}

func (rr *dummyRobot) RemoteByName(name string) (robot.Robot, bool) {
	return nil, false
}

func (rr *dummyRobot) ResourceByName(name resource.Name) (resource.Resource, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return nil, errors.New("offline")
	}
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

func (rr *dummyRobot) Logger() logging.Logger {
	return rr.robot.Logger()
}

func (rr *dummyRobot) CloudMetadata(ctx context.Context) (cloud.Metadata, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return cloud.Metadata{}, errors.New("offline")
	}
	return rr.robot.CloudMetadata(ctx)
}

func (rr *dummyRobot) Close(ctx context.Context) error {
	return rr.robot.Close(ctx)
}

func (rr *dummyRobot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return errors.New("offline")
	}
	return rr.robot.StopAll(ctx, extra)
}

func (rr *dummyRobot) RestartModule(ctx context.Context, req robot.RestartModuleRequest) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return errors.New("offline")
	}
	return rr.robot.RestartModule(ctx, req)
}

func (rr *dummyRobot) Shutdown(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return errors.New("offline")
	}
	return rr.robot.Shutdown(ctx)
}

func (rr *dummyRobot) MachineStatus(ctx context.Context) (robot.MachineStatus, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return robot.MachineStatus{}, errors.New("offline")
	}
	return rr.robot.MachineStatus(ctx)
}

func (rr *dummyRobot) Version(ctx context.Context) (robot.VersionResponse, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if rr.offline {
		return robot.VersionResponse{}, errors.New("offline")
	}
	return rr.robot.Version(ctx)
}

// managerForDummyRobot integrates all parts from a given robot except for its remotes.
// It also close itself when the test and all subtests complete.
func managerForDummyRobot(t *testing.T, robot robot.Robot) *resourceManager {
	t.Helper()

	manager := newResourceManager(resourceManagerOptions{}, robot.Logger().Sublogger("manager"))
	t.Cleanup(func() {
		test.That(t, manager.Close(context.Background()), test.ShouldBeNil)
	})

	// start a dummy module manager so calls to moduleManager.Provides() do not
	// panic.
	manager.startModuleManager(context.Background(), "", nil, false, "", "", robot.Logger(), t.TempDir())

	for _, name := range robot.ResourceNames() {
		res, err := robot.ResourceByName(name)
		if err != nil {
			robot.Logger().Debugw("error getting resource", "resource", name, "error", err)
			continue
		}
		gNode := resource.NewConfiguredGraphNode(resource.Config{}, res, unknownModel)
		test.That(t, manager.resources.AddNode(name, gNode), test.ShouldBeNil)
	}
	return manager
}

// TestReconfigureParity validates that calling the synchronous and asynchronous version
// of `Reconfigure` results in the same resource state on a local robot for all
// combinations of an initial and updated configuration.
func TestReconfigureParity(t *testing.T) {
	// TODO(RSDK-7716): add some configurations with modules.
	// TODO(RSDK-8065): define configurations in-line instead of reading from files.
	files := []string{
		"data/diff_config_deps1.json",
		"data/diff_config_deps2.json",
		"data/diff_config_deps3.json",
		"data/diff_config_deps4.json",
		"data/diff_config_deps5.json",
		"data/diff_config_deps6.json",
		"data/diff_config_deps7.json",
		"data/diff_config_deps8.json",
		"data/diff_config_deps9_good.json",
		"data/diff_config_deps9_bad.json",
		"data/diff_config_deps10.json",
		"data/diff_config_deps11.json",
		"data/diff_config_deps12.json",
	}
	ctx := context.Background()

	testReconfigureParity := func(t *testing.T, initCfg, updateCfg string) {
		name := fmt.Sprintf("%s -> %s", initCfg, updateCfg)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Capture logs for this sub-test run. Only output the logs if the test fails.
			logger := logging.NewInMemoryLogger(t)

			// Configuration may mutate `*config.Config`, so we read it from
			// file each time.
			cfg := ConfigFromFile(t, initCfg)
			r1 := setupLocalRobot(t, ctx, cfg, logger).(*localRobot)
			cfg = ConfigFromFile(t, initCfg)
			r2 := setupLocalRobot(t, ctx, cfg, logger).(*localRobot)

			rdktestutils.VerifySameResourceNames(t, r1.ResourceNames(), r2.ResourceNames())

			cfg = ConfigFromFile(t, updateCfg)
			r1.Reconfigure(ctx, cfg)
			cfg = ConfigFromFile(t, updateCfg)
			// force robot to reconfigure resources serially
			r2.reconfigure(ctx, cfg, true)

			rdktestutils.VerifySameResourceNames(t, r1.ResourceNames(), r2.ResourceNames())
		})
	}

	gen := combin.NewCombinationGenerator(len(files), 2)

	pair := []int{0, 0}
	for gen.Next() {
		gen.Combination(pair)

		i, j := pair[0], pair[1]
		testReconfigureParity(t, files[i], files[j])

		i, j = pair[1], pair[0]
		testReconfigureParity(t, files[i], files[j])
	}
}

// Consider a case where a main part viam-server is configured with a remote part viam-server. We
// want to ensure that once calling `ResourceNames` on the main part returns some remote resource --
// that remote resource will always be returned until it is configured away. When either the remote
// robot removes it from its config. Or when the main part removes the remote.
func TestOfflineRemoteResources(t *testing.T) {
	logger, _ := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	motorName := "remoteMotorFoo"
	motorResourceName := resource.NewName(motor.API, motorName)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                motorName,
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
	}

	remoteRobot := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	remoteOptions, _, remoteAddr := robottestutils.CreateBaseOptionsAndListener(t)
	err := remoteRobot.StartWeb(ctx, remoteOptions)
	test.That(t, err, test.ShouldBeNil)

	// Set up a local main robot which is connected to the remote.
	mainRobotCfg := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: remoteAddr,
				// These values dictate how quickly we'll observe the remote going offline. And how
				// quickly we'll observe it coming back online.
				ConnectionCheckInterval: 10 * time.Millisecond,
				ReconnectInterval:       10 * time.Millisecond,
			},
		},
	}
	mainRobotI := setupLocalRobot(t, ctx, mainRobotCfg, logger.Sublogger("main"))
	// We'll manually access the resource manager to move the test forward.
	mainRobot := mainRobotI.(*localRobot)
	mainOptions, _, mainAddr := robottestutils.CreateBaseOptionsAndListener(t)
	mainRobot.StartWeb(ctx, mainOptions)

	// Create an "application" client to the robot.
	mainClient, err := client.New(ctx, mainAddr, logger.Sublogger("client"))
	test.That(t, err, test.ShouldBeNil)
	defer mainClient.Close(ctx)
	resourceNames := mainClient.ResourceNames()

	// When the `mainClient` requests `ResourceNames`, the motor will be annotated to include its
	// remote.
	motorResourceNameFromMain := motorResourceName.PrependRemote("remote")
	// Search the list of "main" resources for the remote motor. Sanity check that we find it.
	test.That(t, resourceNames, test.ShouldContain, motorResourceNameFromMain)

	// Grab the RobotClient resource graph node from the main robot that is connected to the
	// remote. We'll use this to know when the main robot observes the remote has gone offline.
	mainToRemoteClientRes, _ := mainRobot.RemoteByName("remote")
	test.That(t, mainToRemoteClientRes, test.ShouldNotBeNil)
	mainToRemoteClient := mainToRemoteClientRes.(*client.RobotClient)
	test.That(t, mainToRemoteClient.Connected(), test.ShouldBeTrue)

	// Stop the remote's web server. Wait for the main robot to observe there's a connection problem.
	logger.Info("Stopping web")
	remoteRobot.StopWeb()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, mainToRemoteClient.Connected(), test.ShouldBeFalse)
	})

	// Manually kick the resource manager to update remote resource names.
	logger.Info("Updating remote resource names")
	mainRobot.manager.updateRemotesResourceNames(logging.EnableDebugModeWithKey(ctx, "testOfflineRemoteResources.nodeOffline"))

	// The robot client keeps a cache of resource names. Manually refresh before re-asking the main
	// robot what resources it hsa.
	mainClient.Refresh(ctx)
	resourceNames = mainClient.ResourceNames()

	// Scan again for the remote motor. Assert it still exists.
	test.That(t, resourceNames, test.ShouldContain, motorResourceNameFromMain)

	// Restart the remote web server. We closed the old listener, so just pass in the web address as
	// part of the web options.
	logger.Info("Restarting web server")
	err = remoteRobot.StartWeb(ctx, weboptions.Options{
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress: remoteAddr,
			},
		},
	})
	test.That(t, err, test.ShouldBeNil)

	// Wait until the main robot sees the remote is online. This gets stuck behind a 10 second dial
	// timeout. So we must manually increase the time we're willing to wait.
	testutils.WaitForAssertionWithSleep(t, 50*time.Millisecond, 1000, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, mainToRemoteClient.Connected(), test.ShouldBeTrue)
	})

	// Again, manually refresh the list of resources to clear the cache. Assert the remote motor
	// still exists.
	mainToRemoteClient.Refresh(logging.EnableDebugModeWithKey(ctx, "refresh"))
	test.That(t, resourceNames, test.ShouldContain, motorResourceNameFromMain)

	// Reconfigure away the motor on the remote robot.
	remoteCfg.Components = []resource.Config{}
	remoteRobot.Reconfigure(ctx, remoteCfg)

	// Assert the main robot's client object eventually observes that the motor is no longer a
	// component.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		mainToRemoteClient.Refresh(ctx)
		resourceNames := mainToRemoteClient.ResourceNames()
		test.That(t, resourceNames, test.ShouldNotContain, motorResourceNameFromMain)
	})

	// Manually update remote resource names. Knowing the robot client servicing the information has
	// the updated view.
	mainRobot.manager.updateRemotesResourceNames(ctx)

	// Force a refresh of resource names on the application client connection. Assert the motor no
	// longer appears.
	mainClient.Refresh(ctx)
	test.That(t, resourceNames, test.ShouldNotContain, mainClient.ResourceNames())
}
