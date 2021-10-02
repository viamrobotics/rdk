package main

import (
	"context"
	"fmt"
	"time"
	"go.viam.com/core/input"
	"go.viam.com/core/input/gamepad"
	"go.viam.com/core/config"


	"github.com/edaniels/golog"

)



// "8BitDo Pro 2" wireless
// "Microsoft X-Box 360 pad" wired
// "Xbox Wireless Controller" wireless
// "Microsoft Xbox One X pad"  wired


func main() {

	var logger = golog.NewDevelopmentLogger("gamepadtest")
	ctx := context.Background()

	g, err := gamepad.NewGamepad(ctx, nil, config.Component{}, logger)

	if err != nil {
		fmt.Println(err)
		return
	}

	inputs, err := g.Inputs(ctx)
	if err != nil {
		return
	}


	X, ok := inputs[input.AbsoluteX]
	if !ok {
		fmt.Println("Can't find X axis")
	} 

	fmt.Println(X.Name(ctx))

	repFunc := func(ctx context.Context, input input.Input, event input.Event) error {
		fmt.Println(event.Value)
		return nil
	}

	err = X.RegisterControl(ctx, repFunc, input.PositionChangeAbs)
	if err != nil {
		return
	}
	
	g.EventDispatcher(ctx)


	logger.Debug("SMURF99")

	for {
		time.Sleep(100 * time.Millisecond)
	}

	// fmt.Println("Found gamepad: ", device.Name())

	// fmt.Println(device.EventTypes())
	// fmt.Println(device.KeyTypes())
	// fmt.Println(device.AbsoluteTypes())







}
