package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/framesystem"
)

// FrameSystemService is an injected FrameSystem service
type FrameSystemService struct {
	framesystem.Service
	FrameSystemConfigFunc func(ctx context.Context) ([]*config.FrameSystemPart, error)
	LocalFrameSystemFunc  func(ctx context.Context, name string) (referenceframe.FrameSystem, error)
	ModelFrameFunc        func(ctx context.Context, name string) (*referenceframe.Model, error)
}

// FrameSystemConfig calls the injected FrameSystemConfig or the real version.
func (fss *FrameSystemService) FrameSystemConfig(ctx context.Context) ([]*config.FrameSystemPart, error) {
	if fss.FrameSystemConfigFunc == nil {
		return fss.FrameSystemConfig(ctx)
	}
	return fss.FrameSystemConfigFunc(ctx)
}

// LocalFrameSystem calls the injected LocalFrameSystem or the real version.
func (fss *FrameSystemService) LocalFrameSystem(ctx context.Context, name string) (referenceframe.FrameSystem, error) {
	if fss.LocalFrameSystemFunc == nil {
		return fss.LocalFrameSystem(ctx, name)
	}
	return fss.LocalFrameSystemFunc(ctx, name)
}

// ModelFrame calls the injected ModelFrame or the real version.
func (fss *FrameSystemService) ModelFrame(ctx context.Context, name string) (*referenceframe.Model, error) {
	if fss.ModelFrameFunc == nil {
		return fss.ModelFrame(ctx, name)
	}
	return fss.ModelFrameFunc(ctx, name)
}
