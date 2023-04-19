package resource

import (
	"testing"

	"go.viam.com/test"
)

func TestDependencyTypeError(t *testing.T) {
	name := NewName("foo", "bar", "baz", "bark")
	test.That(t,
		DependencyTypeError[someRes1](name, someRes2{}).Error(),
		test.ShouldContainSubstring,
		`dependency "foo:bar:baz/bark" should be an implementation of resource.someRes1 but it was a resource.someRes2`,
	)

	test.That(t,
		DependencyTypeError[someIfc](name, someRes2{}).Error(),
		test.ShouldContainSubstring,
		`dependency "foo:bar:baz/bark" should be an implementation of resource.someIfc but it was a resource.someRes2`,
	)
}

func TestTypeError(t *testing.T) {
	test.That(t,
		TypeError[someRes1](someRes2{}).Error(),
		test.ShouldContainSubstring,
		"expected implementation of resource.someRes1 but it was a resource.someRes2",
	)

	test.That(t,
		TypeError[someIfc](someRes2{}).Error(),
		test.ShouldContainSubstring,
		"expected implementation of resource.someIfc but it was a resource.someRes2",
	)
}

type someIfc Resource

type someRes1 struct {
	Named
	TriviallyReconfigurable
	TriviallyCloseable
}

type someRes2 struct {
	Named
	TriviallyReconfigurable
	TriviallyCloseable
}
