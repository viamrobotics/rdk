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
