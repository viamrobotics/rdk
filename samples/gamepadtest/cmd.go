package main

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/input/gamepad"
	"go.viam.com/core/registry"

	"github.com/edaniels/golog"
)

func main() {

	var logger = golog.NewDevelopmentLogger("gamepadtest")
	ctx := context.Background()

	registration := registry.InputControllerLookup("gamepad")
	if registration == nil {
		fmt.Println("No gamepad component type found")
		return
	}

	g, err := registration.Constructor(ctx, nil, config.Component{Type: config.ComponentTypeInputController, Model: "gamepad", ConvertedAttributes: gamepad.Config{DevFile: ""}}, logger)

	if err != nil {
		fmt.Println(err)
		return
	}

	repFunc := func(ctx context.Context, input input.Input, event input.Event) {
		fmt.Printf("%s: %.4f\n", event.Code, event.Value)
		return
	}

	inputs, err := g.Inputs(ctx)
	if err != nil {
		return
	}

	for _, v := range inputs {
		err = v.RegisterControl(ctx, repFunc, input.AllEvents)
		if err != nil {
			return
		}
	}

	// Loop forever
	for {
		time.Sleep(1000 * time.Millisecond)
	}

}
