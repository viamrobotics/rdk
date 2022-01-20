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
		base.Named("working"):    workingBase,
		base.Named("notWorking"): brokenBase,
		base.Named("badBase"):    "not a base",
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
			ctx context.Context, distanceMillis int,
			millisPerSec float64, block bool,
		) error {
			return nil
		}
		req := &pb.BaseServiceMoveStraightRequest{
			Name:       "working",
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
			ctx context.Context, distanceMillis int,
			millisPerSec float64, block bool,
		) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceMoveStraightRequest{
			Name:       "notWorking",
			MmPerSec:   2.3,
			DistanceMm: 1,
		}
		resp, err = server.MoveStraight(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceMoveStraightRequest{
			Name:       "badBase",
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
			ctx context.Context, distanceMillis int,
			millisPerSec, degsPerSec float64, block bool,
		) error {
			return nil
		}
		req := &pb.BaseServiceMoveArcRequest{
			Name:       "working",
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
			ctx context.Context, distanceMillis int,
			millisPerSec float64, degsPerSec float64, block bool,
		) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceMoveArcRequest{
			Name:       "notWorking",
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
			Name:       "badBase",
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
			Name:       "working",
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
			Name:       "notWorking",
			DegsPerSec: 42.0,
			AngleDeg:   42.0,
			Block:      true,
		}
		resp, err = server.Spin(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceSpinRequest{
			Name:       "badBase",
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
		req := &pb.BaseServiceStopRequest{Name: "working"}
		resp, err := server.Stop(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &pb.BaseServiceStopResponse{})

		// on failing stop
		errMsg := "stop failed"
		brokenBase.StopFunc = func(ctx context.Context) error {
			return errors.New(errMsg)
		}
		req = &pb.BaseServiceStopRequest{Name: "notWorking"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceStopRequest{Name: "badBase"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceStopRequest{Name: "dne"}
		resp, err = server.Stop(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("WidthGet", func(t *testing.T) {
		// on successful retrieval
		workingBase.WidthGetFunc = func(ctx context.Context) (int, error) {
			return 42, nil
		}
		req := &pb.BaseServiceWidthGetRequest{Name: "working"}
		resp, err := server.WidthGet(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetWidthMm(), test.ShouldEqual, 42)

		// on failure to retrieve
		errMsg := "WidthGet failed"
		brokenBase.WidthGetFunc = func(ctx context.Context) (int, error) {
			return 0, errors.New(errMsg)
		}
		req = &pb.BaseServiceWidthGetRequest{Name: "notWorking"}
		resp, err = server.WidthGet(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New(errMsg))

		// failure on bad base handled
		req = &pb.BaseServiceWidthGetRequest{Name: "badBase"}
		resp, err = server.WidthGet(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		// failure on unfound base
		req = &pb.BaseServiceWidthGetRequest{Name: "dne"}
		resp, err = server.WidthGet(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
