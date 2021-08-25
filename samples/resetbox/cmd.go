// Package main is a reset box for a robot play area.
package main

import (
	"context"
	"flag"
	"math"
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
func (a *LinearAxis) GoTillStop(ctx context.Context, d pb.DirectionRelative, speed float64) error {
	var homeWorkers sync.WaitGroup
	var errs error
	for _, m := range a.m {
		homeWorkers.Add(1)
		go func(motor board.Motor) {
			defer homeWorkers.Done()
			multierr.AppendInto(&errs, motor.GoTillStop(ctx, d, speed*60/a.mmPerRev))
		}(m)
	}
	homeWorkers.Wait()
	return errs
}

func (a *LinearAxis) Off(ctx context.Context) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Off(ctx))
	}
	return errs
}

func (a *LinearAxis) Zero(ctx context.Context, offset float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Zero(ctx, offset))
	}
	return errs
}

func (a *LinearAxis) Position(ctx context.Context) (float64, error) {
	pos, err := a.m[0].Position(ctx)
	if err != nil {
		return 0, err
	}
	return pos * a.mmPerRev, nil
}

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

type Positional interface {
	Position(ctx context.Context) (float64, error)
	IsOn(ctx context.Context) (bool, error)
}

type ResetBox struct {
	logger        golog.Logger
	board         board.Board
	gate, squeeze LinearAxis
	elevator      LinearAxis
	hammer        board.Motor

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

	hammer, ok := b.board.MotorByName("hammer")
	if !ok {
		return nil, errors.New("Can't find motor named: hammer")
	}
	b.hammer = hammer

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

	box.home(ctx, myRobot)

	action.RegisterAction("Home", box.home)
	action.RegisterAction("Vibrate", box.vibrateToggle)
	action.RegisterAction("TipUp", box.tipTableUp)
	action.RegisterAction("TipDown", box.tipTableDown)

	action.RegisterAction("FullReset", box.doReset)

	action.RegisterAction("DuckWhack", box.doWhack)

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
	b.logger.Info("homing")
	errs := multierr.Combine(
		b.gate.GoTillStop(ctx, backward, 20),
		b.squeeze.GoTillStop(ctx, backward, 20),
		b.elevator.GoTillStop(ctx, backward, 200),
		b.hammer.GoTillStop(ctx, backward, 200),
	)

	errs = multierr.Combine(
		b.gate.Zero(ctx, 0),
		b.squeeze.Zero(ctx, 0),
		b.elevator.Zero(ctx, 0),
		b.hammer.Zero(ctx, 0),
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

	b.waitPosReached(ctx, b.hammer, hammerOffset*hammerRatio)
	errs = multierr.Append(errs, b.hammer.Zero(ctx, 0))

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

func (b *ResetBox) waitPosReached(ctx context.Context, motor Positional, target float64) error {
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

func (b *ResetBox) tipTableUp(ctx context.Context, r robot.Robot) {

	if b.tableUp {
		return
	}

	// Go mostly up
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, false)
	utils.SelectContextOrWait(ctx, 10000*time.Millisecond)

	//All off
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, true)

	b.tableUp = true
}

func (b *ResetBox) tipTableDown(ctx context.Context, r robot.Robot) {
	b.board.GPIOSet(tipPinA, false)
	b.board.GPIOSet(tipPinB, true)
	utils.SelectContextOrWait(ctx, 10000*time.Millisecond)

	//All Off
	b.board.GPIOSet(tipPinA, true)
	b.board.GPIOSet(tipPinB, true)

	b.tableUp = false
}

func (b *ResetBox) isTableDown(ctx context.Context) (bool, error) {
	return !b.tableUp, nil
}

func (b *ResetBox) isTableUp(ctx context.Context) (bool, error) {
	return b.tableUp, nil
}

func (b *ResetBox) hammerTime(ctx context.Context, count int) error {
	for i := 0.0; i < float64(count); i++ {
		err := b.hammer.GoTo(ctx, hammerSpeed*hammerRatio, (i+0.2)*hammerRatio)
		if err != nil {
			return err
		}
		b.waitPosReached(ctx, b.hammer, (i+0.2)*hammerRatio)
		utils.SelectContextOrWait(ctx, 500*time.Millisecond)
	}

	// Raise Hammer
	err := b.hammer.GoTo(ctx, hammerSpeed*hammerRatio, float64(count)*hammerRatio)
	if err != nil {
		return err
	}
	b.waitPosReached(ctx, b.hammer, float64(count)*hammerRatio)

	// As we go in one direction indefinitely, this is an easy fix for register overflow
	err = b.hammer.Zero(ctx, 0)
	if err != nil {
		return err
	}

	return nil
}

func (b *ResetBox) doWhack(ctx context.Context, r robot.Robot) {
	b.hammerTime(ctx, duckWhacks)
}

func (b *ResetBox) doReset(ctx context.Context, r robot.Robot) {
	b.gate.GoTo(ctx, gateSpeed, gateCubePass)
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)

	// Wait for elevator down
	b.waitPosReached(ctx, &b.elevator, elevatorBottom)
	b.setSqueeze(ctx, squeezeCubePass)

	// WAIT ROBOT for tip command
	b.vibrate(ctx, vibeLevel)
	b.tipTableUp(ctx, r)
	b.waitFor(ctx, b.isTableUp)
	go b.tipTableDown(ctx, r)

	// Three whacks for cubes-behinds-ducks
	b.hammerTime(ctx, cubeWhacks)

	// Wait for hammer + 4 seconds
	utils.SelectContextOrWait(ctx, 4000*time.Millisecond)

	// Cubes in, going up
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)

	// DuckWhack
	b.hammerTime(ctx, duckWhacks)

	// Cubes at top
	b.waitPosReached(ctx, &b.elevator, elevatorTop)
	b.waitFor(ctx, b.isTableDown)
	// WAIT ROBOT

	// Back down for duck
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	b.waitPosReached(ctx, &b.elevator, elevatorBottom)

	// Open to load duck
	b.gate.GoTo(ctx, gateSpeed, gateOpen)
	b.setSqueeze(ctx, squeezeOpen)
	utils.SelectContextOrWait(ctx, 8000*time.Millisecond)

	// Duck in, silence and up
	b.vibrate(ctx, 0)
	b.setSqueeze(ctx, squeezeClosed)
	b.waitPosReached(ctx, &b.squeeze, (squeezeMaxWidth-squeezeClosed)/2)
	b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)
	b.waitPosReached(ctx, &b.elevator, elevatorTop)

	// WAIT ROBOT

}
