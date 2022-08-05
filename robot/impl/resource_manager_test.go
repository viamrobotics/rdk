package robotimpl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/google/go-cmp/cmp"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/protobuf/testing/protocmp"

	"go.viam.com/rdk/component/arm"
	fakearm "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/base"
	fakebase "go.viam.com/rdk/component/base/fake"
	"go.viam.com/rdk/component/board"
	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/camera"
	fakecamera "go.viam.com/rdk/component/camera/fake"
	"go.viam.com/rdk/component/gripper"
	fakegripper "go.viam.com/rdk/component/gripper/fake"
	"go.viam.com/rdk/component/input"
	fakeinput "go.viam.com/rdk/component/input/fake"
	"go.viam.com/rdk/component/motor"
	fakemotor "go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/component/servo"
	fakeservo "go.viam.com/rdk/component/servo/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	basepb "go.viam.com/rdk/proto/api/component/base/v1"
	boardpb "go.viam.com/rdk/proto/api/component/board/v1"
	camerapb "go.viam.com/rdk/proto/api/component/camera/v1"
	gripperpb "go.viam.com/rdk/proto/api/component/gripper/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
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
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		if _, ok := utils.NewStringSet(injectRobot.RemoteNames()...)[name]; !ok {
			return nil, false
		}
		return &dummyRobot{}, true
	}

	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.Subtype {
				case arm.Subtype:
					return &fakearm.Arm{Name: name.Name}, nil
				case base.Subtype:
					return &fakebase.Base{Name: name.Name}, nil
				case board.Subtype:
					fakeBoard, err := fakeboard.NewBoard(context.Background(), config.Component{
						Name: name.Name,
						ConvertedAttributes: &board.Config{
							Analogs: []board.AnalogConfig{
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
				case camera.Subtype:
					return &fakecamera.Camera{Name: name.Name}, nil
				case gripper.Subtype:
					return &fakegripper.Gripper{Name: name.Name}, nil
				case input.Subtype:
					return &fakeinput.InputController{Name: name.Name}, nil
				case motor.Subtype:
					return &fakemotor.Motor{Name: name.Name}, nil
				case servo.Subtype:
					return &fakeservo.Servo{Name: name.Name}, nil
				}
				if rName.ResourceType == resource.ResourceTypeService {
					return struct{}{}, nil
				}
			}
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	return injectRobot
}

func TestManagerForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
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
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), setupInjectRobot(logger)),
		config.Remote{Name: "remote1"}, nil,
	)
	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), setupInjectRobot(logger)),
		config.Remote{Name: "remote2"}, nil,
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
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) { return struct{}{}, nil }
	injectRobot.LoggerFunc = func() golog.Logger { return logger }

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()

	injectRemote := &inject.Robot{}
	injectRemote.ResourceNamesFunc = func() []resource.Name { return rdktestutils.AddSuffixes(armNames, "") }
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRemote.ResourceByNameFunc = func(name resource.Name) (interface{}, error) { return struct{}{}, nil }
	injectRemote.LoggerFunc = func() golog.Logger { return logger }
	manager.addRemote(context.Background(),
		injectRemote,
		config.Remote{Name: "remote1"}, nil,
	)

	manager.updateRemotesResourceNames(context.Background(), nil)

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
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), setupInjectRobot(logger)),
		config.Remote{Name: "remote1"}, nil,
	)
	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), setupInjectRobot(logger)),
		config.Remote{Name: "remote2"}, nil,
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
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), setupInjectRobot(logger)),
		config.Remote{Name: "remote1"}, nil,
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
	cfg := &config.Component{Type: arm.SubtypeName, Name: "arm1"}
	rName := cfg.ResourceName()
	manager.addResource(rName, injectArm)
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
		return &fakeboard.Analog{}, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return &board.BasicDigitalInterrupt{}, true
	}

	cfg = &config.Component{
		Type:      board.SubtypeName,
		Namespace: resource.ResourceNamespaceRDK,
		Name:      "board1",
	}
	rName = cfg.ResourceName()
	manager.addResource(rName, injectBoard)
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
		worldState *commonpb.WorldState,
	) (bool, error) {
		return false, nil
	}
	objectMResName := motion.Named("motion1")
	manager.addResource(objectMResName, injectMotionService)
	motionService, err := manager.ResourceByName(objectMResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motionService, test.ShouldEqual, injectMotionService)

	injectVisionService := &inject.VisionService{}
	injectVisionService.GetObjectPointCloudsFunc = func(
		ctx context.Context,
		cameraName, segmenterName string,
		parameters config.AttributeMap,
	) ([]*viz.Object, error) {
		return []*viz.Object{viz.NewEmptyObject()}, nil
	}
	objectSegResName := vision.Named("builtin")
	manager.addResource(objectSegResName, injectVisionService)
	objectSegmentationService, err := manager.ResourceByName(objectSegResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objectSegmentationService, test.ShouldEqual, injectVisionService)
}

func TestManagerNewComponent(t *testing.T) {
	cfg := &config.Config{
		Components: []config.Component{
			{
				Name:      "arm1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "arm2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "arm3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "base1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "base2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "base3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:                "board1",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board2",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board3",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:      "camera1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "camera2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "camera3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "gripper1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm1", "camera1"},
			},
			{
				Name:      "gripper2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm2", "camera2"},
			},
			{
				Name:      "gripper3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm3", "camera3"},
			},
			{
				Name:                "inputController1",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                input.SubtypeName,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "inputController2",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                input.SubtypeName,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "inputController3",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                input.SubtypeName,
				ConvertedAttributes: &fakeinput.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:                "motor1",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "motor2",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "motor3",
				Model:               "fake",
				Namespace:           resource.ResourceNamespaceRDK,
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:      "sensor1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "sensor2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "sensor3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "servo1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "servo2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "servo3",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
				DependsOn: []string{"board3"},
			},
		},
	}
	logger := golog.NewTestLogger(t)
	robotForRemote := &localRobot{
		manager: newResourceManager(resourceManagerOptions{}, logger),
		logger:  logger,
		config:  cfg,
	}
	diff, err := config.DiffConfigs(&config.Config{}, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robotForRemote.manager.updateResources(context.Background(), diff, func(name string) (resource.Name, bool) {
		for _, c := range cfg.Components {
			if c.Name == name {
				return c.ResourceName(), true
			}
		}
		return resource.Name{}, false
	}), test.ShouldBeNil)
	robotForRemote.config.Components[8].DependsOn = append(robotForRemote.config.Components[8].DependsOn, "arm3")
	_, err = config.SortComponents(robotForRemote.config.Components)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "circular dependency detected in component list between arm3, board3")

	diff = &config.Diff{
		Added: &config.Config{},
		Modified: &config.ModifiedConfigDiff{
			Components: []config.Component{},
		},
	}

	diff.Modified.Components = append(diff.Modified.Components, config.Component{
		Name:                "board3",
		Model:               "fake",
		Namespace:           resource.ResourceNamespaceRDK,
		Type:                board.SubtypeName,
		ConvertedAttributes: &board.Config{},
		DependsOn:           []string{"arm3"},
	})
	err = robotForRemote.manager.updateResources(context.Background(), diff, func(name string) (resource.Name, bool) {
		for _, c := range cfg.Components {
			if c.Name == name {
				return c.ResourceName(), true
			}
		}
		return resource.Name{}, false
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "circular dependency - \"arm3\" already depends on \"board3\"")
}

func managerForTest(ctx context.Context, t *testing.T, l golog.Logger) *resourceManager {
	t.Helper()
	injectRobot := setupInjectRobot(l)
	manager := managerForDummyRobot(injectRobot)

	manager.addRemote(context.Background(),
		newDummyRobot(ctx, setupInjectRobot(l)),
		config.Remote{Name: "remote1"}, nil,
	)
	manager.addRemote(context.Background(),
		newDummyRobot(ctx, setupInjectRobot(l)),
		config.Remote{Name: "remote2"}, nil,
	)
	_, err := manager.processManager.AddProcess(ctx, &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.processManager.AddProcess(ctx, &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)
	return manager
}

func TestManagerFilterFromConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)

	ctx, cancel := context.WithCancel(context.Background())
	manager := managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	checkEmpty := func(toCheck *resourceManager) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, toCheck.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}

	filtered, err := manager.FilterFromConfig(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "what",
			},
		},
		Components: []config.Component{
			{
				Name:      "what1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "what5",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "what3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "what4",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "what5",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "what6",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "what7",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},
			{
				Name:      "what8",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "what",
				Name: "echo",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Components: []config.Component{
			{
				Name:      "what1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Components: []config.Component{
			{
				Name:      "arm2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "base2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "board2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "camera2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "gripper2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "inputController2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      input.SubtypeName,
			},
			{
				Name:      "motor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "sensor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},

			{
				Name:      "servo2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base2")}
	boardNames := []resource.Name{board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo2")}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldBeEmpty)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
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
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote2",
			},
		},
		Components: []config.Component{
			{
				Name:      "arm2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "base2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "board2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "camera2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "gripper2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "inputController2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      input.SubtypeName,
			},
			{
				Name:      "motor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "sensor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},
			{
				Name:      "servo2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

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
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
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
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	test.That(t, manager.Close(ctx), test.ShouldBeNil)
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	manager = managerForTest(ctx, t, logger)
	test.That(t, manager, test.ShouldNotBeNil)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
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
		Components: []config.Component{
			{
				Name:      "arm1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "arm2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "arm3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
			},
			{
				Name:      "base1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "base2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "base3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      base.SubtypeName,
			},
			{
				Name:      "board1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "board2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "board3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      board.SubtypeName,
			},
			{
				Name:      "camera1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "camera2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "camera3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      camera.SubtypeName,
			},
			{
				Name:      "gripper1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "gripper2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "gripper3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      gripper.SubtypeName,
			},
			{
				Name:      "inputController1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      input.SubtypeName,
			},
			{
				Name:      "inputController2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      input.SubtypeName,
			},
			{
				Name:      "inputController3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      input.SubtypeName,
			},
			{
				Name:      "motor1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "motor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "motor3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      motor.SubtypeName,
			},
			{
				Name:      "sensor1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},
			{
				Name:      "sensor2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},
			{
				Name:      "sensor3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      sensor.SubtypeName,
			},
			{
				Name:      "servo1",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
			},
			{
				Name:      "servo2",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
			},
			{
				Name:      "servo3",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      servo.SubtypeName,
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
	test.That(t, err, test.ShouldBeNil)

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
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
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
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
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
	cert, _, _, certPool, err := testutils.GenerateSelfSignedCertificate("somename", altName)
	test.That(t, err, test.ShouldBeNil)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""
	listener := testutils.ReserveRandomListener(t)
	addr := listener.Addr().String()
	options.Network.Listener = listener
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
			Config: config.AttributeMap{
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

func TestManagerResourceRPCSubtypes(t *testing.T) {
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
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.Subtype {
				case arm.Subtype:
					return &fakearm.Arm{Name: name.Name}, nil
				case base.Subtype:
					return &fakebase.Base{Name: name.Name}, nil
				}
			}
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	manager := managerForDummyRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()

	subtype1 := resource.NewSubtype("acme", resource.ResourceTypeComponent, "huwat")
	subtype2 := resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat")

	resName1 := resource.NameFromSubtype(subtype1, "thing1")
	resName2 := resource.NameFromSubtype(subtype2, "thing2")
	// resName3 := resource.NameFromSubtype(subtype1, "thing3")
	// resName4 := resource.NameFromSubtype(subtype2, "thing4")

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
	injectRobotRemote1.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		for _, rName := range injectRobotRemote1.ResourceNames() {
			if rName == name {
				return grpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	armDesc, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	baseDesc, err := grpcreflect.LoadServiceDescriptor(&basepb.BaseService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	boardDesc, err := grpcreflect.LoadServiceDescriptor(&boardpb.BoardService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	cameraDesc, err := grpcreflect.LoadServiceDescriptor(&camerapb.CameraService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	injectRobotRemote1.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype {
		return []resource.RPCSubtype{
			{
				Subtype: subtype1,
				Desc:    boardDesc,
			},
			{
				Subtype: subtype2,
				Desc:    cameraDesc,
			},
		}
	}

	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), injectRobotRemote1),
		config.Remote{Name: "remote1"}, nil,
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
	injectRobotRemote2.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		for _, rName := range injectRobotRemote2.ResourceNames() {
			if rName == name {
				return grpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	gripperDesc, err := grpcreflect.LoadServiceDescriptor(&gripperpb.GripperService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	injectRobotRemote2.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype {
		return []resource.RPCSubtype{
			{
				Subtype: subtype1,
				Desc:    boardDesc,
			},
			{
				Subtype: subtype2,
				Desc:    gripperDesc,
			},
		}
	}

	manager.addRemote(context.Background(),
		newDummyRobot(context.Background(), injectRobotRemote2),
		config.Remote{Name: "remote2"}, nil,
	)

	subtypes := manager.ResourceRPCSubtypes()
	test.That(t, subtypes, test.ShouldHaveLength, 4)

	subtypesM := make(map[resource.Subtype]*desc.ServiceDescriptor, len(subtypes))
	for _, subtype := range subtypes {
		subtypesM[subtype.Subtype] = subtype.Desc
	}

	test.That(t, subtypesM, test.ShouldContainKey, arm.Subtype)
	test.That(t, cmp.Equal(subtypesM[arm.Subtype].AsProto(), armDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, subtypesM, test.ShouldContainKey, base.Subtype)
	test.That(t, cmp.Equal(subtypesM[base.Subtype].AsProto(), baseDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, subtypesM, test.ShouldContainKey, subtype1)
	test.That(t, cmp.Equal(subtypesM[subtype1].AsProto(), boardDesc.AsProto(), protocmp.Transform()), test.ShouldBeTrue)

	test.That(t, subtypesM, test.ShouldContainKey, subtype2)
	// one of these will be true due to a clash
	test.That(t,
		cmp.Equal(
			subtypesM[subtype2].AsProto(), cameraDesc.AsProto(), protocmp.Transform()) ||
			cmp.Equal(subtypesM[subtype2].AsProto(), gripperDesc.AsProto(), protocmp.Transform()),
		test.ShouldBeTrue)
}

func TestUpdateConfig(t *testing.T) {
	// given a service subtype is reconfigurable, check if it has been reconfigured
	const SubtypeName = resource.SubtypeName("testSubType")

	Subtype := resource.NewSubtype(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeService,
		SubtypeName,
	)

	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)

	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})

	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return &mock{}, nil
		},
	})

	manager := managerForDummyRobot(r)
	defer func() {
		test.That(t, utils.TryClose(ctx, manager), test.ShouldBeNil)
	}()

	svc1 := config.Service{Name: "", Namespace: resource.ResourceNamespaceRDK, Type: "testSubType"}

	local, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)
	newService, err := manager.processService(ctx, svc1, nil, local)
	test.That(t, err, test.ShouldBeNil)
	newService, err = manager.processService(ctx, svc1, newService, local)
	test.That(t, err, test.ShouldBeNil)

	mockRe, ok := newService.(*mock)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, mockRe, test.ShouldNotBeNil)
	test.That(t, mockRe.reconfigCount, test.ShouldEqual, 1)
	test.That(t, mockRe.wrap, test.ShouldEqual, 1)

	defer func() {
		test.That(t, utils.TryClose(ctx, local), test.ShouldBeNil)
	}()
}

var _ = resource.Reconfigurable(&mock{})

func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	sMock, _ := s.(*mock)
	sMock.wrap++
	return sMock, nil
}

type mock struct {
	wrap          int
	reconfigCount int
}

func (m *mock) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	m.reconfigCount++
	return nil
}

// A dummyRobot implements wraps an robot.Robot. It's only use for testing purposes.
type dummyRobot struct {
	mu      sync.Mutex
	robot   robot.Robot
	manager *resourceManager
}

// newDummyRobot returns a new dummy robot wrapping a given robot.Robot
// and its configuration.
func newDummyRobot(ctx context.Context, robot robot.Robot) *dummyRobot {
	remoteManager := managerForDummyRobot(robot)
	remote := &dummyRobot{
		robot:   robot,
		manager: remoteManager,
	}
	return remote
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (rr *dummyRobot) DiscoverComponents(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error) {
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

func (rr *dummyRobot) ResourceRPCSubtypes() []resource.RPCSubtype {
	return rr.robot.ResourceRPCSubtypes()
}

func (rr *dummyRobot) RemoteByName(name string) (robot.Robot, bool) {
	return nil, false
}

func (rr *dummyRobot) ResourceByName(name resource.Name) (interface{}, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.manager.ResourceByName(name)
}

// FrameSystemConfig returns a remote robot's FrameSystem Config.
func (rr *dummyRobot) FrameSystemConfig(
	ctx context.Context,
	additionalTransforms []*commonpb.Transform,
) (framesystemparts.Parts, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	panic("change to return nil")
}

func (rr *dummyRobot) ProcessManager() pexec.ProcessManager {
	panic("change to return nil")
}

func (rr *dummyRobot) OperationManager() *operation.Manager {
	panic("change to return nil")
}

func (rr *dummyRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *dummyRobot) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rr.robot)
}

func (rr *dummyRobot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	return rr.robot.StopAll(ctx, extra)
}

// managerForDummyRobot integrates all parts from a given robot
// except for its remotes.
func managerForDummyRobot(robot robot.Robot) *resourceManager {
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
