package base_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBaseName    = "base1"
	testBaseName2   = "base2"
	failBaseName    = "base3"
	fakeBaseName    = "base4"
	missingBaseName = "base5"
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
	_, err := base.CreateStatus(context.Background(), testutils.NewUnimplementedResource(generic.Named("foo")))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation")

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

		resourceSubtype, ok := registry.ResourceSubtypeLookup(base.Subtype)
		test.That(t, ok, test.ShouldBeTrue)
		status2, err := resourceSubtype.Status(context.Background(), injectBase)
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
