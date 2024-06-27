package base_test

import (
	"context"
	"testing"

	"github.com/go-viper/mapstructure/v2"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

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

func TestStatusValid(t *testing.T) {
	status := &commonpb.ActuatorStatus{
		IsMoving: true,
	}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"is_moving": true,
		},
	)

	convMap := &commonpb.ActuatorStatus{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	t.Run("is moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: true,
		}

		injectBase := &inject.Base{}
		injectBase.IsMovingFunc = func(context.Context) (bool, error) {
			return true, nil
		}
		status1, err := base.CreateStatus(context.Background(), injectBase)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceAPI, ok, err := resource.LookupAPIRegistration[base.Base](base.API)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceAPI.Status(context.Background(), injectBase)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("is not moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: false,
		}

		injectBase := &inject.Base{}
		injectBase.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}
		status1, err := base.CreateStatus(context.Background(), injectBase)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})
}

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
