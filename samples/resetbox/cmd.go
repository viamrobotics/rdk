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

	"github.com/go-errors/errors"

	"go.viam.com/utils/rpc/dialer"
	"go.viam.com/utils"

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


type MotorPair struct {
	a,b board.Motor
}

type LinearMotor struct {
	m board.Motor
	mmPerRev float64
}

type ResetBox struct {
	theBoard        board.Board
	gate, squeeze   MotorPair
	elevator        LinearMotor
	//hammer          board.Motor

	activeBackgroundWorkers            *sync.WaitGroup

	cancel    func()
	cancelCtx context.Context
}



func NewResetBox(r robot.Robot) (*ResetBox, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	b := &ResetBox{activeBackgroundWorkers: &sync.WaitGroup{}, cancelCtx: cancelCtx, cancel: cancel}
	b.theBoard = r.BoardByName("resetboard")
	if b.theBoard == nil {
		return nil, errors.New("Cannot find board: resetboard")
	}

	b.gate.a = b.theBoard.Motor("gateL")
	b.gate.b = b.theBoard.Motor("gateR")
	b.squeeze.a = b.theBoard.Motor("squeezeL")
	b.squeeze.b = b.theBoard.Motor("squeezeR")
	b.elevator.m = b.theBoard.Motor("elevator")
	//b.hammer = b.theBoard.Motor("hammer")

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
		b.elevator.m.Off(ctx), 
		b.gate.a.Off(ctx), 
		b.gate.b.Off(ctx), 
		b.squeeze.a.Off(ctx),
		b.squeeze.b.Off(ctx),
		//b.hammer.Off(ctx),
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

	box, err := NewResetBox(myRobot)
	if err != nil {
		return err
	}
	defer box.Close()
	//boat.StartRC(ctx)

	myRobot.AddProvider(box, config.Component{Name: "resetbox"})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	rpcDialer := dialer.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = dialer.ContextWithDialer(ctx, rpcDialer)


	// var activeBackgroundWorkers sync.WaitGroup
	// activeBackgroundWorkers.Add(2)
	// defer activeBackgroundWorkers.Wait()
	// utils.ManagedGo(func() {
	// 	trackGPS(ctx)
	// }, activeBackgroundWorkers.Done)
	// utils.ManagedGo(func() {
	// 	recordDepthWorker(ctx, myRobot.SensorByName("depth1"))
	// }, activeBackgroundWorkers.Done)


	action.RegisterAction("HomeGate", func(ctx context.Context, r robot.Robot) {
		err := box.home(ctx, r)
		if err != nil {
			logger.Errorf("error homing gate: %s", err)
		}
	})

	action.RegisterAction("Go300", func(ctx context.Context, r robot.Robot) {
		err := box.go300(ctx, r)
		if err != nil {
			logger.Errorf("error homing gate: %s", err)
		}
	})


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


func (b *ResetBox) home(ctx context.Context, r robot.Robot) error {
	raw := b.gate.a.GetRaw(ctx)
	return raw.(*board.TMCStepperMotor).Home(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 100)
}

func (b *ResetBox) go300(ctx context.Context, r robot.Robot) error {
	return b.gate.a.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 300, 1000)
}