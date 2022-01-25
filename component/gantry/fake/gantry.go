// Package fake implements a fake gantry.
package fake

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(gantry.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewGantry(config.Name), nil
		},
	})
}

// NewGantry returns a new fake gantry.
func NewGantry(name string) gantry.Gantry {
	return &Gantry{name, []float64{1.2}, []float64{5}, r3.Vector{1, 0, 0}, 2}
}

// Gantry is a fake gantry that can simply read and set properties.
type Gantry struct {
	name         string
	positions    []float64
	lengths      []float64
	axis         r3.Vector
	lengthMeters float64
}

// CurrentPosition returns the position in meters.
func (g *Gantry) CurrentPosition(ctx context.Context) ([]float64, error) {
	return g.positions, nil
}

// Lengths returns the position in meters.
func (g *Gantry) Lengths(ctx context.Context) ([]float64, error) {
	return g.lengths, nil
}

// MoveToPosition is in meters.
func (g *Gantry) MoveToPosition(ctx context.Context, positions []float64) error {
	g.positions = positions
	return nil
}

// ModelFrame TODO.
func (g *Gantry) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
	f, err := referenceframe.NewTranslationalFrame(g.name, g.axis, referenceframe.Limit{0, g.lengthMeters})
	if err != nil {
		panic(fmt.Errorf("error creating frame: %w", err))
	}
	m.OrdTransforms = append(m.OrdTransforms, f)
	return m
}

// CurrentInputs TODO.
func (g *Gantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

// GoToInputs TODO.
func (g *Gantry) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal))
}
