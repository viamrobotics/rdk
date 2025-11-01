package referenceframe

import (
	"testing"

	"go.viam.com/test"
)

func TestLinearInputs(t *testing.T) {
	li := NewLinearInputs()

	// Getting inputs for a frame that does not exist should return nil. For parity with a typical
	// map access.
	test.That(t, li.Get("3dof"), test.ShouldBeNil)

	// Put inputs in for a frame. Assert we can read it back out.
	li.Put("3dof", []Input{1, 2, 3})
	test.That(t, li.Get("3dof"), test.ShouldResemble, []Input{1, 2, 3})

	// Overwrite existing inputs. Assert we can read it back out.
	li.Put("3dof", []Input{4, 5, 6})
	test.That(t, li.Get("3dof"), test.ShouldResemble, []Input{4, 5, 6})

	// Try overwriting existing inputs with new inputs that have a different size. Silently discard
	// this `Put`. This is not essential behavior. But allowing for it would require the
	// implementation to appropriately resize + reposition its underlying array. And we don't expect
	// this to be a realistic usecase. Callers ought to create a fresh LinearInputs when degrees of
	// freedom change.
	li.Put("3dof", []Input{1, 2, 3, 4})
	test.That(t, li.Get("3dof"), test.ShouldResemble, []Input{4, 5, 6})

	// Add a second frame.
	li.Put("2dof", []Input{1, 2})
	test.That(t, li.Get("2dof"), test.ShouldResemble, []Input{1, 2})
	test.That(t, li.Get("3dof"), test.ShouldResemble, []Input{4, 5, 6})

	// Change the values of the first frame.
	li.Put("3dof", []Input{1, 2, 3})
	test.That(t, li.Get("2dof"), test.ShouldResemble, []Input{1, 2})
	test.That(t, li.Get("3dof"), test.ShouldResemble, []Input{1, 2, 3})

	// Assert that 0 DoF frames can be inserted. And assert that, when getting a 0 DoF frame back
	// out, we return an empty slice rather than nil.
	test.That(t, li.Get("0dof"), test.ShouldBeNil)
	li.Put("0dof", []Input{})
	test.That(t, li.Get("0dof"), test.ShouldResemble, []Input{})
}
