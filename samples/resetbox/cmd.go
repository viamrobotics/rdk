// Package main is a reset box for a robot play area.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/web"
	webserver "go.viam.com/rdk/web/server"
)

const (
	gateSpeed    = 200
	gateCubePass = 50
	gateClosed   = 30
	gateOpen     = 80

	squeezeMaxWidth = 183
	squeezeClosed   = 40
	duckSquish      = 5
	squeezeCubePass = 64
	squeezeOpen     = 80

	elevatorBottom = 27
	elevatorTop    = 850
	elevatorSpeed  = 800

	hammerSpeed  = 20 // May be capped by underlying motor MaxRPM
	hammerOffset = 0.9
	cubeWhacks   = 2.0
	duckWhacks   = 6.0

	armName     = "xArm6"
	gripperName = "vg1"
)

var (
	vibeLevel = float64(0.7)

	safeDumpPos  = &componentpb.ArmJointPositions{Degrees: []float64{0, -43, -71, 0, 98, 0}}
	cubeReadyPos = &componentpb.ArmJointPositions{Degrees: []float64{-182.6, -26.8, -33.0, 0, 51.0, 0}}
	cube1grab    = &componentpb.ArmJointPositions{Degrees: []float64{-182.6, 11.2, -51.8, 0, 48.6, 0}}
	cube2grab    = &componentpb.ArmJointPositions{Degrees: []float64{-182.6, 7.3, -36.9, 0, 17.6, 0}}

	cube1place = &componentpb.ArmJointPositions{Degrees: []float64{50, 20, -35, -0.5, 3.0, 0}}
	cube2place = &componentpb.ArmJointPositions{Degrees: []float64{-130, 30.5, -28.7, -0.5, -32.2, 0}}

	duckgrabFW   = &componentpb.ArmJointPositions{Degrees: []float64{-180.5, 27.7, -79.7, -2.8, 76.20, 180}}
	duckgrabREV  = &componentpb.ArmJointPositions{Degrees: []float64{-180.5, 28.3, -76.8, -2.8, 65.45, 180}}
	duckReadyPos = &componentpb.ArmJointPositions{Degrees: []float64{-180.5, 0.0, -60.0, -2.8, 65.45, 180}}

	duckplaceFW  = &componentpb.ArmJointPositions{Degrees: []float64{-21.3, 14.9, -39.0, 6.8, 22.0, 49.6}}
	duckplaceREV = &componentpb.ArmJointPositions{Degrees: []float64{-19.2, 18, -41.0, 6.3, 22.7, 230}}
)

var logger = golog.NewDevelopmentLogger("resetbox")

// LinearAxis is one or more motors whose motion is converted to linear movement via belts, screw drives, etc.
type LinearAxis struct {
	m        []motor.Motor
	mmPerRev float64
}

// AddMotors takes a slice of motor names and adds them to the axis.
func (a *LinearAxis) AddMotors(_ context.Context, robot robot.Robot, names []string) error {
	for _, n := range names {
		motor, ok := robot.MotorByName(n)
		if ok {
			a.m = append(a.m, motor)
		} else {
			return errors.Errorf("Cannot find motor named \"%s\"", n)
		}
	}
	return nil
}

// GoTo moves to a position specified in mm and at a speed in mm/s.
func (a *LinearAxis) GoTo(ctx context.Context, speed float64, position float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.GoTo(ctx, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// GoFor moves for the distance specified in mm and at a speed in mm/s.
func (a *LinearAxis) GoFor(ctx context.Context, speed float64, position float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.GoFor(ctx, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// GoTillStop simultaneously homes all motors on an axis.
func (a *LinearAxis) GoTillStop(ctx context.Context, speed float64, _ func(ctx context.Context) bool) error {
	var homeWorkers sync.WaitGroup
	var errs error
	for _, m := range a.m {
		homeWorkers.Add(1)
		go func(motor motor.Motor) {
			defer homeWorkers.Done()
			multierr.AppendInto(&errs, motor.GoTillStop(ctx, speed*60/a.mmPerRev, nil))
		}(m)
	}
	homeWorkers.Wait()
	return errs
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (a *LinearAxis) Stop(ctx context.Context) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Stop(ctx))
	}
	return errs
}

// ResetZeroPosition resets the "home" point.
func (a *LinearAxis) ResetZeroPosition(ctx context.Context, offset float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.ResetZeroPosition(ctx, offset))
	}
	return errs
}

// Position returns the position of the first motor in the axis.
func (a *LinearAxis) Position(ctx context.Context) (float64, error) {
	pos, err := a.m[0].Position(ctx)
	if err != nil {
		return 0, err
	}
	return pos * a.mmPerRev, nil
}

// IsOn returns true if moving.
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

// ResetBox is the parent structure for this project.
type ResetBox struct {
	io.Closer
	logger golog.Logger
	// board                    board.Board
	gate, squeeze            LinearAxis
	elevator                 LinearAxis
	hammer, tipper, vibrator motor.Motor
	arm                      arm.Arm
	gripper                  gripper.Gripper

	activeBackgroundWorkers *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context

	vibeState bool
	tableUp   bool
	haveHomed bool
}

// NewResetBox returns a ResetBox.
func NewResetBox(ctx context.Context, r robot.Robot, logger golog.Logger) (*ResetBox, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	b := &ResetBox{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel, logger: logger}
	// resetboard, ok := r.BoardByName("resetboard")
	// if !ok {
	// 	return nil, errors.New("can't find board: resetboard")
	// }
	// b.board = resetboard
	b.gate.mmPerRev = 8.0
	b.squeeze.mmPerRev = 8.0
	b.elevator.mmPerRev = 60.0

	err := multierr.Combine(
		b.gate.AddMotors(cancelCtx, r, []string{"gateL", "gateR"}),
		b.squeeze.AddMotors(cancelCtx, r, []string{"squeezeL", "squeezeR"}),
		b.elevator.AddMotors(cancelCtx, r, []string{"elevator"}),
	)
	if err != nil {
		return nil, err
	}

	hammer, ok := r.MotorByName("hammer")
	if !ok {
		return nil, errors.New("can't find motor named: hammer")
	}
	b.hammer = hammer

	tipper, ok := r.MotorByName("tipper")
	if !ok {
		return nil, errors.New("can't find motor named: tipper")
	}
	b.tipper = tipper

	vibrator, ok := r.MotorByName("vibrator")
	if !ok {
		return nil, errors.New("can't find motor named: vibrator")
	}
	b.vibrator = vibrator

	rArm, ok := arm.FromRobot(r, armName)
	if !ok {
		return nil, fmt.Errorf("failed to find arm %s", armName)
	}
	b.arm = rArm

	rGripper, ok := gripper.FromRobot(r, gripperName)
	if !ok {
		return nil, fmt.Errorf("failed to find gripper %s", gripperName)
	}
	b.gripper = rGripper

	return b, nil
}

// Close stops motors and cancels context.
func (b *ResetBox) Close() {
	defer b.activeBackgroundWorkers.Wait()
	b.Stop(b.cancelCtx)
	b.cancel()
}

// Stop turns off all motors.
func (b *ResetBox) Stop(ctx context.Context) error {
	return multierr.Combine(
		b.elevator.Stop(ctx),
		b.gate.Stop(ctx),
		b.gate.Stop(ctx),
		b.squeeze.Stop(ctx),
		b.squeeze.Stop(ctx),
		b.hammer.Stop(ctx),
	)
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(ctx, flag.Arg(0))
	if err != nil {
		return err
	}

	myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

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

	action.RegisterAction("GrabD1", box.doGrab1)
	action.RegisterAction("GrabD2", box.doGrab2)
	action.RegisterAction("DropD1", box.doDrop1)
	action.RegisterAction("DropD2", box.doDrop2)
	action.RegisterAction("GrabC1", box.doGrabC1)
	action.RegisterAction("GrabC2", box.doGrabC2)
	action.RegisterAction("DropC1", box.doDropC1)
	action.RegisterAction("DropC2", box.doDropC2)

	return webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger)
}

func (b *ResetBox) doGrab1(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckgrabFW),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doGrab2(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckgrabREV),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doGrabC1(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube1grab),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doGrabC2(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube2grab),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDropC1(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube1place),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDropC2(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		b.arm.MoveToJointPositions(ctx, cube2place),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDrop1(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckplaceFW),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDrop2(ctx context.Context, r robot.Robot) {
	if err := multierr.Combine(
		b.arm.MoveToJointPositions(ctx, duckReadyPos),
		b.arm.MoveToJointPositions(ctx, duckplaceREV),
	); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doPlaceDuck(ctx context.Context, r robot.Robot) {
	if err := b.placeDuck(ctx); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doDuckWhack(ctx context.Context, r robot.Robot) {
	if err := b.hammerTime(ctx, duckWhacks); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doHome(ctx context.Context, r robot.Robot) {
	if err := b.home(ctx); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doArmHome(ctx context.Context, r robot.Robot) {
	if err := b.armHome(ctx); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doRunReset(ctx context.Context, r robot.Robot) {
	if err := b.runReset(ctx); err != nil {
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
	vibeLevel += 0.1
	if vibeLevel >= 1.1 {
		vibeLevel = 0.2
	}
	b.logger.Debugf("Vibe Level: %f", vibeLevel)
	b.vibrate(ctx, vibeLevel)
}

func (b *ResetBox) doTipTableUp(ctx context.Context, r robot.Robot) {
	if err := b.tipTableUp(ctx); err != nil {
		b.logger.Error(err)
	}
}

func (b *ResetBox) doTipTableDown(ctx context.Context, r robot.Robot) {
	if err := b.tipTableDown(ctx); err != nil {
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
	errPath := make(chan error)

	go func() {
		errPath <- b.armHome(ctx)
		errPath <- b.gripper.Open(ctx)
	}()
	go func() {
		errPath <- b.gate.GoTillStop(ctx, -20, nil)
		errPath <- b.hammer.GoTillStop(ctx, -200, nil)
	}()
	go func() {
		errPath <- b.squeeze.GoTillStop(ctx, -20, nil)
	}()
	go func() {
		errPath <- b.elevator.GoTillStop(ctx, -200, nil)
	}()

	errs := multierr.Combine(
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		b.gate.ResetZeroPosition(ctx, 0),
		b.squeeze.ResetZeroPosition(ctx, 0),
		b.elevator.ResetZeroPosition(ctx, 0),
		b.hammer.ResetZeroPosition(ctx, 0),
	)

	if errs != nil {
		return errs
	}

	// Go to starting positions
	errs = multierr.Combine(
		b.gate.GoTo(ctx, gateSpeed, gateClosed),
		b.setSqueeze(ctx, squeezeClosed),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.hammer.GoTo(ctx, hammerSpeed, hammerOffset),
		b.waitPosReached(ctx, b.hammer, hammerOffset),
		b.hammer.ResetZeroPosition(ctx, 0),
	)

	if errs != nil {
		return errs
	}
	b.haveHomed = true
	return nil
}

func (b *ResetBox) vibrate(ctx context.Context, level float64) {
	if level < 0.2 {
		b.vibrator.Stop(ctx)
		b.vibeState = false
	} else {
		b.vibrator.Go(ctx, level)
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
	if err := b.armHome(ctx); err != nil {
		return err
	}

	// Go mostly up
	b.tipper.Go(ctx, 1.0)
	utils.SelectContextOrWait(ctx, 11000*time.Millisecond)

	// All off
	b.tipper.Stop(ctx)

	b.tableUp = true
	return nil
}

func (b *ResetBox) tipTableDown(ctx context.Context) error {
	if err := b.tipper.Go(ctx, -1.0); err != nil {
		return err
	}
	if !utils.SelectContextOrWait(ctx, 10000*time.Millisecond) {
		return ctx.Err()
	}

	// Trigger this when we SHOULD be down.
	b.tableUp = false

	// Extra time for safety (actuator automatically stops on retract)
	if !utils.SelectContextOrWait(ctx, 4000*time.Millisecond) {
		return ctx.Err()
	}
	// All Off
	return b.tipper.Stop(ctx)
}

func (b *ResetBox) isTableDown(ctx context.Context) (bool, error) {
	return !b.tableUp, nil
}

func (b *ResetBox) isTableUp(ctx context.Context) (bool, error) {
	return b.tableUp, nil
}

func (b *ResetBox) hammerTime(ctx context.Context, count int) error {
	if !b.haveHomed {
		return errors.New("must successfully home first")
	}

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
	err = b.hammer.ResetZeroPosition(ctx, 0)
	if err != nil {
		return err
	}

	return nil
}

func (b *ResetBox) runReset(ctx context.Context) error {
	if !b.haveHomed {
		return errors.New("must successfully home first")
	}

	defer b.vibrate(ctx, 0)

	errArm := make(chan error)
	errHammer := make(chan error)
	errMisc := make(chan error)

	errs := multierr.Combine(
		b.setSqueeze(ctx, squeezeClosed),
		b.gate.GoTo(ctx, gateSpeed, gateCubePass),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.tipTableUp(ctx),
		// Wait for elevator down
		b.waitPosReached(ctx, &b.elevator, elevatorBottom),
		b.waitFor(ctx, b.isTableUp),
	)
	if errs != nil {
		return errs
	}

	go func() {
		errMisc <- b.tipTableDown(ctx)
	}()

	go func() {
		errArm <- multierr.Combine(
			b.gripper.Open(ctx),
			b.waitFor(ctx, b.isTableDown),
			b.arm.MoveToJointPositions(ctx, cubeReadyPos),
		)
	}()

	// utils.SelectContextOrWait(ctx, 2000*time.Millisecond)
	// Three whacks for cubes-behinds-ducks
	errs = multierr.Combine(
		b.hammerTime(ctx, cubeWhacks),
		b.setSqueeze(ctx, squeezeCubePass),
	)
	if errs != nil {
		return errs
	}

	// DuckWhack
	go func() {
		errHammer <- b.hammerTime(ctx, duckWhacks)
	}()

	b.vibrate(ctx, vibeLevel)
	errs = b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeCubePass)/2)
	if errs != nil {
		return errs
	}
	utils.SelectContextOrWait(ctx, 1000*time.Millisecond)
	b.vibrate(ctx, 0)

	errs = <-errArm
	if errs != nil {
		return errs
	}

	// Cubes in, going up
	errs = multierr.Combine(
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
		b.waitPosReached(ctx, &b.elevator, elevatorTop),
		<-errMisc,
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

	go func() {
		errL := b.placeCube2(ctx)
		if errL != nil {
			errArm <- errL
			return
		}
		errArm <- b.arm.MoveToJointPositions(ctx, duckReadyPos)
	}()

	// Back down for duck
	errs = multierr.Combine(
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		<-errHammer,
		b.waitPosReached(ctx, &b.elevator, elevatorBottom),
		b.gate.GoTo(ctx, gateSpeed, gateOpen),
		b.waitPosReached(ctx, &b.gate, gateOpen),
	)
	if errs != nil {
		return errs
	}
	utils.SelectContextOrWait(ctx, 1000*time.Millisecond)
	// Open to load duck
	b.vibrate(ctx, vibeLevel)
	errs = multierr.Combine(
		b.setSqueeze(ctx, squeezeOpen),
		b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeOpen)/2),
		<-errArm,
	)
	utils.SelectContextOrWait(ctx, 1000*time.Millisecond)
	// Duck in, silence and up
	b.vibrate(ctx, 0)
	if errs != nil {
		return errs
	}
	errs = multierr.Combine(
		b.setSqueeze(ctx, squeezeClosed),
		b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeClosed)/2),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
		b.waitPosReached(ctx, &b.elevator, elevatorTop),
	)
	if errs != nil {
		return errs
	}

	errs = b.placeDuck(ctx)
	// Try again if the duck gets stuck in the squeezer
	if errs != nil && errs.Error() == "missed the duck twice" {
		go func() {
			errArm <- b.gripper.Open(ctx)
			errArm <- b.arm.MoveToJointPositions(ctx, duckReadyPos)
		}()
		errs = multierr.Combine(
			// Squish to reorient if possible
			b.setSqueeze(ctx, duckSquish),
			// Back down for duck
			b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
			b.waitPosReached(ctx, &b.elevator, elevatorBottom),
		)
		if errs != nil {
			return errs
		}
		// Open to load duck
		b.vibrate(ctx, vibeLevel)
		errs = multierr.Combine(
			b.setSqueeze(ctx, squeezeOpen),
			b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeOpen)/2),
			<-errArm,
			<-errArm,
		)
		if errs != nil {
			return errs
		}
		utils.SelectContextOrWait(ctx, 3000*time.Millisecond)
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
		b.arm.MoveToJointPositions(ctx, safeDumpPos),
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
