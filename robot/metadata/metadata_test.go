package metadata_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/robot/metadata"
)

func TestNew(t *testing.T) {
	svc := metadata.New()
	test.That(t, svc, test.ShouldNotBeNil)

	resources, err := svc.Resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldHaveLength, 0)
}
