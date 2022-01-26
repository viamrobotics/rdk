// Package fake implements a fake gantry.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(gantry.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGantry(config.Name), nil
		},
	})
}

func newGantry(name string) gantry.Gantry {
	return &fakeGantry{name: name, positionsMm: []float64{1.2}, lengths: []float64{5}}
}

type fakeGantry struct {
	name        string
	positionsMm []float64
	lengths     []float64
}

// GetPosition returns the position in meters.
func (g *fakeGantry) GetPosition(ctx context.Context) ([]float64, error) {
	return g.positionsMm, nil
}

// GetLengths returns the position in meters.
func (g *fakeGantry) GetLengths(ctx context.Context) ([]float64, error) {
	return g.lengths, nil
}

// MoveToPosition is in meters.
func (g *fakeGantry) MoveToPosition(ctx context.Context, positionsMm []float64) error {
	g.positionsMm = positionsMm
	return nil
}

func (g *fakeGantry) ModelFrame() referenceframe.Model {
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
	m := referenceframe.NewSimpleModel()
	m.OrdTransforms = append(m.OrdTransforms, f)
	return m
}

func (g *fakeGantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

func (g *fakeGantry) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}
