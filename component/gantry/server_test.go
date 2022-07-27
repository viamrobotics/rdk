package gantry_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/gantry"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/gantry/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func newServer() (pb.GantryServiceServer, *inject.Gantry, *inject.Gantry, error) {
	injectGantry := &inject.Gantry{}
	injectGantry2 := &inject.Gantry{}
	gantries := map[resource.Name]interface{}{
		gantry.Named(testGantryName): injectGantry,
		gantry.Named(failGantryName): injectGantry2,
		gantry.Named(fakeGantryName): "notGantry",
	}
	gantrySvc, err := subtype.New(gantries)
	if err != nil {
		return nil, nil, nil, err
	}
	return gantry.NewServer(gantrySvc), injectGantry, injectGantry2, nil
}

func TestServer(t *testing.T) {
	gantryServer, injectGantry, injectGantry2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var gantryPos []float64

	pos1 := []float64{1.0, 2.0, 3.0}
	len1 := []float64{2.0, 3.0, 4.0}
	extra1 := map[string]interface{}{}
	injectGantry.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return pos1, nil
	}
	injectGantry.MoveToPositionFunc = func(
		ctx context.Context,
		pos []float64,
		worldState *commonpb.WorldState,
		extra map[string]interface{},
	) error {
		gantryPos = pos
		extra1 = extra
		return nil
	}
	injectGantry.GetLengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		extra1 = extra
		return len1, nil
	}
	injectGantry.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extra1 = extra
		return nil
	}

	pos2 := []float64{4.0, 5.0, 6.0}
	injectGantry2.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errors.New("can't get position")
	}
	injectGantry2.MoveToPositionFunc = func(
		ctx context.Context,
		pos []float64,
		worldState *commonpb.WorldState,
		extra map[string]interface{},
	) error {
		gantryPos = pos
		return errors.New("can't move to position")
	}
	injectGantry2.GetLengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return nil, errors.New("can't get lengths")
	}
	injectGantry2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("no stop")
	}

	t.Run("gantry position", func(t *testing.T) {
		_, err := gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gantry")

		_, err = gantryServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: fakeGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a gantry")

		ext, err := structpb.NewStruct(map[string]interface{}{"foo": "123", "bar": 234})
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
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gantry")

		ext, err := structpb.NewStruct(map[string]interface{}{"foo": "234", "bar": 345})
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

	t.Run("lengths", func(t *testing.T) {
		_, err := gantryServer.GetLengths(context.Background(), &pb.GetLengthsRequest{Name: missingGantryName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gantry")

		ext, err := structpb.NewStruct(map[string]interface{}{"foo": 123, "bar": "234"})
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
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gantry")

		ext, err := structpb.NewStruct(map[string]interface{}{"foo": 234, "bar": "123"})
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
