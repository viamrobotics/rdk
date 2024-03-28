package sensorcontrolled

import (
	"context"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/control"
)

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	if len(sb.controlLoopConfig.Blocks) == 0 {
		sb.logger.CWarnf(ctx, "control parameters not configured, using %v's SetVelocity method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}

	// convert linear.Y mmPerSec to mPerSec, angular.Z is degPerSec
	if err := sb.updateControlConfig(ctx, linear.Y/1000.0, angular.Z); err != nil {
		return err
	}

	return nil
}
