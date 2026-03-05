package referenceframe

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
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

func TestLinearInputsLimits(t *testing.T) {
	// Set up a frame system with three "top-level" frames. A 0dof static frame, a 1dof rotational
	// frame and a 2dof simple model. We envision the 2dof simple model as an arm with 2 joints and
	// 3 links.
	fs := NewEmptyFrameSystem("fs")
	err := fs.AddFrame(NewZeroStaticFrame("0dof"), fs.World())
	test.That(t, err, test.ShouldBeNil)

	rotFrame, err := NewRotationalFrame("1dof", spatial.R4AA{RX: 1, RY: 0, RZ: 0}, Limit{-10, 10})
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(rotFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	baseArmFrame := NewZeroStaticFrame("base")
	shoulderArmFrame, err := NewRotationalFrame("shoulder", spatial.R4AA{RX: 1, RY: 0, RZ: 0}, Limit{-10, 10})
	test.That(t, err, test.ShouldBeNil)
	upperArmFrame := NewZeroStaticFrame("upperArm")
	elbowArmFrame, err := NewRotationalFrame("elbow", spatial.R4AA{RX: 1, RY: 0, RZ: 0}, Limit{-10, 10})
	test.That(t, err, test.ShouldBeNil)
	handArmFrame := NewZeroStaticFrame("hand")
	armFrame, err := NewSerialModel("arm", []Frame{
		baseArmFrame, shoulderArmFrame, upperArmFrame, elbowArmFrame, handArmFrame,
	})
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(armFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// Create inputs, do a transform call that succeeds.
	li := NewLinearInputs()
	li.Put("arm", []Input{-1, -2})

	dq, err := fs.TransformToDQ(li, "arm", "world")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fmt.Sprintf("%v", spatial.Pose(&dq)), test.ShouldResemble,
		"{X:0.000000 Y:0.000000 Z:0.000000 OX:0.000000 OY:0.141120 OZ:-0.989992 Theta:-90.000000°}")

	// Change inputs to be out of bounds. Assert transforming fails.
	li.Put("arm", []Input{-15, 5})
	dq, err = fs.TransformToDQ(li, "arm", "world")
	test.That(t, err, test.ShouldNotBeNil)

	// Testing the internal state. We have not called `GetSchema`, hence we expect the limits to be
	// all nil.
	for _, meta := range li.schema.metas {
		test.That(t, meta.frame, test.ShouldBeNil)
	}

	// Getting the schema will apply frame limits to the underlying metas.
	schema, err := li.GetSchema(fs)
	test.That(t, err, test.ShouldBeNil)

	// Walk all of the underlying meta objects test the internal state has changed.
	for _, meta := range schema.metas {
		// Assert that limits is no longer nil.
		test.That(t, meta.frame, test.ShouldNotBeNil)

		// We've constructed every limit to be in the range of [-10, 10]. Assert that we see that
		// here.
		//
		// Dan: Perhaps this is a bit too simple and missing an obvious bug that a more intentional
		// test would catch.
		for _, limit := range meta.frame.DoF() {
			test.That(t, limit.Min, test.ShouldEqual, -10)
			test.That(t, limit.Max, test.ShouldEqual, 10)
		}
	}
}
