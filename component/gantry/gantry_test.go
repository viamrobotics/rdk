package gantry_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/sensor"
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
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case gantry.Named(testGantryName):
			return gantry1, true
		case gantry.Named(fakeGantryName):
			return "not a gantry", true
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{gantry.Named(testGantryName), sensor.Named("sensor1")}
	}
	return r
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
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Gantry", "string"))
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
				UUID: "c7e7e1a5-2d0c-5665-af81-0f821bb94793",
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
				UUID: "4f1dd722-b371-59e9-9e66-701823f025b7",
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
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Gantry", nil))
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
	gantry.Gantry
	Name         string
	lengthsCount int
	reconfCount  int
}

func (m *mock) GetLengths(context.Context) ([]float64, error) {
	m.lengthsCount++
	return lengths, nil
}

func (m *mock) Close() { m.reconfCount++ }
