package sensorcontrolled

import (
	"context"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

// TODO: RSDK-5355 useControlLoop bool should be removed after testing.
const useControlLoop = false

// setupControlLoops uses the embedded config in this file to initialize a control
// loop using the controls package and stor in on the sensor controlled base struct
// the sensor base in the controllable interface that implements State and GetState
// called by the endpoing logic of the control thread and the controlLoopConfig
// is included at the end of this file.
func setupControlLoops(sb *sensorBase) error {
	// TODO: RSDK-5355 useControlLoop bool should be removed after testing
	if useControlLoop {
		loop, err := control.NewLoop(sb.logger, controlLoopConfig, sb)
		if err != nil {
			return err
		}
		sb.loop = loop

		if sb.loop != nil {
			if err := sb.loop.Start(); err != nil {
				return err
			}
		}
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

	// set the spin loop to false, so we do not skip the call to SetState in the control loop
	sb.setPolling(false)

	// start a sensor context for the sensor loop based on the longstanding base
	// creator context, and add a timeout for the context
	timeOut := 10 * time.Second
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithTimeout(context.Background(), timeOut)

	// TODO: RSDK-5355 remove control loop bool after testing
	if useControlLoop && sb.loop != nil {
		// if we have a loop, let's use the SetState function to call the SetVelocity command
		// through the control loop
		sb.logger.CInfo(ctx, "using loop")
		sb.pollsensors(sensorCtx, extra)
		return nil
	}

	sb.logger.CInfo(ctx, "setting velocity without loop")
	// else do not use the control loop and pass through the SetVelocity command
	return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
}

// pollsensors is a busy loop in the background that passively polls the LinearVelocity and
// AngularVelocity API calls of the movementsensor attached to the sensor base
// and logs them for toruble shooting.
// This function can eventually be removed.
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

// SetState is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// SetVelocity from within that package.
func (sb *sensorBase) SetState(ctx context.Context, state []*control.Signal) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if sb.isPolling() {
		// if the spin loop is polling, don't call set velocity, immediately return
		// this allows us to keep the control loop unning without stopping it until
		// the resource Close has been called
		sb.logger.CInfo(ctx, "skipping set state call")
		return nil
	}

	sb.logger.CInfo(ctx, "setting state")
	linvel := state[0].GetSignalValueAt(0)
	angvel := state[1].GetSignalValueAt(0)

	return sb.SetVelocity(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)
}

// State is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// movementsensor and insert its LinearVelocity and AngularVelocity values
// in the signal in the control loop's thread in the endpoint code.
func (sb *sensorBase) State(ctx context.Context) ([]float64, error) {
	sb.logger.CInfo(ctx, "getting state")
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

// Control Loop Configuration is embedded in this file so a user does not have to
// configure the loop from within the attributes of the config file.
// it sets up a loop that takes a constant -> sum -> gain -> PID -> Endpoint -> feedback to sum
// structure. The gain is 1 to not magnify the input signal, the PID values are experimental
// this structure can change as hardware experiments with the viam base require.
var controlLoopConfig = control.Config{
	Blocks: []control.BlockConfig{
		{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: rdkutils.AttributeMap{
				"base_name": "base",
			},
			DependsOn: []string{"pid_block"},
		},
		{
			Name: "pid_block",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"kP": 1.0, // kP, kD and kI are random for now
				"kD": 0.5,
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
			DependsOn: []string{"sensor-base", "constant"},
		},
		{
			Name: "gain_block",
			Type: "gain",
			Attribute: rdkutils.AttributeMap{
				"gain": 1.0, // need to update dynamically? Or should I just use the trapezoidal velocity profile
			},
			DependsOn: []string{"sum_block"},
		},
		{
			Name: "constant",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 1.0,
			},
			DependsOn: []string{},
		},
	},
	Frequency: 20,
}
