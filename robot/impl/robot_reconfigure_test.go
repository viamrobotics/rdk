package robotimpl

import (
	"context"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/metadata/service"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	coretestutils "go.viam.com/core/testutils"
)

func TestRobotReconfigure(t *testing.T) {
	ConfigFromFile := func(t *testing.T, filePath string) *config.Config {
		conf, err := config.Read(filePath)
		test.That(t, err, test.ShouldBeNil)
		return conf
	}
	mockSubtype := resource.NewSubtype(resource.ResourceNamespaceCore, resource.ResourceTypeComponent, resource.SubtypeName("mock"))
	mockNamed := func(name string) resource.Name {
		return resource.NewFromSubtype(mockSubtype, name)
	}
	registry.RegisterComponent(mockSubtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return &mockFake{x: 5}, nil
		},
	})
	registry.RegisterComponent(mockSubtype, "fake2", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return &mockFake2{x: 5}, nil
		},
	})

	t.Run("no diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")

		ctx := context.Background()
		svc, err := service.New()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(svc.All()), test.ShouldEqual, 1)
		ctx = service.ContextWithService(ctx, svc)

		robot, err := New(ctx, conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()
		test.That(t, len(svc.All()), test.ShouldEqual, 6)
		rCopy := make([]resource.Name, 6)
		copy(rCopy, svc.All())

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, robot.Reconfigure(ctx, conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		_, ok = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		test.That(t, rCopy, test.ShouldResemble, svc.All())
	})

	t.Run("empty to additive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")

		ctx := context.Background()
		svc, err := service.New()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(svc.All()), test.ShouldEqual, 1)
		ctx = service.ContextWithService(ctx, svc)

		robot, err := New(ctx, emptyConf, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()
		test.That(t, len(svc.All()), test.ShouldEqual, 1)

		test.That(t, robot.Reconfigure(ctx, emptyConf), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		test.That(t, robot.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		test.That(t, robot.Reconfigure(ctx, conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		_, ok = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		test.That(t, svc, test.ShouldResemble, service.ContextService(ctx))
		test.That(t, len(svc.All()), test.ShouldEqual, 6)
	})

	t.Run("additive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf4 := ConfigFromFile(t, "data/diff_config_4.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, robot.Reconfigure(context.Background(), conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, robot.Reconfigure(context.Background(), conf4), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		analog1, ok := board1.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, analog1.(*proxyBoardAnalogReader).actual.(*fake.Analog).CloseCount, test.ShouldEqual, 0)

		analog2, ok := board1.AnalogReaderByName("analog2")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, analog2.(*proxyBoardAnalogReader).actual.(*fake.Analog).CloseCount, test.ShouldEqual, 0)

		_, ok = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)
	})

	t.Run("subtractive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		_, ok = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		test.That(t, robot.Reconfigure(context.Background(), emptyConf), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		test.That(t, robot.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		_, ok = robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeFalse)

		_, ok = robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 1)

		_, ok = robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 1)

		_, ok = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeFalse)

		_, ok = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeFalse)

		_, ok = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeFalse)
	})

	t.Run("modificative diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf2 := ConfigFromFile(t, "data/diff_config_2.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		base1Proxy := base1.(*proxyBase)
		base1Actual := base1Proxy.actual.(*fake.Base)
		test.That(t, base1Actual.CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		board1Proxy := board1.(*proxyBoard)
		board1Actual := board1Proxy.actual
		test.That(t, board1Actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		analog1, ok := board1Proxy.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		analog1Proxy := analog1.(*proxyBoardAnalogReader)
		analog1Actual := analog1Proxy.actual
		test.That(t, analog1Actual.(*fake.Analog).CloseCount, test.ShouldEqual, 0)

		resource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		test.That(t, robot.Reconfigure(context.Background(), conf2), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, base1Actual.CloseCount, test.ShouldEqual, 1)
		test.That(t, board1Actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)
		test.That(t, analog1Actual.(*fake.Analog).CloseCount, test.ShouldEqual, 1)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 1)

		newArm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newArm1, test.ShouldEqual, arm1)

		newBase1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBase1, test.ShouldEqual, base1Proxy)

		newBoard1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBoard1, test.ShouldEqual, board1Proxy)

		newAnalog1, ok := newBoard1.AnalogReaderByName("analog1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newAnalog1, test.ShouldEqual, analog1Proxy)

		_, ok = newBoard1.AnalogReaderByName("analog2")
		test.That(t, ok, test.ShouldBeFalse)

		newResource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newResource1, test.ShouldEqual, resource1)

		newMock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock1, test.ShouldEqual, mock1)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("mixed diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		base1Proxy := base1.(*proxyBase)
		base1Actual := base1Proxy.actual.(*fake.Base)
		test.That(t, base1Actual.CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		board1Proxy := board1.(*proxyBoard)
		board1Actual := board1Proxy.actual
		test.That(t, board1Actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		resource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		test.That(t, robot.Reconfigure(context.Background(), conf3), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "3"))

		test.That(t, base1Actual.CloseCount, test.ShouldEqual, 1)
		test.That(t, board1Actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 1)

		newArm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newArm1, test.ShouldEqual, arm1)

		_, ok = robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeFalse)

		newBoard1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBoard1, test.ShouldEqual, board1Proxy)

		base2, ok := robot.BaseByName("base2")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base2.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board2, ok := robot.BoardByName("board2")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board2.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		newResource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newResource1, test.ShouldEqual, resource1)

		newMock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock1, test.ShouldEqual, mock1)

		_, ok = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeFalse)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("3")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("reconfiguring unreconfigurable", func(t *testing.T) {
		// processing modify will fail
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_4_bad.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		resource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		err = robot.Reconfigure(context.Background(), conf3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "*robotimpl.mockFake2 is not reconfigurable")

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		newArm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newArm1, test.ShouldEqual, arm1)
		newBase1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBase1, test.ShouldEqual, base1)
		newBoard1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBoard1, test.ShouldEqual, board1)
		newResource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newResource1, test.ShouldEqual, resource1)

		newMock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock1, test.ShouldEqual, mock1)

		newMock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock2, test.ShouldEqual, mock2)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("rollback", func(t *testing.T) {
		// processing modify will fail
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3_bad.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources := map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)

		base1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		resource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)

		mock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconCount, test.ShouldEqual, 0)

		mock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconCount, test.ShouldEqual, 0)

		err = robot.Reconfigure(context.Background(), conf3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error processing draft changes")
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldBeEmpty)
		resources = map[resource.Subtype][]resource.Name{
			arm.Subtype: {arm.Named("arm1")},
			mockSubtype: {mockNamed("mock1"), mockNamed("mock2")},
		}
		test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.ResourceMapToStringSet(resources))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, base1.(*proxyBase).actual.(*fake.Base).CloseCount, test.ShouldEqual, 0)
		test.That(t, board1.(*proxyBoard).actual.(*fake.Board).CloseCount, test.ShouldEqual, 0)

		newArm1, ok := robot.ArmByName("arm1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newArm1, test.ShouldEqual, arm1)
		newBase1, ok := robot.BaseByName("base1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBase1, test.ShouldEqual, base1)
		newBoard1, ok := robot.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newBoard1, test.ShouldEqual, board1)
		newResource1, ok := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newResource1, test.ShouldEqual, resource1)
		newMock1, ok := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock1, test.ShouldEqual, mock1)

		newMock2, ok := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, newMock2, test.ShouldEqual, mock2)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})
}

type mockFake struct {
	x          int
	reconCount int
}

func (m *mockFake) Reconfigure(newResource resource.Reconfigurable) error {
	new, ok := newResource.(*mockFake)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", m, newResource)
	}
	m.x = new.x
	m.reconCount++
	return nil
}

type mockFake2 struct {
	x          int
	reconCount int
}
