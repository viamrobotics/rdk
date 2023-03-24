package utils

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestTrustedEnvironment(t *testing.T) {
	test.That(t, IsTrustedEnvironment(context.Background()), test.ShouldBeTrue)

	newCtx, err := WithTrustedEnvironment(context.Background(), true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, IsTrustedEnvironment(newCtx), test.ShouldBeTrue)

	newCtx, err = WithTrustedEnvironment(context.Background(), false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, IsTrustedEnvironment(newCtx), test.ShouldBeFalse)

	newCtx, err = WithTrustedEnvironment(newCtx, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, IsTrustedEnvironment(newCtx), test.ShouldBeFalse)

	newCtx, err = WithTrustedEnvironment(newCtx, true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "elevate")
	test.That(t, newCtx, test.ShouldBeNil)
}
