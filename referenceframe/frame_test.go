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
	c := OffsetBy(a, b)
	test.That(t, c.X, test.ShouldEqual, 2)
	test.That(t, c.Y, test.ShouldEqual, 2)
	test.That(t, c.Z, test.ShouldEqual, 2)
}

func TestFindTranslation(t *testing.T) {
	ctx := context.Background()

	myMap := basicFrameMap{}
	myMap.add(&basicFrame{name: "base"})
	myMap.add(&basicFrame{name: "basex"})
	myMap.add(&basicFrame{name: "arm", parent: "base", offset: &pb.ArmPosition{X: 1, Y: 1, Z: 1}})
	myMap.add(&basicFrame{name: "camera", parent: "arm", offset: &pb.ArmPosition{X: 1, Y: 1, Z: 1}})

	trans, err := FindTranslationChildToParent(ctx, &myMap, "camera", "base")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldEqual, 2)
	test.That(t, trans.Y, test.ShouldEqual, 2)
	test.That(t, trans.Z, test.ShouldEqual, 2)

	_, err = FindTranslationChildToParent(ctx, &myMap, "camera", "basex")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestFindTranslationOrderOfOperations(t *testing.T) {
	ctx := context.Background()

	a := &pb.ArmPosition{X: 0.96, Y: 0.28, Z: 0, OX: 1.2, OY: 1.4, OZ: 1.5, Theta: 1.5}
	b := &pb.ArmPosition{X: 0.5, Y: 0.5, Z: 0.5, OX: 3.2, OY: 5.4, OZ: 5.5, Theta: 5.5}

	c1 := OffsetBy(a, b)
	c2 := OffsetBy(b, a)

	test.That(t, c1.X, test.ShouldNotAlmostEqual, c2.X)

	basicFrameMap := basicFrameMap{}
	basicFrameMap.add(&basicFrame{name: "base"})
	basicFrameMap.add(&basicFrame{name: "arm", parent: "base", offset: a})
	basicFrameMap.add(&basicFrame{name: "camera", parent: "arm", offset: b})

	trans, err := FindTranslationChildToParent(ctx, &basicFrameMap, "camera", "base")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldAlmostEqual, c1.X)
}

func TestFindTranslationInfLoop(t *testing.T) {
	ctx := context.Background()

	a := &pb.ArmPosition{X: 0.96, Y: 0.28, Z: 0, OX: 1.2, OY: 1.4, OZ: 1.5, Theta: 1.5}
	b := &pb.ArmPosition{X: 0.5, Y: 0.5, Z: 0.5, OX: 3.2, OY: 5.4, OZ: 5.5, Theta: 5.5}

	myMap := basicFrameMap{}
	myMap.add(&basicFrame{name: "arm", parent: "camera", offset: a})
	myMap.add(&basicFrame{name: "camera", parent: "arm", offset: b})

	_, err := FindTranslationChildToParent(ctx, &myMap, "camera", "base")
	test.That(t, err, test.ShouldNotBeNil)
}
