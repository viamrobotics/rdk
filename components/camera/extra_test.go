package camera

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestExtraEmpty(t *testing.T) {
	ctx := context.Background()
	_, ok := FromContext(ctx)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestExtraRoundtrip(t *testing.T) {
	ctx := context.Background()
	expected := Extra{
		"It goes one": "by one",
		"even two":    "by two",
	}

	ctx = NewContext(ctx, expected)
	actual, ok := FromContext(ctx)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, actual, test.ShouldEqual, expected)
}
