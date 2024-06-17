package classification

import (
	"testing"

	"go.viam.com/test"
)

func TestTopNClassifications(t *testing.T) {
	cls := Classifications{
		NewClassification(0.2, "a"),
		NewClassification(0.4, "b"),
		NewClassification(0.1, "c"),
	}
	top1, err := cls.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(top1), test.ShouldEqual, 1)
	test.That(t, top1[0].Score(), test.ShouldAlmostEqual, 0.4)

	top3, err := cls.TopN(3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(top3), test.ShouldEqual, 3)
	test.That(t, top3[0].Score(), test.ShouldAlmostEqual, 0.4)
	test.That(t, top3[1].Score(), test.ShouldAlmostEqual, 0.2)
	test.That(t, top3[2].Score(), test.ShouldAlmostEqual, 0.1)

	topN, err := cls.TopN(10000000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(topN), test.ShouldEqual, 3)

	top0, err := cls.TopN(0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(top0), test.ShouldEqual, 3)

	topNeg, err := cls.TopN(-5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(topNeg), test.ShouldEqual, 3)

}
