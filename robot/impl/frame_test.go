package robotimpl

import (
	"context"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/test"

	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
)

func TestFrames1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	cfg, err := config.Read("data/frame1.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(), test.ShouldBeNil)
	}()

	lookup, err := CreateReferenceFrameLookup(ctx, r)
	test.That(t, err, test.ShouldBeNil)

	trans, err := referenceframe.FindTranslationChildToParent(ctx, lookup, "g", "a")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldEqual, 1)

	arm, ok := r.ArmByName("a")
	test.That(t, ok, test.ShouldBeTrue)
	err = arm.MoveToPosition(ctx, &pb.ArmPosition{X: 1, Y: 1, Z: 1})
	test.That(t, err, test.ShouldBeNil)

	trans, err = referenceframe.FindTranslationChildToParent(ctx, lookup, "g", "a")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, trans.X, test.ShouldEqual, 2)

}
