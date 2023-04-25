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

	pos1 := []float64{1.0, 2.0, 3.0}
	len1 := []float64{2.0, 3.0, 4.0}
	extra1 := map[string]interface{}{}
	injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return pos1, nil
	}
	injectGantry.MoveToPositionFunc = func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
		gantryPos = pos
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
	injectGantry2.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errors.New("can't get position")
	}
	injectGantry2.MoveToPositionFunc = func(ctx context.Context, pos []float64, extra map[string]interface{}) error {
		gantryPos = pos
		return errors.New("can't move to position")
	}
	injectGantry2.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errors.New("can't get lengths")
	}
	injectGantry2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("no stop")
	}

	//nolint:dupl
	t.Run("gantry position", func(t *testing.T) {
		_, err := gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "123", "bar": 234})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.PositionsMm, test.ShouldResemble, pos1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": "123", "bar": 234.})

		_, err = gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get position")
	})

	t.Run("move to position", func(t *testing.T) {
		_, err := gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: missingGantryName, PositionsMm: pos2},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": "234", "bar": 345})
		test.That(t, err, test.ShouldBeNil)
		_, err = gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: testGantryName, PositionsMm: pos2, Extra: ext},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gantryPos, test.ShouldResemble, pos2)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": "234", "bar": 345.})

		_, err = gantryServer.MoveToPosition(
			context.Background(),
			&pb.MoveToPositionRequest{Name: failGantryName, PositionsMm: pos1},
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't move to position")
		test.That(t, gantryPos, test.ShouldResemble, pos1)
	})

	//nolint:dupl
	t.Run("lengths", func(t *testing.T) {
		_, err := gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": 123, "bar": "234"})
		test.That(t, err, test.ShouldBeNil)
		resp, err := gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.LengthsMm, test.ShouldResemble, len1)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 123., "bar": "234"})

		_, err = gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get lengths")
	})

	t.Run("stop", func(t *testing.T) {
		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		ext, err := protoutils.StructToStructPb(map[string]interface{}{"foo": 234, "bar": "123"})
		test.That(t, err, test.ShouldBeNil)
		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: testGantryName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extra1, test.ShouldResemble, map[string]interface{}{"foo": 234., "bar": "123"})

		_, err = gantryServer.Stop(context.Background(), &pb.StopRequest{Name: failGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no stop")
	})
}
