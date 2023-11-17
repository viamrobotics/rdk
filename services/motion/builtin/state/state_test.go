package state_test

import (
	"context"
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/motion/builtin/state"
)

func TestState(t *testing.T) {
	logger := logging.NewTestLogger(t)
	t.Parallel()

	ctx := context.Background()

	t.Run("creating & stopping a state with no intermediary calls", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
	})
}
