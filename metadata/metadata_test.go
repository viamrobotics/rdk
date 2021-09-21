package metadata_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/metadata"
	"go.viam.com/core/resource"
	"go.viam.com/core/testutils/inject"
)

func TestPopulate(t *testing.T) {
	var emptyNames = func() []string {
		return []string{}
	}
	injectRobot := &inject.Robot{
		ArmNamesFunc:      emptyNames,
		BaseNamesFunc:     emptyNames,
		BoardNamesFunc:    emptyNames,
		CameraNamesFunc:   emptyNames,
		FunctionNamesFunc: emptyNames,
		GripperNamesFunc:  emptyNames,
		LidarNamesFunc:    emptyNames,
		RemoteNamesFunc:   emptyNames,
		SensorNamesFunc:   emptyNames,
	}

	r := metadata.New()
	err := r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.All()), test.ShouldEqual, 1)
	test.That(t, r.All()[0].Subtype, test.ShouldResemble, resource.ResourceSubtypeMetadata)

	injectRobot.ArmNamesFunc = func() []string {
		return []string{"arm1"}
	}
	injectRobot.BaseNamesFunc = func() []string {
		return []string{"base1"}
	}

	r = metadata.New()
	err = r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.All()), test.ShouldEqual, 3)
	test.That(t, r.All()[1].Name, test.ShouldEqual, "arm1")
	test.That(t, r.All()[2].Name, test.ShouldEqual, "base1")

	armUUID := r.All()[1].UUID
	baseUUID := r.All()[2].UUID

	r = metadata.New()
	test.That(t, err, test.ShouldBeNil)
	err = r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.All()), test.ShouldEqual, 3)
	test.That(t, r.All()[1].Name, test.ShouldEqual, "arm1")
	test.That(t, r.All()[1].UUID, test.ShouldNotEqual, armUUID)
	test.That(t, r.All()[2].Name, test.ShouldEqual, "base1")
	test.That(t, r.All()[2].UUID, test.ShouldNotEqual, baseUUID)
}

func TestAdd(t *testing.T) {
	r := metadata.New()

	metadata := r.All()[0]
	arm := resource.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resource.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	newMetadata := resource.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeService,
		Subtype:   resource.ResourceSubtypeMetadata,
	}

	for _, tc := range []struct {
		Name        string
		NewResource resource.ResourceName
		Expected    []resource.ResourceName
		Err         string
	}{
		{
			"invalid addition",
			resource.ResourceName{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"invalid addition 2",
			newMetadata,
			nil,
			"unable to add a resource with a metadata subtype",
		},
		{
			"one addition",
			arm,
			[]resource.ResourceName{metadata, arm},
			"",
		},
		{
			"duplicate addition",
			arm,
			nil,
			"already exists",
		},
		{
			"another addition",
			sensor,
			[]resource.ResourceName{metadata, arm, sensor},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.Add(tc.NewResource)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.All(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	r := metadata.New()

	metadata := r.All()[0]
	arm := resource.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resource.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resource.ResourceNamespaceCore,
		Type:      resource.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	r.Add(arm)
	r.Add(sensor)

	for _, tc := range []struct {
		Name     string
		Remove   resource.ResourceName
		Expected []resource.ResourceName
		Err      string
	}{
		{
			"invalid removal",
			resource.ResourceName{},
			nil,
			"uuid field for resource missing or invalid",
		},
		{
			"invalid metadata removal",
			metadata,
			nil,
			"unable to remove resource with a metadata subtype",
		},
		{
			"one removal",
			sensor,
			[]resource.ResourceName{metadata, arm},
			"",
		},
		{
			"second removal",
			arm,
			[]resource.ResourceName{metadata},
			"",
		},
		{
			"not found",
			sensor,
			nil,
			"unable to find and remove resource",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.Remove(tc.Remove)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.All(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}
