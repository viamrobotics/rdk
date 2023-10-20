package sensorcontrolled

import (
	"context"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

const useControlLoop = true

func setupControlLoops(sb *sensorBase) error {
	// TODO: RSDK-XXXX make control loop should be removed after testing
	if useControlLoop {
		loop, err := control.NewLoop(sb.logger, controlLoopConfig, sb)
		if err != nil {
			return err
		}
		sb.loop = loop
	}

	return nil
}

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	// check if a sensor context has been started
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.setPolling(true)
	// start a sensor context for the sensor loop based on the longstanding base
	// creator context, and add a timeout for the context
	timeOut := 10 * time.Second
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithTimeout(context.Background(), timeOut)

	if err := sb.loop.Start(); err != nil {
		return err
	}
	// TODO: RSDK-XXXX remove control loop bool after testing
	if useControlLoop {
		// if we have a loop, ;et's use the SetState function to call the SetVelocity command
		if err := sb.loop.Start(); err != nil {
			return err
		}

		sb.pollsensors(sensorCtx, extra)
		return errors.New(
			"setvelocity with sensor feedback not currently implemented, remove movement sensor reporting linear and angular velocity ")
	}
	// else do not use the control loop and pass through the SetVelocity command
	return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
}

func (sb *sensorBase) pollsensors(ctx context.Context, extra map[string]interface{}) {
	sb.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(velocitiesPollTime)
		defer ticker.Stop()

		for {
			// check if we want to poll the sensor at all
			// other API calls set this to false so that this for loop stops
			if !sb.isPolling() {
				ticker.Stop()
			}

			if err := ctx.Err(); err != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				linvel, err := sb.velocities.LinearVelocity(ctx, extra)
				if err != nil {
					sb.logger.Error(err)
					return
				}

				angvel, err := sb.velocities.AngularVelocity(ctx, extra)
				if err != nil {
					sb.logger.Error(err)
					return
				}

				if sensorDebug {
					sb.logger.Infof("sensor readings: linear: %#v, angular %#v", linvel, angvel)
				}
			}
		}
	}, sb.activeBackgroundWorkers.Done)
}

func (sb *sensorBase) SetState(ctx context.Context, state []*control.Signal) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if sb.isPolling() {
		// TODO: CHECK
		// if the spin loop is polling, don't call set velcity, immediately return
		// this allows us to keep the control loop unning without stopping it until
		// the resource Close has been called
		return nil
	}

	linvel := state[0].GetSignalValueAt(0)
	angvel := state[1].GetSignalValueAt(0)

	return sb.SetVelocity(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)
}

func (sb *sensorBase) State(ctx context.Context) ([]float64, error) {
	linvel, err := sb.velocities.LinearVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}

	angvel, err := sb.velocities.AngularVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}

	return []float64{linvel.Y, angvel.Z}, nil
}

// Control Loop Configuration.
var controlLoopConfig = control.Config{
	Blocks: []control.BlockConfig{
		{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: rdkutils.AttributeMap{
				"endpoint_name": "base", // How to input this
			},
			DependsOn: []string{"pid_block"},
		},
		{
			Name: "pid_block",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"kp": 1.0, // random for now
				"kd": 0.5,
				"kI": 0.2,
			},
			DependsOn: []string{"sum_block"},
		},
		{
			Name: "sum_block",
			Type: "sum",
			Attribute: rdkutils.AttributeMap{
				"sum_string": "-+", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"sensor-base", "setpoint"},
		},
		{
			Name: "setpoint",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 0.0, // need to update dynamically? Or should I just use the trapezoidal velocity profile
			},
			DependsOn: []string{""},
		},
	},
	Frequency: 50,
}
