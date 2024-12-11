package gantry_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/gantry/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testGantryName    = "gantry1"
	testGantryName2   = "gantry2"
	failGantryName    = "gantry3"
	missingGantryName = "gantry4"
)

var (
	errPositionFailed       = errors.New("couldn't get position")
	errHomingFailed         = errors.New("homing unsuccessful")
	errMoveToPositionFailed = errors.New("couldn't move to position")
	errLengthsFailed        = errors.New("couldn't get lengths")
	errStopFailed           = errors.New("couldn't stop")
	errGantryNotFound       = errors.New("not found")
)

func newServer() (pb.GantryServiceServer, *inject.Gantry, *inject.Gantry, error) {
	injectGantry := &inject.Gantry{}
	injectGantry2 := &inject.Gantry{}
	gantries := map[resource.Name]gantry.Gantry{
		gantry.Named(testGantryName): injectGantry,
		gantry.Named(failGantryName): injectGantry2,
	}
	gantrySvc, err := resource.NewAPIResourceCollection(gantry.API, gantries)
	if err != nil {
		return nil, nil, nil, err
	}
	return gantry.NewRPCServiceServer(gantrySvc).(pb.GantryServiceServer), injectGantry, injectGantry2, nil
}

func TestServer(t *testing.T) {
	gantryServer, injectGantry, injectGantry2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var gantryPos []float64
	var gantrySpeed []float64

	pos1 := []float64{1.0, 2.0, 3.0}
	speed1 := []float64{100.0, 200.0, 300.0}
	len1 := []float64{2.0, 3.0, 4.0}
	extra1 := map[string]interface{}{}
	injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return pos1, nil
	}
	injectGantry.HomeFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		extra1 = extra
		return true, nil
	}
	injectGantry.MoveToPositionFunc = func(ctx context.Context, pos, speed []float64, extra map[string]interface{}) error {
		gantryPos = pos
		gantrySpeed = speed
		extra1 = extra
		return nil
	}
	injectGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return len1, nil
	}
	injectGantry.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extra1 = extra
		return nil
	}

	pos2 := []float64{4.0, 5.0, 6.0}
	speed2 := []float64{100.0, 80.0, 120.0}
	injectGantry2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errPositionFailed
	}
	injectGantry2.HomeFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		extra1 = extra
		return false, errHomingFailed
	}
	injectGantry2.MoveToPositionFunc = func(ctx context.Context, pos, speed []float64, extra map[string]interface{}) error {
		gantryPos = pos
		gantrySpeed = speed
		return errMoveToPositionFailed
	}
	injectGantry2.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errLengthsFailed
	}
	injectGantry2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopFailed
	}

	//nolint:dupl
	t.Run("gantry position", func(t *testing.T) {
		_, err := gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGantryNotFound.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "123", "bar": 234})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.PositionsMm, test.ShouldResemble, pos1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": "123", "bar": 234.})

		_, err = gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPositionFailed.Error())

		// Redefine Positionfunc to test nil return
		injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return nil, nil
		}
		resp, err = gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.PositionsMm, test.ShouldResemble, []float64{})
	})

	t.Run("move to position", func(t *testing.T) {
		_, err := gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: missingGantryName, PositionsMm: pos2, SpeedsMmPerSec: speed2},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGantryNotFound.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "234", "bar": 345})
		test.That(t, err, test.ShouldBeNil)
		_, err = gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: testGantryName, PositionsMm: pos2, SpeedsMmPerSec: speed2, Extra: ext},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gantryPos, test.ShouldResemble, pos2)
		test.That(t, gantrySpeed, test.ShouldResemble, speed2)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": "234", "bar": 345.})

		_, err = gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: failGantryName, PositionsMm: pos1, SpeedsMmPerSec: speed1},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveToPositionFailed.Error())
		test.That(t, gantryPos, test.ShouldResemble, pos1)
		test.That(t, gantrySpeed, test.ShouldResemble, speed1)
	})

	//nolint:dupl
	t.Run("lengths", func(t *testing.T) {
		_, err := gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGantryNotFound.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": 123, "bar": "234"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LengthsMm, test.ShouldResemble, len1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 123., "bar": "234"})

		_, err = gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errLengthsFailed.Error())

		// Redefine Lengthsfunc to test nil return
		injectGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return nil, nil
		}
		resp, err = gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LengthsMm, test.ShouldResemble, []float64{})
	})

	t.Run("home", func(t *testing.T) {
		_, err := gantryServer.Home(context.Background(), &pb.HomeRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGantryNotFound.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": 123, "bar": "234"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gantryServer.Home(context.Background(), &pb.HomeRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Homed, test.ShouldBeTrue)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 123., "bar": "234"})

		resp, err = gantryServer.Home(context.Background(), &pb.HomeRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp.Homed, test.ShouldBeFalse)
		test.That(t, err.Error(), test.ShouldContainSubstring, errHomingFailed.Error())
	})

	t.Run("stop", func(t *testing.T) {
		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGantryNotFound.Error())

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": 234, "bar": "123"})
		test.That(t, err, test.ShouldBeNil)
		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 234., "bar": "123"})

		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopFailed.Error())
	})
}
