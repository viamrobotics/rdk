// Package main is a remote client to coordinate a resetbox (robot play area.)
package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/gripper/vgripper/v1"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/tmcstepper"
	"go.viam.com/rdk/grpc/client"
	componentpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
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
	elevatorTop    = 855
	elevatorSpeed  = 800

	hammerSpeed  = 20 // May be capped by underlying motor MaxRPM
	hammerOffset = 0.9
	cubeWhacks   = 2.0
	duckWhacks   = 6.0

	armName     = "xArm6"
	gripperName = "vg1"

	moment = 500 * time.Millisecond
)

var (
	vibeLevel = float64(0.7)

	safeDumpPos  = &componentpb.JointPositions{Values: []float64{0, -43, -71, 0, 98, 0}}
	cubeReadyPos = &componentpb.JointPositions{Values: []float64{-182.6, -26.8, -33.0, 0, 51.0, 0}}
	cube1grab    = &componentpb.JointPositions{Values: []float64{-182.6, 11.2, -51.8, 0, 48.6, 0}}
	cube2grab    = &componentpb.JointPositions{Values: []float64{-182.6, 7.3, -36.9, 0, 17.6, 0}}
	cube1place   = &componentpb.JointPositions{Values: []float64{50, 20, -35, -0.5, 3.0, 0}}
	cube2place   = &componentpb.JointPositions{Values: []float64{-130, 30.5, -28.7, -0.5, -32.2, 0}}
	duckgrabFW   = &componentpb.JointPositions{Values: []float64{-180.5, 27.7, -79.7, -2.8, 76.20, 180}}
	duckgrabREV  = &componentpb.JointPositions{Values: []float64{-180.5, 28.3, -76.8, -2.8, 65.45, 180}}
	duckReadyPos = &componentpb.JointPositions{Values: []float64{-180.5, 0.0, -60.0, -2.8, 65.45, 180}}
	duckplaceFW  = &componentpb.JointPositions{Values: []float64{-21.3, 14.9, -39.0, 6.8, 22.0, 49.6}}
	duckplaceREV = &componentpb.JointPositions{Values: []float64{-19.2, 18, -41.0, 6.3, 22.7, 230}}
)

// LinearAxis is one or more motors whose motion is converted to linear movement via belts, screw drives, etc.
type LinearAxis struct {
	m        []motor.Motor
	mmPerRev float64
}

// AddMotors takes a slice of motor names and adds them to the axis.
func (a *LinearAxis) AddMotors(robot robot.Robot, names []string) error {
	for _, n := range names {
		_motor, err := motor.FromRobot(robot, n)
		if err == nil {
			a.m = append(a.m, _motor)
		} else {
			return err
		}
	}
	return nil
}

// GoTo moves to a position specified in mm and at a speed in mm/s.
func (a *LinearAxis) GoTo(ctx context.Context, speed float64, position float64) error {
	var errs error
	errPath := make(chan error, len(a.m))
	for _, m := range a.m {
		go func(m motor.Motor) {
			errPath <- m.GoTo(ctx, speed*60/a.mmPerRev, position/a.mmPerRev, nil)
		}(m)
	}
	for range a.m {
		multierr.AppendInto(&errs, <-errPath)
	}
	return errs
}

// GoFor moves for the distance specified in mm and at a speed in mm/s.
func (a *LinearAxis) GoFor(ctx context.Context, speed float64, position float64) error {
	var errs error
	errPath := make(chan error, len(a.m))
	for _, m := range a.m {
		go func(m motor.Motor) {
			errPath <- m.GoFor(ctx, speed*60/a.mmPerRev, position/a.mmPerRev, nil)
		}(m)
	}
	for range a.m {
		multierr.AppendInto(&errs, <-errPath)
	}
	return errs
}

// Home simultaneously homes all motors on an axis.
func (a *LinearAxis) Home(ctx context.Context) error {
	var errs error
	errPath := make(chan error, len(a.m))
	for _, m := range a.m {
		go func(m motor.Motor) {
			_, err := m.Do(ctx, map[string]interface{}{tmcstepper.Command: tmcstepper.Home})
			errPath <- err
		}(m)
	}
	for range a.m {
		multierr.AppendInto(&errs, <-errPath)
	}
	return errs
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (a *LinearAxis) Stop(ctx context.Context) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.Stop(ctx, nil))
	}
	return errs
}

// ResetZeroPosition resets the "home" point.
func (a *LinearAxis) ResetZeroPosition(ctx context.Context, offset float64) error {
	var errs error
	for _, m := range a.m {
		multierr.AppendInto(&errs, m.ResetZeroPosition(ctx, offset, nil))
	}
	return errs
}

// GetPosition returns the position of the first motor in the axis.
func (a *LinearAxis) GetPosition(ctx context.Context) (float64, error) {
	pos, err := a.m[0].GetPosition(ctx, nil)
	if err != nil {
		return 0, err
	}
	return pos * a.mmPerRev, nil
}

// IsPowered returns true if moving.
func (a *LinearAxis) IsPowered(ctx context.Context) (bool, error) {
	var errs error
	for _, m := range a.m {
		on, err := m.IsPowered(ctx, nil)
		multierr.AppendInto(&errs, err)
		if on {
			return true, errs
		}
	}
	return false, errs
}

// ResetBox is the parent structure for this project.
type ResetBox struct {
	io.Closer
	logger golog.Logger
	// board                    board.Board
	gate, squeeze    LinearAxis
	elevator         LinearAxis
	tipper, vibrator motor.Motor
	hammer           motor.Motor
	arm              arm.Arm
	gripper          gripper.Gripper

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
	// resetboard, err := board.FromRobot(r,"resetboard")
	// if err != nil {
	// 	return nil, err
	// }
	// b.board = resetboard
	b.gate.mmPerRev = 8.0
	b.squeeze.mmPerRev = 8.0
	b.elevator.mmPerRev = 60.0

	err := multierr.Combine(
		b.gate.AddMotors(r, []string{"gateL", "gateR"}),
		b.squeeze.AddMotors(r, []string{"squeezeL", "squeezeR"}),
		b.elevator.AddMotors(r, []string{"elevator"}),
	)
	if err != nil {
		return nil, err
	}

	hammer, err := motor.FromRobot(r, "hammer")
	if err != nil {
		return nil, err
	}
	b.hammer = hammer

	tipper, err := motor.FromRobot(r, "tipper")
	if err != nil {
		return nil, err
	}
	b.tipper = tipper

	vibrator, err := motor.FromRobot(r, "vibrator")
	if err != nil {
		return nil, err
	}
	b.vibrator = vibrator

	rArm, err := arm.FromRobot(r, armName)
	if err != nil {
		return nil, err
	}
	b.arm = rArm

	rGripper, err := gripper.FromRobot(r, gripperName)
	if err != nil {
		return nil, err
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
		b.hammer.Stop(ctx, nil),
	)
}

type arguments struct {
	Address string `flag:"0,required,usage=Address of Resetbox (RDK server) to connect to"`
	Secret  string `flag:"secret,usage=Location secret if connecting securely"`
}

func main() {
	logger := golog.NewDevelopmentLogger("resetbox")
	ctx := context.Background()
	var argsParsed arguments
	if err := utils.ParseFlags(os.Args, &argsParsed); err != nil {
		return
	}

	dialOpts := rpc.WithInsecure()
	if argsParsed.Secret != "" {
		dialOpts = rpc.WithCredentials(rpc.Credentials{
			Type:    rdkutils.CredentialsTypeRobotLocationSecret,
			Payload: argsParsed.Secret,
		})
	}

	robot, err := client.New(
		ctx,
		argsParsed.Address,
		logger,
		client.WithDialOptions(dialOpts),
	)
	if err != nil {
		logger.Error(err)
		return
	}
	box, err := NewResetBox(ctx, robot, logger)
	if err != nil {
		logger.Error(err)
		return
	}
	defer box.Close()

	// logger.Warn("About to Home")
	// err = box.home(ctx)
	// if err != nil {
	// 	logger.Error(err)
	// }
	// logger.Warn("Homing Complete")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		switch key := scanner.Text(); key {
		case "s":
			box.Stop(ctx)
		case "h":
			go func() {
				err := box.home(ctx)
				if err != nil {
					logger.Error(err)
				}
			}()
		case "r":
			go func() {
				logger.Info("Running reset cycle")
				err := box.runReset(ctx)
				if err != nil {
					logger.Error(err)
				}
			}()
		case "q":
			return
		default:
			logger.Infof("Key press: %s", key)
		}

		if err = scanner.Err(); err != nil {
			logger.Error(err)
		}
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
		errPath <- b.gate.Home(ctx)
		_, err := b.hammer.Do(ctx, map[string]interface{}{tmcstepper.Command: tmcstepper.Home})
		errPath <- err
	}()
	go func() {
		errPath <- b.squeeze.Home(ctx)
	}()
	go func() {
		errPath <- b.elevator.Home(ctx)
	}()

	errs := multierr.Combine(
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
	)

	if errs != nil {
		return errs
	}

	// Go to starting positions
	go func() {
		errPath <- b.gate.GoTo(ctx, gateSpeed, gateClosed)
	}()
	go func() {
		errPath <- b.setSqueeze(ctx, squeezeClosed)
	}()
	go func() {
		errPath <- b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	}()
	go func() {
		errPath <- b.hammer.GoTo(ctx, hammerSpeed, hammerOffset, nil)
		errPath <- b.hammer.ResetZeroPosition(ctx, 0, nil)
	}()

	errs = multierr.Combine(
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
		<-errPath,
	)

	if errs != nil {
		return errs
	}
	b.haveHomed = true
	return nil
}

func (b *ResetBox) vibrate(ctx context.Context, level float64) {
	if level < 0.2 {
		b.vibrator.Stop(ctx, nil)
		b.vibeState = false
	} else {
		b.vibrator.SetPower(ctx, level, nil)
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

func (b *ResetBox) tipTableUp(ctx context.Context) error {
	if b.tableUp {
		return nil
	}
	if err := b.armHome(ctx); err != nil {
		return err
	}

	// Go mostly up
	b.tipper.SetPower(ctx, 1.0, nil)
	if !utils.SelectContextOrWait(ctx, 11*time.Second) {
		b.tipper.Stop(ctx, nil)
		return ctx.Err()
	}

	// All off
	b.tipper.Stop(ctx, nil)

	b.tableUp = true
	return nil
}

func (b *ResetBox) tipTableDown(ctx context.Context) error {
	if err := b.tipper.SetPower(ctx, -1.0, nil); err != nil {
		return err
	}
	if !utils.SelectContextOrWait(ctx, 10*time.Second) {
		return ctx.Err()
	}

	// Trigger this when we SHOULD be down.
	b.tableUp = false

	// Extra time for safety (actuator automatically stops on retract)
	if !utils.SelectContextOrWait(ctx, 4*time.Second) {
		b.tipper.Stop(ctx, nil)
		return ctx.Err()
	}
	// All Off
	return b.tipper.Stop(ctx, nil)
}

func (b *ResetBox) hammerTime(ctx context.Context, count int) error {
	if !b.haveHomed {
		return errors.New("must successfully home first")
	}

	for i := 0.0; i < float64(count); i++ {
		err := b.hammer.GoTo(ctx, hammerSpeed, i+0.2, nil)
		if err != nil {
			return err
		}
		if !utils.SelectContextOrWait(ctx, moment) {
			return ctx.Err()
		}
	}

	// Raise Hammer
	err := b.hammer.GoTo(ctx, hammerSpeed, float64(count), nil)
	if err != nil {
		return err
	}

	// As we go in one direction indefinitely, this is an easy fix for register overflow
	err = b.hammer.ResetZeroPosition(ctx, 0, nil)
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
	errGate := make(chan error)
	errSqueeze := make(chan error)
	errElevator := make(chan error)
	errTable := make(chan error)

	go func() {
		errSqueeze <- b.setSqueeze(ctx, squeezeClosed)
	}()
	go func() {
		errGate <- b.gate.GoTo(ctx, gateSpeed, gateCubePass)
	}()
	go func() {
		errElevator <- b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	}()
	go func() {
		errTable <- b.tipTableUp(ctx)
	}()

	errs := multierr.Combine(
		<-errSqueeze,
		<-errGate,
		<-errElevator,
		<-errTable,
	)
	if errs != nil {
		return errs
	}

	go func() {
		errTable <- b.tipTableDown(ctx)
	}()

	go func() {
		errArm <- multierr.Combine(
			b.gripper.Open(ctx),
			<-errTable,
			b.arm.MoveToJointPositions(ctx, cubeReadyPos, nil),
		)
	}()

	// Three whacks for cubes-behinds-ducks
	go func() {
		errHammer <- b.hammerTime(ctx, cubeWhacks)
		errSqueeze <- b.setSqueeze(ctx, squeezeCubePass)
	}()

	errs = multierr.Combine(
		<-errHammer,
	)
	if errs != nil {
		return errs
	}

	// DuckWhack
	go func() {
		errHammer <- b.hammerTime(ctx, duckWhacks)
	}()

	b.vibrate(ctx, vibeLevel)
	errs = <-errSqueeze
	if errs != nil {
		return errs
	}
	if !utils.SelectContextOrWait(ctx, time.Second) {
		return ctx.Err()
	}
	b.vibrate(ctx, 0)

	// Cubes in, going up
	go func() {
		errElevator <- b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop)
	}()

	errs = multierr.Combine(
		<-errArm,
		<-errElevator,
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
	errs = b.waitForGripperRecovery(ctx)
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
		errArm <- b.arm.MoveToJointPositions(ctx, duckReadyPos, nil)
	}()

	go func() {
		errElevator <- b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom)
	}()

	// Back down for duck
	errs = multierr.Combine(
		<-errElevator,
		<-errHammer,
	)
	if errs != nil {
		return errs
	}

	errs = b.gate.GoTo(ctx, gateSpeed, gateOpen)
	if errs != nil {
		return errs
	}

	if !utils.SelectContextOrWait(ctx, time.Second) {
		return ctx.Err()
	}
	// Open to load duck
	b.vibrate(ctx, vibeLevel)
	errs = multierr.Combine(
		b.setSqueeze(ctx, squeezeOpen),
		<-errArm,
	)
	if !utils.SelectContextOrWait(ctx, time.Second) {
		return ctx.Err()
	}
	// Duck in, silence and up
	b.vibrate(ctx, 0)
	if errs != nil {
		return errs
	}
	errs = multierr.Combine(
		b.setSqueeze(ctx, squeezeClosed),
		b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
	)
	if errs != nil {
		return errs
	}

	errs = b.placeDuck(ctx)
	// Try again if the duck gets stuck in the squeezer
	if errs != nil && errs.Error() == "missed the duck twice" {
		go func() {
			errArm <- b.gripper.Open(ctx)
			errArm <- b.arm.MoveToJointPositions(ctx, duckReadyPos, nil)
		}()
		errs = multierr.Combine(
			// Squish to reorient if possible
			b.setSqueeze(ctx, duckSquish),
			// Back down for duck
			b.elevator.GoTo(ctx, elevatorSpeed, elevatorBottom),
		)
		if errs != nil {
			return errs
		}
		// Open to load duck
		b.vibrate(ctx, vibeLevel)
		errs = multierr.Combine(
			b.setSqueeze(ctx, squeezeOpen),
			<-errArm,
			<-errArm,
		)
		if errs != nil {
			return errs
		}
		if !utils.SelectContextOrWait(ctx, 3*time.Second) {
			return ctx.Err()
		}
		b.vibrate(ctx, 0)
		errs = multierr.Combine(
			b.setSqueeze(ctx, squeezeClosed),
			b.elevator.GoTo(ctx, elevatorSpeed, elevatorTop),
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
		b.arm.MoveToJointPositions(ctx, safeDumpPos, nil),
		b.gripper.Open(ctx),
	)
}

// Aged gripper's pressure sensor takes several seconds to recover after releasing an object.
// This gets run in open air, and waits for the sensor to properly report empty.
func (b *ResetBox) waitForGripperRecovery(ctx context.Context) error {
	startTime := time.Now()
	for {
		ret, err := b.gripper.Do(ctx, map[string]interface{}{vgripper.Command: vgripper.GetPressure})
		if err != nil {
			return err
		}
		pressure, ok := ret[vgripper.ReturnHasPressure].(bool)
		if ok && !pressure {
			return nil
		}
		if !utils.SelectContextOrWait(ctx, moment) {
			return ctx.Err()
		}
		if time.Since(startTime) >= 30*time.Second {
			return errors.New("timed out waiting for gripper's pressure sensor to recover")
		}
	}
}

func (b *ResetBox) pickCube1(ctx context.Context) error {
	// Grab cube 1 and reset it on the field
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, cubeReadyPos, nil),
		b.arm.MoveToJointPositions(ctx, cube1grab, nil),
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
	} else if !utils.SelectContextOrWait(ctx, moment) {
		return ctx.Err()
	}

	return b.arm.MoveToJointPositions(ctx, cubeReadyPos, nil)
}

func (b *ResetBox) placeCube1(ctx context.Context) error {
	return multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cube1place, nil),
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, safeDumpPos, nil),
	)
}

func (b *ResetBox) pickCube2(ctx context.Context) error {
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, cubeReadyPos, nil),
		b.arm.MoveToJointPositions(ctx, cube2grab, nil),
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
	} else if !utils.SelectContextOrWait(ctx, moment) {
		return ctx.Err()
	}
	return b.arm.MoveToJointPositions(ctx, cubeReadyPos, nil)
}

func (b *ResetBox) placeCube2(ctx context.Context) error {
	return multierr.Combine(
		b.arm.MoveToJointPositions(ctx, cube2place, nil),
		b.gripper.Open(ctx),
	)
}

func (b *ResetBox) placeDuck(ctx context.Context) error {
	errs := multierr.Combine(
		b.gripper.Open(ctx),
		b.arm.MoveToJointPositions(ctx, duckReadyPos, nil),
		b.arm.MoveToJointPositions(ctx, duckgrabFW, nil),
	)
	if errs != nil {
		return errs
	}

	// Try to grab- this should succeed if the duck is facing forwards, and fail if facing backwards
	grabbed, errs := b.gripper.Grab(ctx)
	if grabbed {
		if !utils.SelectContextOrWait(ctx, moment) {
			return ctx.Err()
		}
		multierr.Combine(
			errs,
			b.arm.MoveToJointPositions(ctx, duckReadyPos, nil),
			b.arm.MoveToJointPositions(ctx, duckplaceFW, nil),
			b.gripper.Open(ctx),
		)
		if errs != nil {
			return errs
		}
	} else {
		// Duck was facing backwards. Grab where the backwards-facing head should be
		multierr.Combine(
			b.arm.MoveToJointPositions(ctx, duckReadyPos, nil),
			b.gripper.Open(ctx),
			b.arm.MoveToJointPositions(ctx, duckgrabREV, nil),
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
		} else if !utils.SelectContextOrWait(ctx, moment) {
			return ctx.Err()
		}
		multierr.Combine(
			b.arm.MoveToJointPositions(ctx, duckReadyPos, nil),
			b.arm.MoveToJointPositions(ctx, duckplaceREV, nil),
			b.gripper.Open(ctx),
		)
	}
	return errs
}
