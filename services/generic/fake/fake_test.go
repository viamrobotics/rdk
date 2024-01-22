package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/generic"
)

func TestDoCommand(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	gen := newGeneric(generic.Named("foo"), logger)
	cmd := map[string]interface{}{"bar": "baz"}
	resp, err := gen.DoCommand(ctx, cmd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, cmd)
}
