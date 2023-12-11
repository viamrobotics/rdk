package sensorcontrolled

import (
	"context"
	"math"
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
func (sb *sensorBase) setupControlLoops() error {
	// create control loop
	loop, err := control.NewLoop(sb.logger, controlLoopConfig, sb)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	sb.loop = loop

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

	if useControlLoop {
		// stop and restart loop
		if sb.loop != nil {
			if err := sb.Stop(ctx, nil); err != nil {
				sb.logger.Error(err)
			}
		}
		loop, err := control.NewLoop(sb.logger, controlLoopConfig, sb)
		if err != nil {
			return err
		}
		if err := loop.Start(); err != nil {
			return err
		}
		sb.loop = loop

		// set linear setpoint config
		linConf := control.BlockConfig{
			Name: "linear_setpoint",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": linear.Y / 1000.0,
			},
			DependsOn: []string{},
		}
		if err := sb.loop.SetConfigAt(ctx, "linear_setpoint", linConf); err != nil {
			return err
		}

		// set angular setpoint config
		angConf := control.BlockConfig{
			Name: "angular_setpoint",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": angular.Z,
			},
			DependsOn: []string{},
		}
		if err := sb.loop.SetConfigAt(ctx, "angular_setpoint", angConf); err != nil {
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

func sign(x float64) float64 { // A quick helper function
	if math.Signbit(x) {
		return -1.0
	}
	return 1.0
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

	linvel := state[0].GetSignalValueAt(0)
	// FIX: multiply angvel by the sign of linvel... why does this work?
	angvel := (state[1].GetSignalValueAt(0) * sign(linvel))

	return sb.SetPower(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)
}

// State is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// movementsensor and insert its LinearVelocity and AngularVelocity values
// in the signal in the control loop's thread in the endpoint code.
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

// Control Loop Configuration is embedded in this file so a user does not have to
// configure the loop from within the attributes of the config file.
// it sets up a loop that takes a constant -> sum -> PID -> gain -> Endpoint -> feedback to sum
// structure. The gain is 0.0039 (1/255) to account for the PID range, the PID values are experimental
// this structure can change as hardware experiments with the viam base require.
var controlLoopConfig = control.Config{
	Blocks: []control.BlockConfig{
		{
			Name: "endpoint",
			Type: "endpoint",
			Attribute: rdkutils.AttributeMap{
				"base_name": "feedback",
			},
			DependsOn: []string{"linear_gain", "angular_gain"},
		},
		{
			Name: "linear_PID",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"kD":             0.0,
				"kI":             520.763911,
				"kP":             291.489819,
				"int_sat_lim_lo": -255.0,
				"int_sat_lim_up": 255.0,
				"limit_lo":       -255.0,
				"limit_up":       255.0,
				"tune_method":    "ziegerNicholsPI",
				"tune_ssr_value": 2.0,
				"tune_step_pct":  0.35,
			},
			DependsOn: []string{"sum"},
		},
		{
			Name: "angular_PID",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"kD":             0.0,
				"kI":             0.904513,
				"kP":             0.677894,
				"int_sat_lim_lo": -255.0,
				"int_sat_lim_up": 255.0,
				"limit_lo":       -255.0,
				"limit_up":       255.0,
				"tune_method":    "ziegerNicholsPI",
				"tune_ssr_value": 2.0,
				"tune_step_pct":  0.35,
			},
			DependsOn: []string{"sum"},
		},
		{
			Name: "sum",
			Type: "sum",
			Attribute: rdkutils.AttributeMap{
				"sum_string": "++-", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"linear_setpoint", "angular_setpoint", "endpoint"},
		},
		{
			Name: "linear_gain",
			Type: "gain",
			Attribute: rdkutils.AttributeMap{
				"gain": 0.00392157, // need to update dynamically? Or should I just use the trapezoidal velocity profile
			},
			DependsOn: []string{"linear_PID"},
		},
		{
			Name: "angular_gain",
			Type: "gain",
			Attribute: rdkutils.AttributeMap{
				"gain": 0.00392157, // need to update dynamically? Or should I just use the trapezoidal velocity profile
			},
			DependsOn: []string{"angular_PID"},
		},
		{
			Name: "linear_setpoint",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 0.0,
			},
			DependsOn: []string{},
		},
		{
			Name: "angular_setpoint",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 0.0,
			},
			DependsOn: []string{},
		},
	},
	Frequency: 100,
}
