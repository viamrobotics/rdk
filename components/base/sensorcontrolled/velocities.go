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
const useControlLoop = true

// setupControlLoops uses the embedded config in this file to initialize a control
// loop using the controls package and stor in on the sensor controlled base struct
// the sensor base in the controllable interface that implements State and GetState
// called by the endpoing logic of the control thread and the controlLoopConfig
// is included at the end of this file.
func setupControlLoops(sb *sensorBase) error {
	sb.logger.Error("setupControlLoops")
	// TODO: RSDK-5355 useControlLoop bool should be removed after testing
	if useControlLoop {
		loop, err := control.NewLoop(sb.logger, controlLoopConfig, sb)
		if err != nil {
			return err
		}
		// time.Sleep(1 * time.Second)
		if err := loop.Start(); err != nil {
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

	// set the spin loop to false, so we do not skip the call to SetState in the control loop
	sb.setPolling(false)

	// start a sensor context for the sensor loop based on the longstanding base
	// creator context, and add a timeout for the context
	timeOut := 10 * time.Second
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithTimeout(context.Background(), timeOut)

	// TODO: RSDK-5355 remove control loop bool after testing
	if useControlLoop && sb.loop != nil {
		posConf := control.BlockConfig{
			Name: "set_point",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": linear,
			},
			DependsOn: []string{},
		}
		if err := sb.loop.SetConfigAt(ctx, "set_point", posConf); err != nil {
			return err
		}
		// if we have a loop, let's use the SetState function to call the SetVelocity command
		// through the control loop
		sb.logger.Info("using loop")
		sb.pollsensors(sensorCtx, extra)
		return nil
	}

	sb.logger.Info("setting velocity without loop")
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
				// sb.logger.Error("not polling, so stopping")
				ticker.Stop()
			}

			if err := ctx.Err(); err != nil {
				sb.logger.Error(err)
				return
			}

			select {
			case <-ctx.Done():
				// sb.logger.Error("ctx done")
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
		// this allows us to keep the control loop running without stopping it until
		// the resource Close has been called
		sb.logger.Info("skipping set state call")
		return nil
	}

	// sb.logger.Info("setting state")
	linvel := state[0].GetSignalValueAt(0)
	angvel := state[1].GetSignalValueAt(0)

	return sb.controlledBase.SetPower(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)

	// return sb.SetVelocity(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)
}

// State is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// movementsensor and insert its LinearVelocity and AngularVelocity values
// in the signal in the control loop's thread in the endpoint code.
func (sb *sensorBase) State(ctx context.Context) ([]float64, error) {
	// sb.logger.Info("getting state")
	linvel, err := sb.velocities.LinearVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}

	angvel, err := sb.velocities.AngularVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}
	// sb.logger.Warnf("linvel = %v, angvel = %v", linvel, angvel)

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
			Name: "endpoint",
			Type: "endpoint",
			Attribute: rdkutils.AttributeMap{
				"base_name": "viam_base",
			},
			DependsOn: []string{"PID"},
		},
		{
			Name: "PID",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"kP":             0.0, // kP, kD and kI are random for now
				"kD":             0.0,
				"kI":             0.0,
				"int_sat_lim_lo": -255.0,
				"int_sat_lim_up": 255.0,
				"limit_lo":       -255.0,
				"limit_up":       255.0,
				"tune_method":    "ziegerNicholsSomeOvershoot",
				"tune_ssr_value": 2.0,
				"tune_step_pct":  0.35,
			},
			DependsOn: []string{"gain"},
		},
		{
			Name: "sum",
			Type: "sum",
			Attribute: rdkutils.AttributeMap{
				"sum_string": "+-", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"endpoint", "set_point"},
		},
		{
			Name: "gain",
			Type: "gain",
			Attribute: rdkutils.AttributeMap{
				"gain": 0.3, // need to update dynamically? Or should I just use the trapezoidal velocity profile
			},
			DependsOn: []string{"sum"},
		},
		{
			Name: "set_point",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 0.0,
			},
			DependsOn: []string{},
		},
	},
	Frequency: 20,
}
