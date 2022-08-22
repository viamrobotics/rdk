package subtype_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
)

func TestSubtypeService(t *testing.T) {
	strType := resource.SubtypeName("string")
	name1 := "name1"
	name2 := "name2"
	resources := map[resource.Name]interface{}{
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name1,
		): name1,
	}
	svc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldEqual, name1)
	test.That(t, svc.Resource(name2), test.ShouldBeNil)

	rName2 := resource.NewName(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeComponent,
		strType,
		name2,
	)
	resources[rName2] = name2
	err = svc.Replace(resources)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldEqual, name1)
	test.That(t, svc.Resource(name2), test.ShouldEqual, name2)

	err = svc.Replace(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldBeNil)
	test.That(t, svc.Resource(name2), test.ShouldBeNil)
}

func TestSubtypeRemoteNames(t *testing.T) {
	strType := resource.SubtypeName("string")
	name0 := "name0"
	name1 := "remote1:name1"
	name2 := "remote2:name2"
	name3 := "remote2:remote1:name1"
	name4 := "remote2:remote1:name4"
	name5 := "remote2:remote1:name5name6"
	name7 := "remote2:remote4:name1"
	resources := map[resource.Name]interface{}{
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name0,
		): name0,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name1,
		): name1,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name2,
		): name2,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name3,
		): name3,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name4,
		): name4,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name5,
		): name5,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name7,
		): name7,
	}
	svc, err := subtype.New(resources)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name0), test.ShouldEqual, name0)
	test.That(t, svc.Resource(name1), test.ShouldEqual, name1)
	test.That(t, svc.Resource(name2), test.ShouldEqual, name2)
	test.That(t, svc.Resource(name3), test.ShouldEqual, name3)
	test.That(t, svc.Resource(name4), test.ShouldEqual, name4)
	test.That(t, svc.Resource(name5), test.ShouldEqual, name5)
	test.That(t, svc.Resource(name7), test.ShouldEqual, name7)

	test.That(t, svc.Resource("name2"), test.ShouldEqual, name2)
	test.That(t, svc.Resource("remote1:name2"), test.ShouldBeNil)
	test.That(t, svc.Resource("remote2:name2"), test.ShouldEqual, name2)
	test.That(t, svc.Resource("name1"), test.ShouldBeNil)
	test.That(t, svc.Resource("remote1:name1"), test.ShouldEqual, name1)
	test.That(t, svc.Resource("name4"), test.ShouldEqual, name4)
	test.That(t, svc.Resource("remote1:name3"), test.ShouldBeNil)
	test.That(t, svc.Resource("remote1:name3"), test.ShouldBeNil)
	test.That(t, svc.Resource("name5"), test.ShouldBeNil)
	test.That(t, svc.Resource("name6"), test.ShouldBeNil)
	test.That(t, svc.Resource("name5name6"), test.ShouldEqual, name5)
}
