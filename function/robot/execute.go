package functionrobot

import (
	"context"

	"github.com/go-errors/errors"

	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/robot"
)

// ExecutionResult is the result of executing a particular piece of code.
type ExecutionResult struct {
	Results []functionvm.Value
	StdOut  string
	StdErr  string
}

// Execute executes the given function with an accessible robot.
func Execute(ctx context.Context, f functionvm.FunctionConfig, r robot.Robot) (*ExecutionResult, error) {
	engine, err := functionvm.NewEngine(f.Engine)
	if err != nil {
		return nil, err
	}
	// TODO(erd): maybe this should be an argument and not a global set of functions
	if err := engine.ImportFunction("robot.gripperOpen", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for gripper name")
		}
		gripperName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		gripper, ok := r.GripperByName(gripperName)
		if !ok {
			return nil, errors.Errorf("no gripper with that name %s", gripperName)
		}
		return nil, gripper.Open(ctx)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.gripperGrab", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for gripper name")
		}
		gripperName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		gripper, ok := r.GripperByName(gripperName)
		if !ok {
			return nil, errors.Errorf("no gripper with that name %s", gripperName)
		}
		grabbed, err := gripper.Grab(ctx)
		if err != nil {
			return nil, err
		}
		return []functionvm.Value{functionvm.NewBool(grabbed)}, nil
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorPower", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for motor name and power percentage")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		powerPct, err := args[1].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.SetPower(context.TODO(), powerPct)

	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorGo", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 3 arguments for motor name, and power percentage")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		powerPct, err := args[1].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.Go(context.TODO(), powerPct)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorGoFor", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 3 {
			return nil, errors.New("expected 3 arguments for motor name, rpm, and revolutions")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		rpm, err := args[1].Number()
		if err != nil {
			return nil, err
		}
		revolutions, err := args[2].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoFor(context.TODO(), rpm, revolutions)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorGoTo", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 3 {
			return nil, errors.New("expected 3 arguments for motor name, rpm, and position")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		rpm, err := args[1].Number()
		if err != nil {
			return nil, err
		}
		position, err := args[2].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoTo(context.TODO(), rpm, position)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorGoTillStop", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 3 arguments for motor name, and rpm (Note: stopFunc input current not available")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		rpm, err := args[1].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoTillStop(context.TODO(), rpm, nil)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorZero", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for motor name and offset")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		offset, err := args[1].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.SetToZeroPosition(context.TODO(), offset)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorPosition", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for motor name")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		pos, err := motor.Position(context.TODO())
		if err != nil {
			return nil, err
		}
		return []functionvm.Value{functionvm.NewFloat(pos)}, nil
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorPositionSupported", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for motor name")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		isSupported, err := motor.PositionSupported(context.TODO())
		if err != nil {
			return nil, err
		}
		return []functionvm.Value{functionvm.NewBool(isSupported)}, nil
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorOff", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for motor name")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		return nil, motor.Off(context.TODO())
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.motorIsOn", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 1 {
			return nil, errors.New("expected 1 argument for motor name")
		}
		motorName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		motor, ok := r.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		isOn, err := motor.IsOn(context.TODO())
		if err != nil {
			return nil, err
		}
		return []functionvm.Value{functionvm.NewBool(isOn)}, nil
	}); err != nil {
		return nil, err
	}
	results, err := engine.ExecuteSource(f.Source)
	return &ExecutionResult{Results: results, StdOut: engine.StandardOutput(), StdErr: engine.StandardError()}, err
}
