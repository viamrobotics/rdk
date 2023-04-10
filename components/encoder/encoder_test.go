package encoder_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testEncoderName    = "encoder1"
	testEncoderName2   = "encoder2"
	failEncoderName    = "encoder3"
	fakeEncoderName    = "encoder4"
	missingEncoderName = "encoder5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[encoder.Named(testEncoderName)] = &mock{Name: testEncoderName}
	deps[encoder.Named(fakeEncoderName)] = "not a encoder"
	return deps
}

func setupInjectRobot() *inject.Robot {
	encoder1 := &mock{Name: testEncoderName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case encoder.Named(testEncoderName):
			return encoder1, nil
		case encoder.Named(fakeEncoderName):
			return "not a encoder", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{encoder.Named(testEncoderName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	m, err := encoder.FromRobot(r, testEncoderName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := m.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := encoder.FromDependencies(deps, testEncoderName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, _, err := res.GetPosition(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)

	res, err = encoder.FromDependencies(deps, fakeEncoderName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyTypeError[encoder.Encoder](fakeEncoderName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = encoder.FromDependencies(deps, missingEncoderName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingEncoderName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := encoder.FromRobot(r, testEncoderName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, _, err := res.GetPosition(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)

	res, err = encoder.FromRobot(r, fakeEncoderName)
	test.That(t, err, test.ShouldBeError, encoder.NewUnimplementedInterfaceError("string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = encoder.FromRobot(r, missingEncoderName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(encoder.Named(missingEncoderName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := encoder.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testEncoderName})
}

func TestEncoderName(t *testing.T) {
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
					ResourceSubtype: encoder.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testEncoderName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: encoder.SubtypeName,
				},
				Name: testEncoderName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := encoder.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualEncoder1 encoder.Encoder = &mock{Name: testEncoderName}
	reconfEncoder1, err := encoder.WrapWithReconfigurable(actualEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = encoder.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, encoder.NewUnimplementedInterfaceError(nil))

	reconfEncoder2, err := encoder.WrapWithReconfigurable(reconfEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfEncoder2, test.ShouldEqual, reconfEncoder1)

	var actualEncoder2 encoder.Encoder = &mockLocal{Name: testEncoderName}
	reconfEncoder3, err := encoder.WrapWithReconfigurable(actualEncoder2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	reconfEncoder4, err := encoder.WrapWithReconfigurable(reconfEncoder3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfEncoder4, test.ShouldResemble, reconfEncoder3)

	_, ok := reconfEncoder4.(encoder.Encoder)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestReconfigurableEncoder(t *testing.T) {
	actualEncoder1 := &mock{Name: testEncoderName}
	reconfEncoder1, err := encoder.WrapWithReconfigurable(actualEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualEncoder2 := &mock{Name: testEncoderName2}
	reconfEncoder2, err := encoder.WrapWithReconfigurable(actualEncoder2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualEncoder1.reconfCount, test.ShouldEqual, 0)

	err = reconfEncoder1.Reconfigure(context.Background(), reconfEncoder2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfEncoder1, test.ShouldResemble, reconfEncoder2)
	test.That(t, actualEncoder1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualEncoder1.posCount, test.ShouldEqual, 0)
	test.That(t, actualEncoder2.posCount, test.ShouldEqual, 0)
	result, _, err := reconfEncoder1.(encoder.Encoder).GetPosition(
		context.Background(),
		encoder.PositionTypeUNSPECIFIED.Enum(),
		nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)
	test.That(t, actualEncoder1.posCount, test.ShouldEqual, 0)
	test.That(t, actualEncoder2.posCount, test.ShouldEqual, 1)

	err = reconfEncoder1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfEncoder1, nil))

	actualEncoder3 := &mock{Name: failEncoderName}
	reconfEncoder3, err := encoder.WrapWithReconfigurable(actualEncoder3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfEncoder3, test.ShouldNotBeNil)
}

func TestResetPosition(t *testing.T) {
	actualEncoder1 := &mock{Name: testEncoderName}
	reconfEncoder1, err := encoder.WrapWithReconfigurable(actualEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualEncoder1.zeroCount, test.ShouldEqual, 0)
	err = reconfEncoder1.(encoder.Encoder).ResetPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualEncoder1.zeroCount, test.ShouldEqual, 1)
}

func TestGetPosition(t *testing.T) {
	actualEncoder1 := &mock{Name: testEncoderName}
	reconfEncoder1, err := encoder.WrapWithReconfigurable(actualEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualEncoder1.posCount, test.ShouldEqual, 0)
	pos1, positionType, err := reconfEncoder1.(encoder.Encoder).GetPosition(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos1, test.ShouldResemble, position)
	test.That(t, actualEncoder1.posCount, test.ShouldEqual, 1)
	test.That(t, positionType, test.ShouldEqual, encoder.PositionTypeUNSPECIFIED)

	props, err := reconfEncoder1.(encoder.Encoder).GetProperties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props[encoder.TicksCountSupported], test.ShouldBeTrue)
	test.That(t, props[encoder.AngleDegreesSupported], test.ShouldBeTrue)
}

func TestClose(t *testing.T) {
	actualEncoder1 := &mock{Name: testEncoderName}
	reconfEncoder1, err := encoder.WrapWithReconfigurable(actualEncoder1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualEncoder1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfEncoder1), test.ShouldBeNil)
	test.That(t, actualEncoder1.reconfCount, test.ShouldEqual, 1)
}

var (
	position = 5.5
	features = map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: true,
	}
)

type mock struct {
	Name string

	zeroCount     int
	posCount      int
	featuresCount int
	reconfCount   int
	extra         map[string]interface{}
}

func (m *mock) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	m.zeroCount++
	return nil
}

func (m *mock) GetPosition(
	ctx context.Context,
	positionType *encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	m.posCount++
	m.extra = extra
	return position, encoder.PositionTypeUNSPECIFIED, nil
}

func (m *mock) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	m.featuresCount++
	m.extra = extra
	return features, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type mockLocal struct {
	mock
	Name string
}
