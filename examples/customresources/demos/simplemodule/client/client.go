// Package main tests out all four custom models in the complexmodule.
package main

import (
	"context"
	"math/rand"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := golog.NewDevelopmentLogger("client")

	// Connect to the default localhost port for viam-server.
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer robot.Close(context.Background())

	// Get the two counter components.
	logger.Info("---- Getting counter1 (generic api) -----")
	res, err := robot.ResourceByName(generic.Named("counter1"))
	if err != nil {
		logger.Fatal(err)
	}
	counter1 := res.(generic.Generic)

	logger.Info("---- Getting counter2 (generic api) -----")
	res, err = robot.ResourceByName(generic.Named("counter2"))
	if err != nil {
		logger.Fatal(err)
	}
	counter2 := res.(generic.Generic)

	// For each counter, we'll look at the total, then add 20 random numbers to it.
	// Only on restart of the server will they get reset.
	for name, c := range []generic.Generic{counter1, counter2} {
		// Get the starting value of the given counter.
		ret, err := counter1.DoCommand(context.Background(), map[string]interface{}{"command": "get"})
		if err != nil {
			logger.Fatal(err)
		}
		logger.Infof("---- Adding random values to counter%d -----", name+1)
		logger.Infof("start\t=%.0f", ret["total"]) // numeric values are floats by default
		for n := 0; n < 20; n++ {
			val := rand.Intn(100)
			ret, err := c.DoCommand(context.Background(), map[string]interface{}{"command": "add", "value": val})
			if err != nil {
				logger.Fatal(err)
			}
			logger.Infof("+%d\t=%.0f", val, ret["total"])
		}
	}

}
