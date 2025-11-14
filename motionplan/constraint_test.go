package motionplan

import (
	"testing"

	"go.viam.com/test"
)

func TestConstraintConstructors(t *testing.T) {
	c := NewEmptyConstraints()

	desiredLinearTolerance := float64(1000.0)
	desiredOrientationTolerance := float64(0.0)

	c.AddLinearConstraint(LinearConstraint{
		LineToleranceMm:          desiredLinearTolerance,
		OrientationToleranceDegs: desiredOrientationTolerance,
	})

	test.That(t, len(c.LinearConstraint), test.ShouldEqual, 1)
	test.That(t, c.LinearConstraint[0].LineToleranceMm, test.ShouldEqual, desiredLinearTolerance)
	test.That(t, c.LinearConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddOrientationConstraint(OrientationConstraint{
		OrientationToleranceDegs: desiredOrientationTolerance,
	})
	test.That(t, len(c.OrientationConstraint), test.ShouldEqual, 1)
	test.That(t, c.OrientationConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddCollisionSpecification(CollisionSpecification{
		Allows: []CollisionSpecificationAllowedFrameCollisions{
			{
				Frame1: "frame1",
				Frame2: "frame2",
			},
			{
				Frame1: "frame3",
				Frame2: "frame4",
			},
		},
	})
	test.That(t, len(c.CollisionSpecification), test.ShouldEqual, 1)
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame1, test.ShouldEqual, "frame1")
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame2, test.ShouldEqual, "frame2")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame1, test.ShouldEqual, "frame3")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame2, test.ShouldEqual, "frame4")

	pbConstraint := c.ToProtobuf()
	pbToRDKConstraint := ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)

	c.AddPseudolinearConstraint(PseudolinearConstraint{5, 2})

	pbConstraint = c.ToProtobuf()
	pbToRDKConstraint = ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)
}

func TestOrientationConstraintHelpers(t *testing.T) {
	test.That(t, between(1, 5, 3), test.ShouldBeTrue)
	test.That(t, between(1, 5, 0), test.ShouldBeFalse)
	test.That(t, between(1, 5, 6), test.ShouldBeFalse)
	test.That(t, between(5, 1, 3), test.ShouldBeTrue)
	test.That(t, between(5, 1, 0), test.ShouldBeFalse)
	test.That(t, between(5, 1, 6), test.ShouldBeFalse)
}
