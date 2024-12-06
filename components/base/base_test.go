package base_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBaseName = "base1"
	failBaseName = "base2"
)

func TestFromRobot(t *testing.T) {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{
		base.Named("base1"): inject.NewBase("base1"),
		base.Named("base2"): inject.NewBase("base2"),
		generic.Named("g"):  inject.NewGenericComponent("g"),
	}
	r.MockResourcesFromMap(rs)

	expected := []string{"base1", "base2"}
	testutils.VerifySameElements(t, base.NamesFromRobot(r), expected)

	_, err := base.FromRobot(r, "base1")
	test.That(t, err, test.ShouldBeNil)

	_, err = base.FromRobot(r, "base2")
	test.That(t, err, test.ShouldBeNil)

	_, err = base.FromRobot(r, "base0")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = base.FromRobot(r, "g")
	test.That(t, err, test.ShouldNotBeNil)
}
