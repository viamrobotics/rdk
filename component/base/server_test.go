package base_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/base"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.BaseServiceServer, *inject.Base, *inject.Base, error) {
	workingBase := &inject.Base{}
	brokenBase := &inject.Base{}
	bases := map[resource.Name]interface{}{
		base.Named(testBaseName): workingBase,
		base.Named(failBaseName): brokenBase,
		base.Named(fakeBaseName): "not a base",
	}
	baseSvc, err := subtype.New(bases)
	if err != nil {
		return nil, nil, nil, err
	}
	return base.NewServer(baseSvc), workingBase, brokenBase, nil
}

func TestServer(t *testing.T) {
	server, workingBase, brokenBase, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	t.Run("MoveStraight", func(t *testing.T) {
		// on successful move straight
		workingBase.MoveStraightFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64, block bool,
		) error {
			return nil
		}
		req := &pb.BaseServiceMoveStraightRequest{
			Name:       testBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
			Block:      true,
		}
		resp, err := server.MoveStraight(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.BaseServiceMoveStraightResponse{})

		// on failing move straight
		errMsg := "move straight failed"
		brokenBase.MoveStraightFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64, block bool,
		) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceMoveStraightRequest{
			Name:       failBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceMoveStraightRequest{
			Name:       fakeBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceMoveStraightRequest{
			Name:       "dne",
			MmPerSec:   2.3,
			DistanceMm: 1,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("MoveArc", func(t *testing.T) {
		// on successful move arc
		workingBase.MoveArcFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec, angleDeg float64, block bool,
		) error {
			return nil
		}
		req := &pb.BaseServiceMoveArcRequest{
			Name:       testBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err := server.MoveArc(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.BaseServiceMoveArcResponse{})

		// on failing move arc
		errMsg := "move arc failed"
		brokenBase.MoveArcFunc = func(
			ctx context.Context, distanceMm int,
			mmPerSec float64, angleDeg float64, block bool,
		) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceMoveArcRequest{
			Name:       failBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.MoveArc(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceMoveArcRequest{
			Name:       fakeBaseName,
			MmPerSec:   2.3,
			DistanceMm: 1,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.MoveArc(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceMoveArcRequest{
			Name:       "dne",
			MmPerSec:   2.3,
			DistanceMm: 1,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.MoveArc(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("Spin", func(t *testing.T) {
		// on successful spin
		workingBase.SpinFunc = func(
			ctx context.Context,
			angleDeg, degsPerSec float64, block bool,
		) error {
			return nil
		}
		req := &pb.BaseServiceSpinRequest{
			Name:       testBaseName,
			DegsPerSec: 42.0,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err := server.Spin(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.BaseServiceSpinResponse{})

		// on failing spin
		errMsg := "spin failed"
		brokenBase.SpinFunc = func(
			ctx context.Context,
			angleDeg, degsPerSec float64, block bool,
		) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceSpinRequest{
			Name:       failBaseName,
			DegsPerSec: 42.0,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceSpinRequest{
			Name:       fakeBaseName,
			DegsPerSec: 42.0,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceSpinRequest{
			Name:       "dne",
			DegsPerSec: 42.0,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("Stop", func(t *testing.T) {
		// on successful stop
		workingBase.StopFunc = func(ctx context.Context) error {
			return nil
		}
		req := &pb.BaseServiceStopRequest{Name: testBaseName}
		resp, err := server.Stop(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.BaseServiceStopResponse{})

		// on failing stop
		errMsg := "stop failed"
		brokenBase.StopFunc = func(ctx context.Context) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceStopRequest{Name: failBaseName}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceStopRequest{Name: fakeBaseName}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceStopRequest{Name: "dne"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
