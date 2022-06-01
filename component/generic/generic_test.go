package generic_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testGenericName    = "generic1"
	testGenericName2   = "generic2"
	failGenericName    = "generic3"
	fakeGenericName    = "generic4"
	missingGenericName = "generic5"
)

func setupInjectRobot() *inject.Robot {
	generic1 := &mock{Name: testGenericName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case generic.Named(testGenericName):
			return generic1, nil
		case generic.Named(fakeGenericName):
			return "not a generic", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{generic.Named(testGenericName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := generic.FromRobot(r, testGenericName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.Do(context.Background(), generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, generic.TestCommand)

	s, err = generic.FromRobot(r, fakeGenericName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Generic", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = generic.FromRobot(r, missingGenericName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(generic.Named(missingGenericName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := generic.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testGenericName})
}

func TestGenericName(t *testing.T) {
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
					ResourceSubtype: generic.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testGenericName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: generic.SubtypeName,
				},
				Name: testGenericName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := generic.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualGeneric1 generic.Generic = &mock{Name: testGenericName}
	reconfGeneric1, err := generic.WrapWithReconfigurable(actualGeneric1)
	test.That(t, err, test.ShouldBeNil)

	_, err = generic.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Generic", nil))

	reconfGeneric2, err := generic.WrapWithReconfigurable(reconfGeneric1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGeneric2, test.ShouldEqual, reconfGeneric1)
}

func TestReconfigurableGeneric(t *testing.T) {
	actualGeneric1 := &mock{Name: testGenericName}
	reconfGeneric1, err := generic.WrapWithReconfigurable(actualGeneric1)
	test.That(t, err, test.ShouldBeNil)

	actualGeneric2 := &mock{Name: testGenericName2}
	reconfGeneric2, err := generic.WrapWithReconfigurable(actualGeneric2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGeneric1.reconfCount, test.ShouldEqual, 0)

	err = reconfGeneric1.Reconfigure(context.Background(), reconfGeneric2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGeneric1, test.ShouldResemble, reconfGeneric2)
	test.That(t, actualGeneric1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualGeneric1.doCount, test.ShouldEqual, 0)
	test.That(t, actualGeneric2.doCount, test.ShouldEqual, 0)
	result, err := reconfGeneric1.(generic.Generic).Do(context.Background(), generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, generic.TestCommand)
	test.That(t, actualGeneric1.doCount, test.ShouldEqual, 0)
	test.That(t, actualGeneric2.doCount, test.ShouldEqual, 1)

	err = reconfGeneric1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *generic.reconfigurableGeneric")
}

func TestClose(t *testing.T) {
	actualGeneric1 := &mock{Name: testGenericName}
	reconfGeneric1, err := generic.WrapWithReconfigurable(actualGeneric1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualGeneric1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfGeneric1), test.ShouldBeNil)
	test.That(t, actualGeneric1.reconfCount, test.ShouldEqual, 1)
}

type mock struct {
	Name string

	doCount    int
	reconfCount int
}

func (mGeneric *mock) Close() { mGeneric.reconfCount++ }

func (mGeneric *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	mGeneric.doCount++
	return cmd, nil
}
