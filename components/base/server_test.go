package base_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	pbcommon "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errMoveStraight     = errors.New("critical failure in MoveStraight")
	errSpinFailed       = errors.New("critical failure in Spin")
	errPropertiesFailed = errors.New("critical failure in Properties")
	errStopFailed       = errors.New("critical failure in Stop")
)

func newServer() (pb.BaseServiceServer, *inject.Base, *inject.Base, error) {
	workingBase := &inject.Base{}
	brokenBase := &inject.Base{}
	bases := map[resource.Name]base.Base{
		base.Named(testBaseName): workingBase,
		base.Named(failBaseName): brokenBase,
	}
	baseSvc, err := resource.NewAPIResourceCollection(base.API, bases)
	if err != nil {
		return nil, nil, nil, err
	}
	return base.NewRPCServiceServer(baseSvc).(pb.BaseServiceServer), workingBase, brokenBase, nil
}

func TestServer(t *testing.T) {
	server, workingBase, brokenBase, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	t.Run("MoveStraight", func(t *testing.T) {
		speed := 2.3
		distance := int64(1)

		// on successful move straight
		workingBase.MoveStraightFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64,
			extra map[string]interface{},
		) error {
			return nil
		}
		req := &pb.MoveStraightRequest{
			Name:       testBaseName,
			MmPerSec:   speed,
			DistanceMm: distance,
		}
		resp, err := server.MoveStraight(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.MoveStraightResponse{})

		// on failing move straight
		brokenBase.MoveStraightFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64,
			extra map[string]interface{},
		) error {
			return errMoveStraight
		}
		req = &pb.MoveStraightRequest{
			Name:       failBaseName,
			MmPerSec:   speed,
			DistanceMm: distance,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errMoveStraight)

		// failure on unfound base
		req = &pb.MoveStraightRequest{
			Name:       "dne",
			MmPerSec:   speed,
			DistanceMm: distance,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})

	t.Run("Spin", func(t *testing.T) {
		angSpeed := 45.0
		angle := 90.0
		// on successful spin
		workingBase.SpinFunc = func(
			ctx context.Context,
			angleDeg, degsPerSec float64,
			extra map[string]interface{},
		) error {
			return nil
		}
		req := &pb.SpinRequest{
			Name:       testBaseName,
			DegsPerSec: angSpeed,
			AngleDeg:   angle,
		}
		resp, err := server.Spin(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.SpinResponse{})

		// on failing spin
		brokenBase.SpinFunc = func(
			ctx context.Context,
			angleDeg, degsPerSec float64,
			extra map[string]interface{},
		) error {
			return errSpinFailed
		}
		req = &pb.SpinRequest{
			Name:       failBaseName,
			DegsPerSec: angSpeed,
			AngleDeg:   angle,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errSpinFailed)

		// failure on unfound base
		req = &pb.SpinRequest{
			Name:       "dne",
			DegsPerSec: angSpeed,
			AngleDeg:   angle,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})

	t.Run("Properties", func(t *testing.T) {
		turnRadius := 0.1
		width := 0.2
		// on a successful get properties
		workingBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{
				TurningRadiusMeters: turnRadius,
				WidthMeters:         width,
			}, nil
		}
		req := &pb.GetPropertiesRequest{Name: testBaseName}
		resp, err := server.GetProperties(context.Background(), req) // TODO (rh) rename server to bServer after review
		test.That(t, resp, test.ShouldResemble, &pb.GetPropertiesResponse{WidthMeters: width, TurningRadiusMeters: turnRadius})
		test.That(t, err, test.ShouldBeNil)

		// on a failing get properties
		brokenBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{}, errPropertiesFailed
		}
		req = &pb.GetPropertiesRequest{Name: failBaseName}
		resp, err = server.GetProperties(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errPropertiesFailed)

		// failure on base not found
		req = &pb.GetPropertiesRequest{Name: "dne"}
		resp, err = server.GetProperties(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})

	t.Run("Stop", func(t *testing.T) {
		// on successful stop
		workingBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		}
		req := &pb.StopRequest{Name: testBaseName}
		resp, err := server.Stop(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.StopResponse{})

		// on failing stop
		brokenBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			return errStopFailed
		}
		req = &pb.StopRequest{Name: failBaseName}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errStopFailed)

		// failure on base not found
		req = &pb.StopRequest{Name: "dne"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})

	t.Run("Geometries", func(t *testing.T) {
		box, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewZeroPose().Orientation()),
			r3.Vector{},
			testBaseName,
		)
		test.That(t, err, test.ShouldBeNil)

		// on a successful get geometries
		workingBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return []spatialmath.Geometry{box}, nil
		}
		req := &pbcommon.GetGeometriesRequest{Name: testBaseName}
		resp, err := server.GetGeometries(context.Background(), req) // TODO (rh) rename server to bServer after review
		test.That(t, resp, test.ShouldResemble, &pbcommon.GetGeometriesResponse{
			Geometries: spatialmath.NewGeometriesToProto([]spatialmath.Geometry{box}),
		})
		test.That(t, err, test.ShouldBeNil)

		// on a failing get properties
		brokenBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return nil, nil
		}
		req = &pbcommon.GetGeometriesRequest{Name: failBaseName}
		resp, err = server.GetGeometries(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, base.ErrGeometriesNil(failBaseName))
	})
}
