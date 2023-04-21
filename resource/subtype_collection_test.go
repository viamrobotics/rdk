package resource_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
)

func TestSubtypeService(t *testing.T) {
	res1 := testutils.NewUnimplementedResource(generic.Named("name1"))
	res2 := testutils.NewUnimplementedResource(generic.Named("name2"))
	resources := map[resource.Name]resource.Resource{
		res1.Name(): res1,
	}
	svc, err := resource.NewSubtypeCollection(generic.Subtype, resources)
	test.That(t, err, test.ShouldBeNil)
	res, err := svc.Resource(res1.Name().ShortName())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res1)
	_, err = svc.Resource(res2.Name().ShortName())
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(res2.Name()))

	resources[res2.Name()] = res2
	err = svc.ReplaceAll(resources)
	test.That(t, err, test.ShouldBeNil)
	res, err = svc.Resource(res1.Name().ShortName())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res1)
	res, err = svc.Resource(res2.Name().ShortName())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res2)

	err = svc.ReplaceAll(map[resource.Name]resource.Resource{})
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.Resource(res1.Name().ShortName())
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(res1.Name()))
	_, err = svc.Resource(res2.Name().ShortName())
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(res2.Name()))
	// Test should error if resource name is empty
	resources = map[resource.Name]resource.Resource{
		resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			"foo",
			"",
		): testutils.NewUnimplementedResource(generic.Named("")),
	}
	err = svc.ReplaceAll(resources)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "empty name used for resource:")
}

func TestSubtypeRemoteNames(t *testing.T) {
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

	res0 := testutils.NewUnimplementedResource(generic.Named(name0))
	res1 := testutils.NewUnimplementedResource(generic.Named(name1))
	res2 := testutils.NewUnimplementedResource(generic.Named(name2))
	res3 := testutils.NewUnimplementedResource(generic.Named(name3))
	res4 := testutils.NewUnimplementedResource(generic.Named(name4))
	res5 := testutils.NewUnimplementedResource(generic.Named(name5))
	res7 := testutils.NewUnimplementedResource(generic.Named(name7))
	res8 := testutils.NewUnimplementedResource(generic.Named(name8))
	res9 := testutils.NewUnimplementedResource(generic.Named(name9))
	res10 := testutils.NewUnimplementedResource(generic.Named(name10))

	resources := map[resource.Name]resource.Resource{
		generic.Named(name0):  res0,
		generic.Named(name1):  res1,
		generic.Named(name2):  res2,
		generic.Named(name3):  res3,
		generic.Named(name4):  res4,
		generic.Named(name5):  res5,
		generic.Named(name7):  res7,
		generic.Named(name8):  res8,
		generic.Named(name9):  res9,
		generic.Named(name10): res10,
	}
	svc, err := resource.NewSubtypeCollection(generic.Subtype, resources)
	test.That(t, err, test.ShouldBeNil)
	res, err := svc.Resource(name0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res0)
	res, err = svc.Resource(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res1)
	res, err = svc.Resource(name2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res2)
	res, err = svc.Resource(name3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res3)
	res, err = svc.Resource(name4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res4)
	res, err = svc.Resource(name5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res5)
	res, err = svc.Resource(name7)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res7)
	res, err = svc.Resource(name8)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res8)
	res, err = svc.Resource(name9)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res9)
	res, err = svc.Resource(name10)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res10)

	res, err = svc.Resource("name2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res2)
	_, err = svc.Resource("remote1:name2")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("remote1:name2")))
	res, err = svc.Resource("remote2:name2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res2)
	_, err = svc.Resource("name1")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("name1")))
	res, err = svc.Resource("remote1:name1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res1)
	res, err = svc.Resource("name4")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res4)
	_, err = svc.Resource("remote1:name3")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("remote1:name3")))
	_, err = svc.Resource("remote2:name3")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("remote2:name3")))
	_, err = svc.Resource("name5")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("name5")))
	_, err = svc.Resource("name6")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("name6")))
	res, err = svc.Resource("name5name6")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res5)

	res, err = svc.Resource("name0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, res0)
	_, err = svc.Resource("nameX")
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("nameX")))
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

	svc, err := resource.NewSubtypeCollection(generic.Subtype, map[resource.Name]resource.Resource{
		key1: testutils.NewUnimplementedResource(generic.Named(str1)),
		key4: testutils.NewUnimplementedResource(generic.Named(str4)),
	})
	test.That(t, err, test.ShouldBeNil)
	res, err := svc.Resource(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str1)))
	_, err = svc.Resource(name2)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named(name2)))
	_, err = svc.Resource(name3)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named(name3)))
	res, err = svc.Resource(name4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str4)))
	res, err = svc.Resource(name4s)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str4)))

	test.That(t, svc.Add(key2, testutils.NewUnimplementedResource(generic.Named(str2))), test.ShouldBeNil)
	res, err = svc.Resource(name2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str2)))

	test.That(t, svc.Add(key3, testutils.NewUnimplementedResource(generic.Named(str3))), test.ShouldBeNil)
	res, err = svc.Resource(name3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str3)))

	test.That(t, svc.ReplaceOne(key2, testutils.NewUnimplementedResource(generic.Named(strR))), test.ShouldBeNil)
	res, err = svc.Resource(name2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(strR)))

	test.That(t, svc.ReplaceOne(key4, testutils.NewUnimplementedResource(generic.Named(strR))), test.ShouldBeNil)
	res, err = svc.Resource(name4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(strR)))
	res, err = svc.Resource(name4s)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(strR)))

	test.That(t, svc.Remove(key3), test.ShouldBeNil)
	_, err = svc.Resource(name3)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named(name3)))

	test.That(t, svc.Add(key4d, testutils.NewUnimplementedResource(generic.Named(str4))), test.ShouldBeNil)
	res, err = svc.Resource(name4d)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(str4)))
	_, err = svc.Resource(name4s)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named(name4s)))

	test.That(t, svc.Remove(key4d), test.ShouldBeNil)
	_, err = svc.Resource(name4d)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named(name4d)))
	res, err = svc.Resource(name4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(strR)))
	res, err = svc.Resource(name4s)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldResemble, testutils.NewUnimplementedResource(generic.Named(strR)))
}
