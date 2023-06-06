package base_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/base/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
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
		errMsg := "move straight failed"
		brokenBase.MoveStraightFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64,
			extra map[string]interface{},
		) error {
			return errors.New(errMsg)
		}
		req = &pb.MoveStraightRequest{
			Name:       failBaseName,
			MmPerSec:   speed,
			DistanceMm: distance,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

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
		errMsg := "spin failed"
		brokenBase.SpinFunc = func(
			ctx context.Context,
			angleDeg, degsPerSec float64,
			extra map[string]interface{},
		) error {
			return errors.New(errMsg)
		}
		req = &pb.SpinRequest{
			Name:       failBaseName,
			DegsPerSec: angSpeed,
			AngleDeg:   angle,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

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
		workingBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Feature, error) {
			return base.Feature{
				TurningRadiusMeters: turnRadius,
				WidthMeters:         width,
			}, nil
		}
		req := &pb.GetPropertiesRequest{Name: testBaseName}
		resp, err := server.GetProperties(context.Background(), req) // TODO (rh) rename server to bServer after review
		test.That(t, resp, test.ShouldResemble, &pb.GetPropertiesResponse{WidthMeters: width, TurningRadiusMeters: turnRadius})
		test.That(t, err, test.ShouldBeNil)

		// on a failing get properties
		errMsg := "properties not found"

		brokenBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Feature, error) {
			return base.Feature{}, errors.New(errMsg)
		}
		req = &pb.GetPropertiesRequest{Name: failBaseName}
		resp, err = server.GetProperties(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

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
		errMsg := "stop failed"
		brokenBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			return errors.New(errMsg)
		}
		req = &pb.StopRequest{Name: failBaseName}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on base not found
		req = &pb.StopRequest{Name: "dne"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	})
}
