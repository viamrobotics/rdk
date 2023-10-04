package builtin

import (
	"context"
)

//nolint:unparam
func (svc *builtIn) startExploreMode(ctx context.Context) {
	svc.logger.Warn("Explore mode is currently disabled")
}
