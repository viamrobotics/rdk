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
	err = svc.ReplaceAll(resources)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldEqual, name1)
	test.That(t, svc.Resource(name2), test.ShouldEqual, name2)

	err = svc.ReplaceAll(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldBeNil)
	test.That(t, svc.Resource(name2), test.ShouldBeNil)
	// Test should error if resource name is empty
	resources = map[resource.Name]interface{}{
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			"",
		): name1,
	}
	err = svc.ReplaceAll(resources)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "empty name used for resource:")
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
	name8 := "remote1:name0"
	name9 := "remote1:nameX"
	name10 := "remote2:nameX"
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
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name8,
		): name8,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name9,
		): name9,
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			strType,
			name10,
		): name10,
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
	test.That(t, svc.Resource(name8), test.ShouldEqual, name8)
	test.That(t, svc.Resource(name9), test.ShouldEqual, name9)
	test.That(t, svc.Resource(name10), test.ShouldEqual, name10)

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

	test.That(t, svc.Resource("name0"), test.ShouldEqual, name0)
	test.That(t, svc.Resource("nameX"), test.ShouldBeNil)
}

func TestSubtypeAddRemoveReplaceOne(t *testing.T) {
	ns := resource.Namespace("acme")
	ct := resource.TypeName("test")
	st := resource.SubtypeName("string")

	name1 := "name1"
	name2 := "name2"
	name3 := "name3"
	name4 := "remote1:name4"
	name4d := "remote2:name4"
	name4s := "name4"

	str1 := "string1"
	str2 := "string2"
	str3 := "string3"
	str4 := "string4"
	strR := "stringReplaced"

	key1 := resource.NewName(ns, ct, st, name1)
	key2 := resource.NewName(ns, ct, st, name2)
	key3 := resource.NewName(ns, ct, st, name3)
	key4 := resource.NewName(ns, ct, st, name4)
	key4d := resource.NewName(ns, ct, st, name4d)

	svc, err := subtype.New(map[resource.Name]interface{}{key1: str1, key4: str4})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Resource(name1), test.ShouldEqual, str1)
	test.That(t, svc.Resource(name2), test.ShouldBeNil)
	test.That(t, svc.Resource(name3), test.ShouldBeNil)
	test.That(t, svc.Resource(name4), test.ShouldEqual, str4)
	test.That(t, svc.Resource(name4s), test.ShouldEqual, str4)

	test.That(t, svc.Add(key2, str2), test.ShouldBeNil)
	test.That(t, svc.Resource(name2), test.ShouldEqual, str2)

	test.That(t, svc.Add(key3, str3), test.ShouldBeNil)
	test.That(t, svc.Resource(name3), test.ShouldEqual, str3)

	test.That(t, svc.ReplaceOne(key2, strR), test.ShouldBeNil)
	test.That(t, svc.Resource(name2), test.ShouldEqual, strR)

	test.That(t, svc.ReplaceOne(key4, strR), test.ShouldBeNil)
	test.That(t, svc.Resource(name4), test.ShouldEqual, strR)
	test.That(t, svc.Resource(name4s), test.ShouldEqual, strR)

	test.That(t, svc.Remove(key3), test.ShouldBeNil)
	test.That(t, svc.Resource(name3), test.ShouldBeNil)

	test.That(t, svc.Add(key4d, str4), test.ShouldBeNil)
	test.That(t, svc.Resource(name4d), test.ShouldEqual, str4)
	test.That(t, svc.Resource(name4s), test.ShouldBeNil)

	test.That(t, svc.Remove(key4d), test.ShouldBeNil)
	test.That(t, svc.Resource(name4d), test.ShouldBeNil)
	test.That(t, svc.Resource(name4), test.ShouldEqual, strR)
	test.That(t, svc.Resource(name4s), test.ShouldEqual, strR)
}
