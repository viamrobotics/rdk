// Package main is a reset box for a robot play area.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/arm"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	pb "go.viam.com/core/proto/api/v1"

	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"

	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"
	_ "go.viam.com/core/robots/xarm"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const (
	backward = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD

	tipPinA = "29"
	tipPinB = "31"
	vibePin = "33"

	gateSpeed    = 25
	gateCubePass = 50
	gateOpen     = 80

	squeezeMaxWidth = 183
	squeezeClosed   = 40
	squeezeCubePass = 55
	squeezeOpen     = 80

	elevatorBottom = 25
	elevatorTop    = 850
	elevatorSpeed  = 300

	hammerSpeed  = 20 // May be capped by underlying motor MaxRPM
	hammerOffset = 0.9
	cubeWhacks   = 3.0
	duckWhacks   = 5.0

	armName     = "xArm6"
	gripperName = "vg1"
)

var (
	vibeLevel = uint8(96)

	safeDumpPos  = &pb.JointPositions{Degrees: []float64{0, -43, -71, 0, 98, 0}}
	cubeReadyPos = &pb.JointPositions{Degrees: []float64{-182.6, -26.8, -33.0, 0, 51.0, 0}}
	cube1grab    = &pb.JointPositions{Degrees: []float64{-182.6, 11.2, -51.8, 0, 48.6, 0}}
	cube2grab    = &pb.JointPositions{Degrees: []float64{-182.6, 7.3, -36.9, 0, 17.6, 0}}

	cube1place = &pb.JointPositions{Degrees: []float64{49.9, 14.0, -25.3, -0.5, -1.0, 0}}
	cube2place = &pb.JointPositions{Degrees: []float64{-116, 28.5, -25.7, -0.5, -32.2, 0}}

	duckgrabFW   = &pb.JointPositions{Degrees: []float64{-180.5, 27.7, -79.7, -2.8, 76.20, 180}}
	duckgrabREV  = &pb.JointPositions{Degrees: []float64{-180.5, 28.3, -76.8, -2.8, 65.45, 180}}
	duckReadyPos = &pb.JointPositions{Degrees: []float64{-180.5, 0.0, -60.0, -2.8, 65.45, 180}}

	duckplaceFW  = &pb.JointPositions{Degrees: []float64{-21.3, 14, -39.0, 6.3, 22.7, 50}}
	duckplaceREV = &pb.JointPositions{Degrees: []float64{-19.2, 18, -41.0, 6.3, 22.7, -130}}
)

var logger = golog.NewDevelopmentLogger("resetbox")

// LinearAxis is one or more motors whose motion is converted to linear movement via belts, screw drives, etc.
type LinearAxis struct {
	m        []board.Motor
	mmPerRev float64
}

// AddMotors takes a slice of motor names and adds them to the axis.
func (a *LinearAxis) AddMotors(ctx context.Context, board board.Board, names []string) error {
	for _, n := range names {
		motor, ok := board.MotorByName(n)
		if ok {
			a.m = append(a.m, motor)
		} else {
			return errors.Errorf("Cannot find motor named \"%s\"", n)
		}
	}
	return nil
}

// GoTo moves to a position specified in mm and at a speed in mm/s
func (a *LinearAxis) GoTo(ctx context.Context, speed float64, position float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.GoTo(ctx, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// GoFor moves for the distance specified in mm and at a speed in mm/s
func (a *LinearAxis) GoFor(ctx context.Context, d pb.DirectionRelative, speed float64, position float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.GoFor(ctx, d, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// GoTillStop simultaneously homes all motors on an axis
func (a *LinearAxis) GoTillStop(ctx context.Context, d pb.DirectionRelative, speed float64, stopFunc func(ctx context.Context) bool) error {
	var homeWorkers sync.WaitGroup
	var errs error
	for _, m := range a.m {
		homeWorkers.Add(1)
		go func(motor board.Motor) {
			defer homeWorkers.Done()
			multierr.AppendInto(&errs, motor.GoTillStop(ctx, d, speed*60/a.mmPerRev, nil))
		}(m)
	}
	homeWorkers.Wait()
	return errs
}

// Off turns the motor off
func (a *LinearAxis) Off(ctx context.Context) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Off(ctx))
	}
	return errs
}

// Zero resets the "home" point
func (a *LinearAxis) Zero(ctx context.Context, offset float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Zero(ctx, offset))
	}
	return errs
}

// Position returns the position of the first motor in the axis
func (a *LinearAxis) Position(ctx context.Context) (float64, error) {
	pos, err := a.m[0].Position(ctx)
	if err != nil {
		return 0, err
	}
	return pos * a.mmPerRev, nil
}

// IsOn returns true if moving
func (a *LinearAxis) IsOn(ctx context.Context) (bool, error) {
	var errs error
	for _, m := range a.m {
		on, err := m.IsOn(ctx)
		multierr.AppendInto(&errs, err)
		if on {
			return true, errs
		}
	}
	return false, errs
}

type positional interface {
	Position(ctx context.Context) (float64, error)
	IsOn(ctx context.Context) (bool, error)
}

// ResetBox is the parent structure for this project
type ResetBox struct {
	logger        golog.Logger
	board         board.Board
	gate, squeeze LinearAxis
	elevator      LinearAxis
	hammer        board.Motor
	arm           arm.Arm
	gripper       gripper.Gripper

	activeBackgroundWorkers *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context

	vibeState bool
	tableUp   bool
}

// NewResetBox returns a ResetBox
func NewResetBox(ctx context.Context, r robot.Robot, logger golog.Logger) (*ResetBox, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	b := &ResetBox{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel, logger: logger}
	resetboard, ok := r.BoardByName("resetboard")
	if !ok {
		return nil, errors.New("can't find board: resetboard")
	}
	b.board = resetboard

	b.gate.mmPerRev = 8.0
	b.squeeze.mmPerRev = 8.0
	b.elevator.mmPerRev = 60.0

	err := multierr.Combine(
		b.gate.AddMotors(cancelCtx, b.board, []string{"gateL", "gateR"}),
		b.squeeze.AddMotors(cancelCtx, b.board, []string{"squeezeL", "squeezeR"}),
		b.elevator.AddMotors(cancelCtx, b.board, []string{"elevator"}),
	)

	if err != nil {
		return nil, err
	}

	hammer, ok := b.board.MotorByName("hammer")
	if !ok {
		return nil, errors.New("can't find motor named: hammer")
	}
	b.hammer = hammer

	rArm, ok := r.ArmByName(armName)
	if !ok {
		return nil, fmt.Errorf("failed to find arm %s", armName)
	}
	b.arm = rArm

	rGripper, ok := r.GripperByName(gripperName)
	if !ok {
		return nil, fmt.Errorf("failed to find gripper %s", gripperName)
	}
	b.gripper = rGripper

	return b, nil
}

// Close stops motors and cancels context
func (b *ResetBox) Close() error {
	defer b.activeBackgroundWorkers.Wait()
	b.Stop(b.cancelCtx)
	b.cancel()
	return nil
}

// Stop turns off all motors
func (b *ResetBox) Stop(ctx context.Context) error {
	return multierr.Combine(
		b.elevator.Off(ctx),
		b.gate.Off(ctx),
		b.gate.Off(ctx),
		b.squeeze.Off(ctx),
		b.squeeze.Off(ctx),
		b.hammer.Off(ctx),
	)
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	box, err := NewResetBox(ctx, myRobot, logger)
	if err != nil {
		return err
	}
	defer box.Close()

	box.home(ctx)
	// err = box.home(ctx)
	// if err != nil {
	// 	return err
	// }

	action.RegisterAction("Run Reset", box.doRunReset)
	action.RegisterAction("Home", box.doHome)
	action.RegisterAction("HomeArm", box.doArmHome)
	action.RegisterAction("PlaceDuck", box.doPlaceDuck)
	action.RegisterAction("Vibrate", box.doVibrateToggle)
	action.RegisterAction("VibeLevel", box.doVibrateLevel)
	action.RegisterAction("TipUp", box.doTipTableUp)
	action.RegisterAction("TipDown", box.doTipTableDown)
	action.RegisterAction("ElevatorUp", box.doElevatorUp)
	action.RegisterAction("ElevatorDown", box.doElevatorDown)
	action.RegisterAction("DuckWhack", box.doDuckWhack)

	action.RegisterAction("Grab1", box.doGrab1)
	action.RegisterAction("Grab2", box.doGrab2)
	action.RegisterAction("Drop1", box.doDrop1)
	action.RegisterAction("Drop2", box.doDrop2)

	webOpts := web.NewOptions()
	webOpts.Insecure = true

	err = webserver.RunWeb(ctx, myRobot, webOpts, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		cancel()
		return err
	}

	return nil
}

func (b *ResetBox) doGrab1(ctx context.Context, r robot.Robot) {
	err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckgrabFW),
	)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doGrab2(ctx context.Context, r robot.Robot) {
	err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckgrabREV),
	)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDrop1(ctx context.Context, r robot.Robot) {
	err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckplaceFW),
	)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDrop2(ctx context.Context, r robot.Robot) {
	err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckplaceREV),
	)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doPlaceDuck(ctx context.Context, r robot.Robot) {
	err := b.placeDuck(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDuckWhack(ctx context.Context, r robot.Robot) {
	err := b.hammerTime(ctx, duckWhacks)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doHome(ctx context.Context, r robot.Robot) {
	err := b.home(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doArmHome(ctx context.Context, r robot.Robot) {
	err := b.armHome(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doRunReset(ctx context.Context, r robot.Robot) {
	err := b.runReset(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doVibrateToggle(ctx context.Context, r robot.Robot) {
	if b.vibeState {
		b.vibrate(ctx, 0)
	} else {
		b.vibrate(ctx, vibeLevel)
	}
}

func (b *ResetBox) doVibrateLevel(ctx context.Context, r robot.Robot) {
	vibeLevel += 16
	if vibeLevel > 192 {
		vibeLevel = 64
	}
	b.vibrate(ctx, vibeLevel)
}

func (b *ResetBox) doTipTableUp(ctx context.Context, r robot.Robot) {
	err := b.tipTableUp(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doTipTableDown(ctx context.Context, r robot.Robot) {
	err := b.tipTableDown(ctx)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doElevatorUp(ctx context.Context, r robot.Robot) {
	err := b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doElevatorDown(ctx context.Context, r robot.Robot) {
	err := b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	if err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) home(ctx context.Context) error {
	b.logger.Info("homing")
	errs := multierr.Combine(
		b.armHome(ctx),
		b.gripper.Open(ctx),
		b.gate.GoTillStop(ctx, backward, 20, nil),
		b.squeeze.GoTillStop(ctx, backward, 20, nil),
		b.elevator.GoTillStop(ctx, backward, 200, nil),
		b.hammer.GoTillStop(ctx, backward, 200, nil),
	)

	errs = multierr.Combine(
		errs,
		b.gate.Zero(ctx, 0),
		b.squeeze.Zero(ctx, 0),
		b.elevator.Zero(ctx, 0),
		b.hammer.Zero(ctx, 0),
	)

	if errs != nil {
		return errs
	}

	// Go to starting positions
	errs = multierr.Combine(
		b.gate.GoTo(ctx, gateSpeed, gateCubePass),
		b.setSqueeze(ctx, squeezeClosed),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.hammer.GoTo(ctx, hammerSpeed, hammerOffset),
	)

	b.waitPosReached(ctx, b.hammer, hammerOffset)
	errs = multierr.Append(errs, b.hammer.Zero(ctx, 0))

	return errs

}

func (b *ResetBox) vibrate(ctx context.Context, level uint8) {
	if level < 32 {
		b.board.PWMSet(ctx, vibePin, 0)
		b.vibeState = false
	} else {
		b.board.PWMSet(ctx, vibePin, level)
		b.vibeState = true
	}
}

func (b *ResetBox) setSqueeze(ctx context.Context, width float64) error {
	target := (squeezeMaxWidth - width) / 2
	if target < 0 {
		target = 0
	}
	return b.squeeze.GoTo(ctx, gateSpeed, target)
}

func (b *ResetBox) waitPosReached(ctx context.Context, motor positional, target float64) error {
	var i int
	for {
		pos, err := motor.Position(ctx)
		if err != nil {
			return err
		}
		on, err := motor.IsOn(ctx)
		if err != nil {
			return err
		}
		if math.Abs(pos-target) < 1.0 && !on {
			return nil
		}
		if i > 100 {
			return errors.New("timed out waiting for position")
		}
		utils.SelectContextOrWait(ctx, 100*time.Millisecond)
		i++
	}
}

func (b *ResetBox) waitFor(ctx context.Context, f func(context.Context) (bool, error)) error {
	var i int
	for {
		if ok, _ := f(ctx); ok {
			return nil
		}
		if i > 100 {
			return errors.New("timed out waiting")
		}
		utils.SelectContextOrWait(ctx, 100*time.Millisecond)
		i++
	}
}

func (b *ResetBox) tipTableUp(ctx context.Context) error {

	if b.tableUp {
		return nil
	}
	err := b.armHome(ctx)
	if err != nil {
		return err
	}

	// Go mostly up
	b.board.GPIOSet(ctx, tipPinA, true)
	b.board.GPIOSet(ctx, tipPinB, false)
	utils.SelectContextOrWait(ctx, 10000*time.Millisecond)

	//All off
	b.board.GPIOSet(ctx, tipPinA, true)
	b.board.GPIOSet(ctx, tipPinB, true)

	b.tableUp = true
	return nil
}

func (b *ResetBox) tipTableDown(ctx context.Context) error {
	b.board.GPIOSet(ctx, tipPinA, false)
	b.board.GPIOSet(ctx, tipPinB, true)
	utils.SelectContextOrWait(ctx, 10000*time.Millisecond)

	//All Off
	b.board.GPIOSet(ctx, tipPinA, true)
	b.board.GPIOSet(ctx, tipPinB, true)

	b.tableUp = false
	return nil
}

func (b *ResetBox) isTableDown(ctx context.Context) (bool, error) {
	return !b.tableUp, nil
}

func (b *ResetBox) isTableUp(ctx context.Context) (bool, error) {
	return b.tableUp, nil
}

func (b *ResetBox) hammerTime(ctx context.Context, count int) error {
	for i := 0.0; i < float64(count); i++ {
		err := b.hammer.GoTo(ctx, hammerSpeed, i+0.2)
		if err != nil {
			return err
		}
		b.waitPosReached(ctx, b.hammer, i+0.2)
		utils.SelectContextOrWait(ctx, 500*time.Millisecond)
	}

	// Raise Hammer
	err := b.hammer.GoTo(ctx, hammerSpeed, float64(count))
	if err != nil {
		return err
	}
	b.waitPosReached(ctx, b.hammer, float64(count))

	// As we go in one direction indefinitely, this is an easy fix for register overflow
	err = b.hammer.Zero(ctx, 0)
	if err != nil {
		return err
	}

	return nil
}

func (b *ResetBox) runReset(ctx context.Context) error {

	errs := multierr.Combine(
		b.gate.GoTo(ctx, gateSpeed, gateCubePass),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.armHome(ctx),
		// Wait for elevator down
		b.waitPosReached(ctx, &b.elevator, elevatorBottom),
		b.setSqueeze(ctx, squeezeCubePass),
		b.tipTableUp(ctx),
		b.waitFor(ctx, b.isTableUp),
	)
	if errs != nil {
		return errs
	}
	b.vibrate(ctx, vibeLevel)

	errC := make(chan error)
	go func() {
		errC <- multierr.Combine(
			b.tipTableDown(ctx),
			b.gripper.Open(ctx),
			b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		)
	}()

	// Three whacks for cubes-behinds-ducks
	errs = b.hammerTime(ctx, cubeWhacks)
	if errs != nil {
		return errs
	}
	// Wait for hammer + 4 seconds
	utils.SelectContextOrWait(ctx, 4000*time.Millisecond)
	// Cubes in, going up
	errs = multierr.Combine(
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
		<-errC,
	)
	if errs != nil {
		return errs
	}

	// DuckWhack
	go func() {
		errs2 := b.hammerTime(ctx, duckWhacks)
		utils.SelectContextOrWait(ctx, 8000*time.Millisecond)
		b.vibrate(ctx, 0)
		errC <- errs2
	}()

	// Cubes at top
	errs = multierr.Combine(
		b.waitPosReached(ctx, &b.elevator, elevatorTop),
		b.waitFor(ctx, b.isTableDown),
	)
	if errs != nil {
		return errs
	}

	errs = b.pickCube1(ctx)
	if errs != nil {
		return errs
	}
	errs = b.placeCube1(ctx)
	if errs != nil {
		return errs
	}
	errs = b.pickCube2(ctx)
	if errs != nil {
		return errs
	}
	errs = <-errC
	if errs != nil {
		return errs
	}

	go func() {
		errC <- multierr.Combine(
			b.placeCube2(ctx),
			b.arm.MoveToJointPositions(ctx, duckReadyPos),
		)
	}()

	errs = multierr.Combine(
		// Back down for duck
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.waitPosReached(ctx, &b.elevator, elevatorBottom),
		// Open to load duck
		b.gate.GoTo(ctx, gateSpeed, gateOpen),
		b.setSqueeze(ctx, squeezeOpen),
	)
	if errs != nil {
		return errs
	}
	b.vibrate(ctx, vibeLevel)
	utils.SelectContextOrWait(ctx, 10000*time.Millisecond)

	// Duck in, silence and up
	b.vibrate(ctx, 0)
	errs = multierr.Combine(
		b.setSqueeze(ctx, squeezeClosed),
		b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeClosed)/2),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
		b.waitPosReached(ctx, &b.elevator, elevatorTop),
		<-errC,
	)
	if errs != nil {
		return errs
	}

	errs = b.placeDuck(ctx)

	// Try again if the duck gets stuck in the squeezer
	if errs != nil && errs.Error() == "missed the duck twice" {
		errs = multierr.Combine(
			// Back down for duck
			b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
			b.gripper.Open(ctx),
			b.waitPosReached(ctx, &b.elevator, elevatorBottom),
			// Open to load duck
			b.gate.GoTo(ctx, gateSpeed, gateOpen),
			b.setSqueeze(ctx, squeezeOpen),
		)
		if errs != nil {
			return errs
		}
		b.vibrate(ctx, vibeLevel)
		utils.SelectContextOrWait(ctx, 10000*time.Millisecond)
		b.vibrate(ctx, 0)
		errs = multierr.Combine(
			b.setSqueeze(ctx, squeezeClosed),
			b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeClosed)/2),
			b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
			b.waitPosReached(ctx, &b.elevator, elevatorTop),
			b.placeDuck(ctx),
		)
	}

	if errs != nil {
		return errs
	}

	return b.armHome(ctx)
}

func (b *ResetBox) armHome(ctx context.Context) error {
	return multierr.Combine(
		b.arm.MoveToJointPositions(ctx, safeDumpPos),
		b.gripper.Open(ctx),
	)
}

func (b *ResetBox) pickCube1(ctx context.Context) error {
	// Grab cube 1 and reset it on the field
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube1grab),
	)
	if errs != nil {
		return errs
	}

	grabbed, errs := b.gripper.Grab(ctx)
	if errs != nil {
		return errs
	}
	if !grabbed {
		return errors.New("missed first cube")
	}
	return b.arm.MoveToJointPositions(ctx, cubeReadyPos)
}

func (b *ResetBox) placeCube1(ctx context.Context) error {
	return multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cube1place),
		b.gripper.Open(ctx),
	)
}

func (b *ResetBox) pickCube2(ctx context.Context) error {
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube2grab),
	)
	if errs != nil {
		return errs
	}
	grabbed, errs := b.gripper.Grab(ctx)
	if errs != nil {
		return errs
	}
	if !grabbed {
		return errors.New("missed second cube")
	}
	return b.arm.MoveToJointPositions(ctx, cubeReadyPos)
}

func (b *ResetBox) placeCube2(ctx context.Context) error {
	return multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cube2place),
		b.gripper.Open(ctx),
	)
}

func (b *ResetBox) placeDuck(ctx context.Context) error {
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckgrabFW),
	)
	if errs != nil {
		return errs
	}

	// Try to grab- this should succeed if the duck is facing forwards, and fail if facing backwards
	grabbed, errs := b.gripper.Grab(ctx)
	if grabbed {
		multierr.Combine(
			errs,
			b.arm.MoveToJointPositions(ctx, duckReadyPos),
			b.arm.MoveToJointPositions(ctx, duckplaceFW),
			b.gripper.Open(ctx),
		)
		if errs != nil {
			return errs
		}

	} else {
		// Duck was facing backwards. Grab where the backwards-facing head should be
		multierr.Combine(
			b.arm.MoveToJointPositions(ctx, duckReadyPos),
			b.gripper.Open(ctx),
			b.arm.MoveToJointPositions(ctx, duckgrabREV),
		)
		if errs != nil {
			return errs
		}

		grabbed, errs := b.gripper.Grab(ctx)
		if errs != nil {
			return errs
		}
		if !grabbed {
			return errors.New("missed the duck twice")
		}
		multierr.Combine(
			b.arm.MoveToJointPositions(ctx, duckReadyPos),
			b.arm.MoveToJointPositions(ctx, duckplaceREV),
			b.gripper.Open(ctx),
		)
	}
	return errs
}
