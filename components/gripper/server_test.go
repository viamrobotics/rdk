package gripper_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	pbcommon "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errCantOpen          = errors.New("can't open")
	errCantGrab          = errors.New("can't grab")
	errStopUnimplemented = errors.New("stop unimplemented")
	errGripperNotFound   = errors.New("not found")
)

func newServer() (pb.GripperServiceServer, *inject.Gripper, *inject.Gripper, error) {
	injectGripper := &inject.Gripper{}
	injectGripper2 := &inject.Gripper{}
	grippers := map[resource.Name]gripper.Gripper{
		gripper.Named(testGripperName):  injectGripper,
		gripper.Named(testGripperName2): injectGripper2,
	}
	gripperSvc, err := resource.NewAPIResourceCollection(gripper.API, grippers)
	if err != nil {
		return nil, nil, nil, err
	}
	return gripper.NewRPCServiceServer(gripperSvc).(pb.GripperServiceServer), injectGripper, injectGripper2, nil
}

func TestServer(t *testing.T) {
	gripperServer, injectGripper, injectGripper2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var gripperOpen string
	var extraOptions map[string]interface{}

	success1 := true
	injectGripper.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		gripperOpen = testGripperName
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		extraOptions = extra
		return success1, nil
	}
	injectGripper.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		return nil
	}
	injectGripper.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		box, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewZeroPose().Orientation()),
			r3.Vector{},
			testGripperName,
		)
		return []spatialmath.Geometry{box}, err
	}

	injectGripper2.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		gripperOpen = testGripperName2
		return errCantOpen
	}
	injectGripper2.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return false, errCantGrab
	}
	injectGripper2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopUnimplemented
	}
	injectGripper2.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return nil, nil
	}

	t.Run("open", func(t *testing.T) {
		_, err := gripperServer.Open(context.Background(), &pb.OpenRequest{Name: missingGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())

		extra := map[string]interface{}{"foo": "Open"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		_, err = gripperServer.Open(context.Background(), &pb.OpenRequest{Name: testGripperName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = gripperServer.Open(context.Background(), &pb.OpenRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantOpen.Error())
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName2)
	})

	t.Run("grab", func(t *testing.T) {
		_, err := gripperServer.Grab(context.Background(), &pb.GrabRequest{Name: missingGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())

		extra := map[string]interface{}{"foo": "Grab"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		resp, err := gripperServer.Grab(context.Background(), &pb.GrabRequest{Name: testGripperName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Success, test.ShouldEqual, success1)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		resp, err = gripperServer.Grab(context.Background(), &pb.GrabRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGrab.Error())
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("stop", func(t *testing.T) {
		_, err = gripperServer.Stop(context.Background(), &pb.StopRequest{Name: missingGripperName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())

		extra := map[string]interface{}{"foo": "Stop"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		_, err = gripperServer.Stop(context.Background(), &pb.StopRequest{Name: testGripperName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = gripperServer.Stop(context.Background(), &pb.StopRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errStopUnimplemented)
	})

	t.Run("geometries", func(t *testing.T) {
		_, err = gripperServer.GetGeometries(context.Background(), &pbcommon.GetGeometriesRequest{Name: testGripperName})
		test.That(t, err, test.ShouldBeNil)

		_, err = gripperServer.GetGeometries(context.Background(), &pbcommon.GetGeometriesRequest{Name: testGripperName2})
		test.That(t, err, test.ShouldBeError, gripper.ErrGeometriesNil(testGripperName2))
	})
}
