//go:build !windows && !no_cgo

package sim

import (
	"context"
	"errors"

	"github.com/viam-modules/trajex/go/totg"
	trajexrdk "github.com/viam-modules/trajex/go/totg/rdk"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
)

// trajexTrajectoryGenerator is the cgo-required, TOTG-backed implementation.
type trajexTrajectoryGenerator struct {
	svc *trajexrdk.Service
}

// newTrajectoryGenerator returns the trajex-backed generator under cgo builds.
// The logger is unused here (trajex surfaces errors directly) but accepted for
// signature parity with the no_cgo variant.
func newTrajectoryGenerator(_ logging.Logger) trajectoryGenerator {
	return &trajexTrajectoryGenerator{
		svc: trajexrdk.NewService(resource.NewName(mlmodel.API, "")),
	}
}

func (g *trajexTrajectoryGenerator) Plan(
	ctx context.Context,
	waypoints [][]float64,
	velLimit, accelLimit float64,
	pathTolerance float64,
) (*plannedTrajectory, error) {
	if len(waypoints) == 0 {
		return nil, errors.New("at least one waypoint is required")
	}
	nDof := len(waypoints[0])

	flatWaypoints := make([]float64, 0, len(waypoints)*nDof)
	for _, wp := range waypoints {
		flatWaypoints = append(flatWaypoints, wp...)
	}

	velLimits := make([]float64, nDof)
	accelLimits := make([]float64, nDof)
	for i := range velLimits {
		velLimits[i] = velLimit
		accelLimits[i] = accelLimit
	}

	inputs := ml.Tensors{
		totg.KeyWaypointsRads: tensor.New(
			tensor.WithShape(len(waypoints), nDof),
			tensor.WithBacking(flatWaypoints),
		),
		totg.KeyVelocityLimitsRadsPerSec: tensor.New(
			tensor.WithShape(nDof),
			tensor.WithBacking(velLimits),
		),
		totg.KeyAccelerationLimitsRadsPerSec2: tensor.New(
			tensor.WithShape(nDof),
			tensor.WithBacking(accelLimits),
		),
		totg.KeyPathToleranceDeltaRads: tensor.New(
			tensor.WithShape(1),
			tensor.WithBacking([]float64{pathTolerance}),
		),
	}

	outputs, err := g.svc.Infer(ctx, inputs)
	if err != nil {
		return nil, err
	}

	timesT, ok := outputs[totg.KeySampleTimesSec]
	if !ok {
		return nil, errors.New("trajex output missing sample times")
	}
	configsT, ok := outputs[totg.KeyConfigurationsRads]
	if !ok {
		return nil, errors.New("trajex output missing configurations")
	}

	return &plannedTrajectory{
		sampleTimes:   timesT.Data().([]float64),
		sampleConfigs: configsT.Data().([]float64),
		nDof:          nDof,
	}, nil
}
