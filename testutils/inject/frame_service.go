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
	ConfigFunc      func(ctx context.Context) ([]*config.FrameSystemPart, error)
	FrameSystemFunc func(ctx context.Context, name string, prefix string) (referenceframe.FrameSystem, error)
}

// FrameSystemConfig calls the injected FrameSystemConfig or the real version.
func (fss *FrameSystemService) Config(ctx context.Context) ([]*config.FrameSystemPart, error) {
	if fss.ConfigFunc == nil {
		return fss.Config(ctx)
	}
	return fss.ConfigFunc(ctx)
}

// FrameSystem calls the injected FrameSystem or the real version.
func (fss *FrameSystemService) FrameSystem(ctx context.Context, name string, prefix string) (referenceframe.FrameSystem, error) {
	if fss.FrameSystemFunc == nil {
		return fss.FrameSystem(ctx, name, prefix)
	}
	return fss.FrameSystemFunc(ctx, name, prefix)
}
