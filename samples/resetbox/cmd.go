// Package main is a reset box for a robot play area.
package main

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"

	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"

	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const (
	forward  = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	backward = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD

	tipPinA = "29"
	tipPinB = "31"
	vibePin = "33"

	vibeLevel = 96

	gateSpeed    = 25
	gateCubePass = 50
	gateOpen     = 80

	squeezeMaxWidth = 183
	squeezeClosed   = 40
	squeezeCubePass = 55
	squeezeOpen     = 80

	elevatorBottom = 75
	elevatorTop    = 800
	elevatorSpeed  = 300

	hammerSpeed  = 25.0
	hammerOffset = 0.9
	hammerRatio  = 11.8455 // 26.85:1 motor + 30/68 teeth gears
	cubeWhacks   = 3.0
	duckWhacks   = 5.0
)

var logger = golog.NewDevelopmentLogger("resetbox")

type LinearAxis struct {
	m        []board.Motor
	mmPerRev float64
}

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
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		multierr.AppendInto(&errs, raw.GoTo(ctx, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// GoFor moves for the distance specified in mm and at a speed in mm/s
func (a *LinearAxis) GoFor(ctx context.Context, d pb.DirectionRelative, speed float64, position float64) error {
	var errs error
	for _, m := range a.m {
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		multierr.AppendInto(&errs, raw.GoFor(ctx, d, speed*60/a.mmPerRev, position/a.mmPerRev))
	}
	return errs
}

// Home simultaneously homes all motors on an axis
func (a *LinearAxis) Home(ctx context.Context, d pb.DirectionRelative, speed float64) error {
	var homeWorkers sync.WaitGroup
	var errs error
	for _, m := range a.m {
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		homeWorkers.Add(1)
		utils.ManagedGo(func() {
			multierr.AppendInto(&errs, raw.Home(ctx, d, speed*60/a.mmPerRev))
		}, homeWorkers.Done)
	}
	homeWorkers.Wait()
	return errs
}

func (a *LinearAxis) Off(ctx context.Context) error {
	var errs error
	for _, m := range a.m {
		if m == nil {
			continue
		}
		multierr.AppendInto(&errs, m.Off(ctx))
	}
	return errs
}

func (a *LinearAxis) PositionReached(ctx context.Context) bool {
	for _, m := range a.m {
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		if !raw.PositionReached(ctx) {
			return false
		}
	}
	return true
}

type ResetBox struct {
	logger        golog.Logger
	board         board.Board
	gate, squeeze LinearAxis
	elevator      LinearAxis
	hammer        *board.TMCStepperMotor

	activeBackgroundWorkers *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context

	vibeState bool
	tableUp   bool
}

func NewResetBox(ctx context.Context, r robot.Robot, logger golog.Logger) (*ResetBox, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	b := &ResetBox{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel, logger: logger}
	resetboard, ok := r.BoardByName("resetboard")
	if !ok {
		return nil, errors.New("Cannot find board: resetboard")
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

	baseHammer, ok := b.board.MotorByName("hammer")
	if !ok {
		return nil, errors.New("Can't find motor named: hammer")
	}
	b.hammer = baseHammer.GetRaw(cancelCtx).(*board.TMCStepperMotor)

	return b, nil
}

// Close TODO
func (b *ResetBox) Close() error {
	defer b.activeBackgroundWorkers.Wait()
	b.cancel()
	return nil // b.Stop(context.Background())
}

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

func (b *ResetBox) Ready(r robot.Robot) error {
	return nil
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

	action.RegisterAction("Home", box.home)
	action.RegisterAction("Vibrate", box.vibrateToggle)
	action.RegisterAction("TipUp", box.tipTableUp)
	action.RegisterAction("TipDown", box.tipTableDown)

	action.RegisterAction("FullReset", box.doReset)

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

func (b *ResetBox) home(ctx context.Context, r robot.Robot) {
	errs := multierr.Combine(
		b.gate.Home(ctx, backward, 20),
		b.squeeze.Home(ctx, backward, 20),
		b.elevator.Home(ctx, backward, 200),
		b.hammer.Home(ctx, backward, 200),
	)

	if errs != nil {
		b.logger.Error(errs)
	}

	// Go to starting positions
	errs = multierr.Combine(
		b.gate.GoTo(ctx, gateSpeed, gateCubePass),
		b.setSqueeze(ctx, squeezeClosed),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		b.hammer.GoTo(ctx, hammerSpeed*hammerRatio, hammerOffset*hammerRatio),
	)

	if errs != nil {
		b.logger.Error(errs)
	}

}

func (b *ResetBox) vibrate(ctx context.Context, level uint8) {
	if level < 32 {
		b.board.PWMSet(vibePin, 0)
		b.vibeState = false
	} else {
		b.board.PWMSet(vibePin, level)
		b.vibeState = true
	}
}

func (b *ResetBox) vibrateToggle(ctx context.Context, r robot.Robot) {
	if b.vibeState {
		b.vibrate(ctx, 0)
	} else {
		b.vibrate(ctx, vibeLevel)
	}
}

func (b *ResetBox) setSqueeze(ctx context.Context, width float64) error {
	target := (squeezeMaxWidth - width) / 2
	if target < 0 {
		target = 0
	}
	return b.squeeze.GoTo(ctx, gateSpeed, target)
}

func (b *ResetBox) sleep(ctx context.Context, millis time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(millis * time.Millisecond):
	}
}

func (b *ResetBox) waitFor(ctx context.Context, f func(context.Context) bool) error {
	for {
		select {
		case <-ctx.Done():
			return errors.New("Context cancelled while waiting")
		case <-time.After(100 * time.Millisecond):
		}
		if f(ctx) {
			return nil
		}
	}
}

func (b *ResetBox) tipTableUp(ctx context.Context, r robot.Robot) {

	if b.tableUp {
		return
	}

	// Go mostly up
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, false)
	b.sleep(ctx, 10000)

	//All off
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, true)

	b.tableUp = true
}

func (b *ResetBox) tipTableDown(ctx context.Context, r robot.Robot) {
	b.board.GPIOSet(tipPinA, false)
	b.board.GPIOSet(tipPinB, true)
	b.sleep(ctx, 10000)

	//All Off
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, true)

	b.tableUp = false
}

func (b *ResetBox) isTableDown(ctx context.Context) bool {
	return !b.tableUp
}

func (b *ResetBox) isTableUp(ctx context.Context) bool {
	return b.tableUp
}

func (b *ResetBox) doReset(ctx context.Context, r robot.Robot) {
	b.gate.GoTo(ctx, gateSpeed, gateCubePass)
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)

	// Wait for elevator down
	b.waitFor(ctx, b.elevator.PositionReached)
	b.setSqueeze(ctx, squeezeCubePass)

	// WAIT ROBOT for tip command
	b.vibrate(ctx, vibeLevel)
	b.tipTableUp(ctx, r)
	b.waitFor(ctx, b.isTableUp)
	b.tipTableDown(ctx, r)

	// Three whacks for cubes-behinds-ducks
	b.hammer.GoFor(ctx, forward, hammerSpeed*hammerRatio, cubeWhacks*hammerRatio)
	b.waitFor(ctx, b.hammer.PositionReached)

	// Wait for hammer + 4 seconds
	b.sleep(ctx, 4000)

	// Cubes in, going up
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)

	// DuckWhack
	b.hammer.GoFor(ctx, forward, hammerSpeed*hammerRatio, duckWhacks*hammerRatio)

	// Cubes at top
	b.waitFor(ctx, b.elevator.PositionReached)
	b.waitFor(ctx, b.isTableDown)
	// WAIT ROBOT

	// Back down for duck
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	b.waitFor(ctx, b.elevator.PositionReached)
	b.waitFor(ctx, b.hammer.PositionReached)

	// Open to load duck
	b.gate.GoTo(ctx, gateSpeed, gateOpen)
	b.setSqueeze(ctx, squeezeOpen)
	b.sleep(ctx, 8000)

	// Duck in, silence and up
	b.vibrate(ctx, 0)
	b.setSqueeze(ctx, squeezeClosed)
	b.waitFor(ctx, b.squeeze.PositionReached)
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)
	b.waitFor(ctx, b.elevator.PositionReached)

	// WAIT ROBOT

	// As we go in one direction indefinitely, this is an easy fix for register overflow
	b.hammer.GetRaw(ctx).(*board.TMCStepperMotor).Zero(ctx)

}
