package wheeled

import (
	"context"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

type kinematicWheeledBase struct {
	*wheeledBase
	slam  slam.Service
	model referenceframe.Model
}

func WrapWithKinematics(base *wheeledBase, slam slam.Service) (base.KinematicBase, error) {
	var err error
	kwb := &kinematicWheeledBase{
		wheeledBase: base,
		slam:        slam,
	}
	return kwb, err
}

func (kwb *kinematicWheeledBase) ModelFrame() referenceframe.Model {
	return kwb.model
}

func (kwb *kinematicWheeledBase) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	// TODO(RSDK-2311): complete the implementation
	return []referenceframe.Input{}, errors.New("not implemented yet")
}

func (kwb *kinematicWheeledBase) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// TODO(RSDK-2311): complete the implementation
	return errors.New("not implemented yet")
}

func model(cfg config.Component) (referenceframe.Model, error) {
	// TODO(rb): this is a hacky workaround for not having kinematics for bases yet
	// we create a sphere that would encompass the config geometry's rotation a full 360 degrees
	geoCfg := cfg.Frame.Geometry
	r := geoCfg.TranslationOffset.Norm()
	switch geoCfg.Type {
	case spatialmath.BoxType:
		r += r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}.Norm() / 2
	case spatialmath.SphereType:
		r += geoCfg.R
	case spatialmath.CapsuleType:
		r += geoCfg.L / 2
	case spatialmath.PointType:
	case spatialmath.UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		if norm := (r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}).Norm(); norm > 0 {
			r += norm / 2
		} else if geoCfg.L != 0 {
			r += geoCfg.L / 2
		} else {
			r += geoCfg.R
		}
	default:
		return nil, spatialmath.ErrGeometryTypeUnsupported
	}
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), r, geoCfg.Label)
	if err != nil {
		// could not create a sphere, just use a point instead
		sphere = spatialmath.NewPoint(r3.Vector{}, geoCfg.Label)
	}

	// TODO(rb): figure out a better set of limits to impose on the base frame
	frame2D, err := referenceframe.NewMobile2DFrame(
		sphere.Label(),
		[]referenceframe.Limit{{Min: math.Inf(-1), Max: math.Inf(1)}, {Min: math.Inf(-1), Max: math.Inf(1)}},
		sphere)
	if err != nil {
		return nil, err
	}
	model := referenceframe.NewSimpleModel(cfg.Name)
	model.OrdTransforms = []referenceframe.Frame{frame2D}
	return model, nil
}
