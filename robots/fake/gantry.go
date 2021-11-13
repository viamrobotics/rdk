package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/component/gantry"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterComponent(gantry.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGantry(config.Name), nil
		}})
}

func newGantry(name string) gantry.Gantry {
	return &fakeGantry{name: name, positions: []float64{1.2}, lengths: []float64{5}}
}

type fakeGantry struct {
	name      string
	positions []float64
	lengths   []float64
}

// CurrentPosition returns the position in meters
func (g *fakeGantry) CurrentPosition(ctx context.Context) ([]float64, error) {
	return g.positions, nil
}

// Lengths returns the position in meters
func (g *fakeGantry) Lengths(ctx context.Context) ([]float64, error) {
	return g.lengths, nil
}

// MoveToPosition is in meters
func (g *fakeGantry) MoveToPosition(ctx context.Context, positions []float64) error {
	g.positions = positions
	return nil
}

func (g *fakeGantry) ModelFrame() *kinematics.Model {
	axes := []bool{}
	limits := []referenceframe.Limit{}

	for _, l := range g.lengths {
		axes = append(axes, true)
		limits = append(limits, referenceframe.Limit{0, l})
	}

	f, err := referenceframe.NewTranslationalFrame(
		g.name,
		axes,
		limits,
	)
	if err != nil {
		panic(err)
	}
	m := kinematics.NewModel()
	m.OrdTransforms = append(m.OrdTransforms, f)
	return m
}
