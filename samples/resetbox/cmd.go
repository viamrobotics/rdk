// Package main is a reset box for a robot play area.
package main

import (
	//"bufio"
	//"bytes"
	"context"
	//"encoding/json"
	"flag"
	//"log"
	//"math"
	//"net/http"
	"sync"
	//"time"
	// "fmt"
	// "reflect"

	"github.com/go-errors/errors"

	"go.viam.com/utils"
	//"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/action"
	"go.viam.com/core/board"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"

	//"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"

	//"go.viam.com/core/sensor"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/board/detector"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

var logger = golog.NewDevelopmentLogger("resetbox")

type LinearAxis struct {
	m       []board.Motor
	mmPerRev float64
}

// GoTo moves to a position specified in mm and at a speed in mm/s
func (a *LinearAxis) GoTo(ctx context.Context, speed float64, position float64) error {
	var errors error
	for _, m := range a.m {
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		multierr.AppendInto(&errors, raw.GoTo(ctx, speed * 60 / a.mmPerRev, position / a.mmPerRev))
	}
	return errors
}

// GoFor moves for the distance specified in mm and at a speed in mm/s
func (a *LinearAxis) GoFor(ctx context.Context, d pb.DirectionRelative, speed float64, position float64) error {
	var errors error
	for _, m := range a.m {
		if m == nil {
			continue
		}
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
		multierr.AppendInto(&errors, raw.GoFor(ctx, d, speed * 60 / a.mmPerRev, position / a.mmPerRev))
	}
	return errors
}


// Home simultaneously homes all motors on an axis
func (a *LinearAxis) Home(ctx context.Context, d pb.DirectionRelative, speed float64) error {
	var homeWorkers sync.WaitGroup
	var errors error
	for _, m := range a.m {
		raw := m.GetRaw(ctx).(*board.TMCStepperMotor)
	 	homeWorkers.Add(1)
	 	utils.ManagedGo(func() {
	 		multierr.AppendInto(&errors, raw.Home(ctx, d, speed * 60 / a.mmPerRev))
	 	}, homeWorkers.Done)
	}
	homeWorkers.Wait()
	return errors
}

func (a *LinearAxis) Off(ctx context.Context) error {
	var errors error
	for _, m := range a.m {
		multierr.AppendInto(&errors, m.Off(ctx))
	}
	return errors
}


type ResetBox struct {
	logger        golog.Logger
	board      board.Board
	gate, squeeze LinearAxis
	elevator      LinearAxis
	hammer        board.Motor

	activeBackgroundWorkers *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context
}

func NewResetBox(r robot.Robot, logger golog.Logger) (*ResetBox, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	b := &ResetBox{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel, logger: logger}
	b.board = r.BoardByName("resetboard")
	if b.board == nil {
		return nil, errors.New("Cannot find board: resetboard")
	}

	b.gate.m = append(b.gate.m, b.board.Motor("gateL"))
	b.gate.m = append(b.gate.m, b.board.Motor("gateR"))
	b.gate.mmPerRev = 8.0

	b.squeeze.m = append(b.squeeze.m, b.board.Motor("squeezeL"))
	b.squeeze.m = append(b.squeeze.m, b.board.Motor("squeezeR"))
	b.squeeze.mmPerRev = 8.0

	b.elevator.m = append(b.elevator.m, b.board.Motor("elevator"))
	b.elevator.mmPerRev = 8.0

	b.hammer = b.board.Motor("hammer")

	// TODO Check for missing motors



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

	box, err := NewResetBox(myRobot, logger)
	if err != nil {
		return err
	}
	defer box.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	action.RegisterAction("Home", box.home)
	action.RegisterAction("Go Fast", box.goFast)

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
	errors := multierr.Combine(
		b.gate.Home(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 10),
		//b.squeeze.Home(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 10),
		//b.elevator.Home(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 10),
		//b.hammer.Home(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 10),
	)

	if errors != nil {
		b.logger.Error(errors)
	}
}

func (b *ResetBox) goFast(ctx context.Context, r robot.Robot) {
	b.gate.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 500, 1000)
}


