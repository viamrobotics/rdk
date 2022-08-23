package server

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestRestartChecker(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	t.Run("without AppAddress should create HTTP", func(t *testing.T) {
		c, err := newRestartChecker(ctx, &config.Cloud{}, logger)
		test.That(t, err, test.ShouldBeNil)
		defer c.close()

		_, ok := c.(*needsRestartCheckerHTTP)
		test.That(t, ok, test.ShouldBeTrue)
	})
}
