package gantry_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/sensor"
	pb "go.viam.com/rdk/proto/api/component/gantry/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testGantryName    = "gantry1"
	testGantryName2   = "gantry2"
	failGantryName    = "gantry3"
	fakeGantryName    = "gantry4"
	missingGantryName = "gantry5"
)

func setupInjectRobot() *inject.Robot {
	gantry1 := &mock{Name: testGantryName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case gantry.Named(testGantryName):
			return gantry1, nil
		case gantry.Named(fakeGantryName):
			return "not a gantry", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{gantry.Named(testGantryName), sensor.Named("sensor1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	g, err := gantry.FromRobot(r, testGantryName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := g.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := gantry.FromRobot(r, testGantryName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	lengths1, err := res.GetLengths(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths1, test.ShouldResemble, lengths)

	res, err = gantry.FromRobot(r, fakeGantryName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGantry", "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = gantry.FromRobot(r, missingGantryName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(gantry.Named(missingGantryName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := gantry.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testGantryName})
}

func TestStatusValid(t *testing.T) {
	status := &pb.Status{
		PositionsMm: []float64{1.1, 2.2, 3.3},
		LengthsMm:   []float64{4.4, 5.5, 6.6},
		IsMoving:    true,
	}
	map1, err := protoutils.InterfaceToMap(status)
	test.That(t, err, test.ShouldBeNil)
	newStruct, err := structpb.NewStruct(map1)
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
	_, err := gantry.CreateStatus(context.Background(), "not a gantry")
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGantry", "string"))

	status := &pb.Status{
		PositionsMm: []float64{1.1, 2.2, 3.3},
		LengthsMm:   []float64{4.4, 5.5, 6.6},
		IsMoving:    true,
	}

	injectGantry := &inject.Gantry{}
	injectGantry.GetPositionFunc = func(ctx context.Context) ([]float64, error) {
		return status.PositionsMm, nil
	}
	injectGantry.GetLengthsFunc = func(ctx context.Context) ([]float64, error) {
		return status.LengthsMm, nil
	}
	injectGantry.IsMovingFunc = func() bool {
		return true
	}

	t.Run("working", func(t *testing.T) {
		status1, err := gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})

	t.Run("fail on GetLengths", func(t *testing.T) {
		errFail := errors.New("can't get lengths")
		injectGantry.GetLengthsFunc = func(ctx context.Context) ([]float64, error) {
			return nil, errFail
		}
		_, err = gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on GetPositions", func(t *testing.T) {
		errFail := errors.New("can't get positions")
		injectGantry.GetPositionFunc = func(ctx context.Context) ([]float64, error) {
			return nil, errFail
		}
		_, err = gantry.CreateStatus(context.Background(), injectGantry)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}

func TestGantryName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: gantry.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testGantryName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: gantry.SubtypeName,
				},
				Name: testGantryName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := gantry.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualGantry1 gantry.Gantry = &mock{Name: testGantryName}
	reconfGantry1, err := gantry.WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)

	_, err = gantry.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGantry", nil))
	reconfGantry2, err := gantry.WrapWithReconfigurable(reconfGantry1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGantry2, test.ShouldEqual, reconfGantry1)
}

func TestReconfigurableGantry(t *testing.T) {
	actualGantry1 := &mock{Name: testGantryName}
	reconfGantry1, err := gantry.WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)

	actualGantry2 := &mock{Name: testGantryName2}
	reconfGantry2, err := gantry.WrapWithReconfigurable(actualGantry2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGantry1.reconfCount, test.ShouldEqual, 0)

	err = reconfGantry1.Reconfigure(context.Background(), reconfGantry2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGantry1, test.ShouldResemble, reconfGantry2)
	test.That(t, actualGantry1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualGantry1.lengthsCount, test.ShouldEqual, 0)
	test.That(t, actualGantry2.lengthsCount, test.ShouldEqual, 0)
	lengths1, err := reconfGantry1.(gantry.Gantry).GetLengths(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths1, test.ShouldResemble, lengths)
	test.That(t, actualGantry1.lengthsCount, test.ShouldEqual, 0)
	test.That(t, actualGantry2.lengthsCount, test.ShouldEqual, 1)

	err = reconfGantry1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *gantry.reconfigurableGantry")
}

func TestStop(t *testing.T) {
	actualGantry1 := &mock{Name: testGantryName}
	reconfGantry1, err := gantry.WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualGantry1.stopCount, test.ShouldEqual, 0)
	test.That(t, reconfGantry1.(gantry.Gantry).Stop(context.Background()), test.ShouldBeNil)
	test.That(t, actualGantry1.stopCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualGantry1 := &mock{Name: testGantryName}
	reconfGantry1, err := gantry.WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualGantry1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfGantry1), test.ShouldBeNil)
	test.That(t, actualGantry1.reconfCount, test.ShouldEqual, 1)
}

var lengths = []float64{1.0, 2.0, 3.0}

type mock struct {
	gantry.LocalGantry
	Name         string
	lengthsCount int
	stopCount    int
	reconfCount  int
}

func (m *mock) GetLengths(context.Context) ([]float64, error) {
	m.lengthsCount++
	return lengths, nil
}

func (m *mock) Stop(ctx context.Context) error {
	m.stopCount++
	return nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
