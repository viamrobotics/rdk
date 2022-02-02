package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/framesystem"
)

// FrameSystemService is an injected FrameSystem service.
type FrameSystemService struct {
	framesystem.Service
	ConfigFunc           func(ctx context.Context) ([]*config.FrameSystemPart, error)
	LocalFrameSystemFunc func(ctx context.Context, name string, prefix string) (referenceframe.FrameSystem, error)
}

// Config calls the injected Config or the real version.
func (fss *FrameSystemService) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	if fss.ConfigFunc == nil {
		return fss.Config(ctx)
	}
	return fss.ConfigFunc(ctx)
}

// LocalFrameSystem calls the injected FrameSystem or the real version.
func (fss *FrameSystemService) LocalFrameSystem(ctx context.Context, name string, prefix string) (referenceframe.FrameSystem, error) {
	if fss.LocalFrameSystemFunc == nil {
		return fss.LocalFrameSystem(ctx, name, prefix)
	}
	return fss.LocalFrameSystemFunc(ctx, name, prefix)
}
