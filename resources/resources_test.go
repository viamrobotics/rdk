package resources_test

import (
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/core/resources"
	"go.viam.com/core/testutils/inject"
)

func TestResourceValidate(t *testing.T) {
	for _, tc := range []struct {
		Name        string
		NewResource resources.ResourceName
		Err         string
	}{
		{
			"missing uuid",
			resources.ResourceName{
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"invalid uuid",
			resources.ResourceName{
				UUID:      "abcd",
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"uuid field for resource missing or invalid",
		},
		{
			"missing namespace",
			resources.ResourceName{
				UUID:    uuid.NewString(),
				Type:    resources.ResourceTypeComponent,
				Subtype: "arm",
				Name:    "arm1",
			},
			"namespace field for resource missing or invalid",
		},
		{
			"missing type",
			resources.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"type field for resource missing or invalid",
		},
		{
			"missing subtype",
			resources.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Name:      "arm1",
			},
			"subtype field for resource missing or invalid",
		},
		{
			"missing name",
			resources.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
			},
			"",
		},
		{
			"all fields included",
			resources.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resources.ResourceNamespaceCore,
				Type:      resources.ResourceTypeComponent,
				Subtype:   "arm",
				Name:      "arm1",
			},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := tc.NewResource.Validate()
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestResourcesPopulate(t *testing.T) {
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

	r := resources.New()
	err := r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.GetResources()), test.ShouldEqual, 1)
	test.That(t, r.GetResources()[0].Subtype, test.ShouldResemble, resources.ResourceSubtypeMetadata)

	injectRobot.ArmNamesFunc = func() []string {
		return []string{"arm1"}
	}
	injectRobot.BaseNamesFunc = func() []string {
		return []string{"base1"}
	}
	err = r.ClearResources()
	test.That(t, err, test.ShouldBeNil)
	err = r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.GetResources()), test.ShouldEqual, 3)
	test.That(t, r.GetResources()[1].Name, test.ShouldEqual, "arm1")
	test.That(t, r.GetResources()[2].Name, test.ShouldEqual, "base1")

	armUUID := r.GetResources()[1].UUID
	baseUUID := r.GetResources()[2].UUID
	err = r.ClearResources()
	test.That(t, err, test.ShouldBeNil)
	err = r.Populate(injectRobot)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(r.GetResources()), test.ShouldEqual, 3)
	test.That(t, r.GetResources()[1].Name, test.ShouldEqual, "arm1")
	test.That(t, r.GetResources()[1].UUID, test.ShouldNotEqual, armUUID)
	test.That(t, r.GetResources()[2].Name, test.ShouldEqual, "base1")
	test.That(t, r.GetResources()[2].UUID, test.ShouldNotEqual, baseUUID)
}

func TestAddResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	newMetadata := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeService,
		Subtype:   resources.ResourceSubtypeMetadata,
	}

	for _, tc := range []struct {
		Name        string
		NewResource resources.ResourceName
		Expected    []resources.ResourceName
		Err         string
	}{
		{
			"invalid addition",
			resources.ResourceName{},
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
			[]resources.ResourceName{metadata, arm},
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
			[]resources.ResourceName{metadata, arm, sensor},
			"",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			err := r.AddResource(tc.NewResource)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.GetResources(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestRemoveResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	r.AddResource(arm)
	r.AddResource(sensor)

	for _, tc := range []struct {
		Name           string
		RemoveResource resources.ResourceName
		Expected       []resources.ResourceName
		Err            string
	}{
		{
			"invalid removal",
			resources.ResourceName{},
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
			[]resources.ResourceName{metadata, arm},
			"",
		},
		{
			"second removal",
			arm,
			[]resources.ResourceName{metadata},
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
			err := r.RemoveResource(tc.RemoveResource)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r.GetResources(), test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}

func TestClearResource(t *testing.T) {
	r := resources.New()

	metadata := r.GetResources()[0]
	arm := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "arm",
		Name:      "arm1",
	}
	sensor := resources.ResourceName{
		UUID:      uuid.NewString(),
		Namespace: resources.ResourceNamespaceCore,
		Type:      resources.ResourceTypeComponent,
		Subtype:   "sensor",
		Name:      "sensor1",
	}

	r.AddResource(arm)
	r.AddResource(sensor)

	err := r.ClearResources()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.GetResources(), test.ShouldResemble, []resources.ResourceName{metadata})
}
