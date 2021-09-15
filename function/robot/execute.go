package functionrobot

import (
	"context"

	"github.com/go-errors/errors"

	functionvm "go.viam.com/core/function/vm"
	pb "go.viam.com/core/proto/api/v1"
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
	if err := engine.ImportFunction("robot.boardMotorPower", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 3 {
			return nil, errors.New("expected 3 arguments for board name, motor name, and power percentage")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		powerPct, err := args[2].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.Power(context.TODO(), float32(powerPct))
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorGo", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 4 {
			return nil, errors.New("expected 4 arguments for board name, motor name, direction (1-forward,2-backward), and power percentage")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		dirRel, err := args[2].Number()
		if err != nil {
			return nil, err
		}
		powerPct, err := args[3].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.Go(context.TODO(), pb.DirectionRelative(dirRel), float32(powerPct))
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorGoFor", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 5 {
			return nil, errors.New("expected 5 arguments for board name, motor name, direction (1-forward,2-backward), rpm, and revolutions")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		dirRel, err := args[2].Number()
		if err != nil {
			return nil, err
		}
		rpm, err := args[3].Number()
		if err != nil {
			return nil, err
		}
		revolutions, err := args[4].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoFor(context.TODO(), pb.DirectionRelative(dirRel), rpm, revolutions)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorGoTo", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 4 {
			return nil, errors.New("expected 4 arguments for board name, motor name, rpm, and position")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		rpm, err := args[2].Number()
		if err != nil {
			return nil, err
		}
		position, err := args[3].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoTo(context.TODO(), rpm, position)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorGoTillStop", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 4 {
			return nil, errors.New("expected 4 arguments for board name, motor name, direction (1-forward,2-backward), and rpm")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		dirRel, err := args[2].Number()
		if err != nil {
			return nil, err
		}
		rpm, err := args[3].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.GoTillStop(context.TODO(), pb.DirectionRelative(dirRel), rpm, nil)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorZero", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 4 {
			return nil, errors.New("expected 4 arguments for board name, motor name, and offset")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}
		offset, err := args[2].Number()
		if err != nil {
			return nil, err
		}

		return nil, motor.Zero(context.TODO(), offset)
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorPosition", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for board name and motor name")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
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
	if err := engine.ImportFunction("robot.boardMotorPositionSupported", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for board name and motor name")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
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
	if err := engine.ImportFunction("robot.boardMotorOff", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for board name and motor name")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
		if !ok {
			return nil, errors.Errorf("no motor with that name %s", motorName)
		}

		return nil, motor.Off(context.TODO())
	}); err != nil {
		return nil, err
	}
	if err := engine.ImportFunction("robot.boardMotorIsOn", func(args ...functionvm.Value) ([]functionvm.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("expected 2 arguments for board name and motor name")
		}
		boardName, err := args[0].String()
		if err != nil {
			return nil, err
		}
		board, ok := r.BoardByName(boardName)
		if !ok {
			return nil, errors.Errorf("no board with that name %s", boardName)
		}
		motorName, err := args[1].String()
		if err != nil {
			return nil, err
		}
		motor, ok := board.MotorByName(motorName)
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
