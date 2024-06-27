package encoder_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestFromRobot(t *testing.T) {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{
		encoder.Named("e1"): inject.NewEncoder("e1"),
		generic.Named("g"):  inject.NewGenericComponent("g"),
	}
	r.MockResourcesFromMap(rs)

	expected := []string{"e1"}
	testutils.VerifySameElements(t, encoder.NamesFromRobot(r), expected)

	_, err := encoder.FromRobot(r, "e1")
	test.That(t, err, test.ShouldBeNil)

	_, err = encoder.FromRobot(r, "e0")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = encoder.FromRobot(r, "g")
	test.That(t, err, test.ShouldNotBeNil)
}
