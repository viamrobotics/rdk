package referenceframe

import (
	"context"
	"testing"

	"go.viam.com/test"

	pb "go.viam.com/core/proto/api/v1"
)

func TestOffset(t *testing.T) {
	a := &pb.ArmPosition{X: 1, Y: 1, Z: 1}
	b := &pb.ArmPosition{X: 1, Y: 1, Z: 1}
	c := OffsetAdd(a, b)
	test.That(t, c.X, test.ShouldEqual, 2)
	test.That(t, c.Y, test.ShouldEqual, 2)
	test.That(t, c.Z, test.ShouldEqual, 2)
}

func TestFindTranslation(t *testing.T) {
	ctx := context.Background()

	basicFrameMap := basicFrameMap{}
	basicFrameMap.add(&basicFrame{name: "base"})
	basicFrameMap.add(&basicFrame{name: "basex"})
	basicFrameMap.add(&basicFrame{name: "arm", parent: "base", offset: &pb.ArmPosition{X: 1, Y: 1, Z: 1}})
	basicFrameMap.add(&basicFrame{name: "camera", parent: "arm", offset: &pb.ArmPosition{X: 1, Y: 1, Z: 1}})

	trans, err := FindTranslationChildToParent(ctx, &basicFrameMap, "camera", "base")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldEqual, 2)
	test.That(t, trans.Y, test.ShouldEqual, 2)
	test.That(t, trans.Z, test.ShouldEqual, 2)

	_, err = FindTranslationChildToParent(ctx, &basicFrameMap, "camera", "basex")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestFindTranslation2(t *testing.T) {
	ctx := context.Background()

	a := &pb.ArmPosition{X: 0.96, Y: 0.28, Z: 0, OX: 1.2, OY: 1.4, OZ: 1.5, Theta: 1.5}
	b := &pb.ArmPosition{X: 0.5, Y: 0.5, Z: 0.5, OX: 3.2, OY: 5.4, OZ: 5.5, Theta: 5.5}

	c1 := OffsetAdd(a, b)
	c2 := OffsetAdd(b, a)

	test.That(t, c1.X, test.ShouldNotAlmostEqual, c2.X)

	basicFrameMap := basicFrameMap{}
	basicFrameMap.add(&basicFrame{name: "base"})
	basicFrameMap.add(&basicFrame{name: "arm", parent: "base", offset: a})
	basicFrameMap.add(&basicFrame{name: "camera", parent: "arm", offset: b})

	trans, err := FindTranslationChildToParent(ctx, &basicFrameMap, "camera", "base")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldAlmostEqual, c1.X)
}
