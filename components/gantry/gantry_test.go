package gantry_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
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

func TestStatusValid(t *testing.T) {
	status := &pb.Status{
		PositionsMm: []float64{1.1, 2.2, 3.3},
		LengthsMm:   []float64{4.4, 5.5, 6.6},
		IsMoving:    true,
	}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"lengths_mm":   []interface{}{4.4, 5.5, 6.6},
			"positions_mm": []interface{}{1.1, 2.2, 3.3},
			"is_moving":    true,
		},
	)

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	status := &pb.Status{
		PositionsMm: []float64{1.1, 2.2, 3.3},
		LengthsMm:   []float64{4.4, 5.5, 6.6},
		IsMoving:    true,
	}

	injectGantry := &inject.Gantry{}
	injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return status.PositionsMm, nil
	}
	injectGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return status.LengthsMm, nil
	}
	injectGantry.IsMovingFunc = func(context.Context) (bool, error) {
		return true, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceAPI, ok, err := resource.LookupAPIRegistration[gantry.Gantry](gantry.API)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceAPI.Status(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("not moving", func(t *testing.T) {
		injectGantry.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{
			PositionsMm: []float64{1.1, 2.2, 3.3},
			LengthsMm:   []float64{4.4, 5.5, 6.6},
			IsMoving:    false,
		}
		status1, err := gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status2)
	})

	t.Run("fail on Lengths", func(t *testing.T) {
		errFail := errors.New("can't get lengths")
		injectGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return nil, errFail
		}
		_, err := gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on Positions", func(t *testing.T) {
		errFail := errors.New("can't get positions")
		injectGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
			return nil, errFail
		}
		_, err := gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}
