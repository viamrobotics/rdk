// Package fake implements a fake gantry.
package fake

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
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
	positionsMm  []float64
	lengths      []float64
	axis         r3.Vector
	lengthMeters float64
}

// GetPosition returns the position in meters.
func (g *Gantry) GetPosition(ctx context.Context) ([]float64, error) {
	return g.positionsMm, nil
}

// GetLengths returns the position in meters.
func (g *Gantry) GetLengths(ctx context.Context) ([]float64, error) {
	return g.lengths, nil
}

// MoveToPosition is in meters.
func (g *Gantry) MoveToPosition(ctx context.Context, positionsMm []float64, worldState *commonpb.WorldState) error {
	g.positionsMm = positionsMm
	return nil
}

// ModelFrame returns a Gantry frame.
func (g *Gantry) ModelFrame() referenceframe.Model {
	m := referenceframe.NewSimpleModel()
	f, err := referenceframe.NewTranslationalFrame(g.name, g.axis, referenceframe.Limit{0, g.lengthMeters})
	if err != nil {
		panic(fmt.Errorf("error creating frame: %w", err))
	}
	m.OrdTransforms = append(m.OrdTransforms, f)
	return m
}

// CurrentInputs returns positions in the Gantry frame model..
func (g *Gantry) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := g.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.FloatsToInputs(res), nil
}

// GoToInputs moves using the Gantry frames..
func (g *Gantry) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return g.MoveToPosition(ctx, referenceframe.InputsToFloats(goal), &commonpb.WorldState{})
}
