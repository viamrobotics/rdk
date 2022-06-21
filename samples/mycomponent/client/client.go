package main

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/grpc/client"
	myc "go.viam.com/rdk/samples/mycomponent/component"
)

func main() {
	logger := golog.NewDevelopmentLogger("client")
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}

	res, err := robot.ResourceByName(myc.Named("comp1"))
	if err != nil {
		logger.Fatal(err)
	}
	comp1 := res.(myc.MyComponent)
	ret1, err := comp1.DoOne(context.Background(), "hello")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret1)

	ret2, err := comp1.DoOneClientStream(context.Background(), []string{"hello", "arg1", "foo"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret2)

	ret2, err = comp1.DoOneClientStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret2)

	ret3, err := comp1.DoOneServerStream(context.Background(), "hello")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret3, err = comp1.DoOneBiDiStream(context.Background(), []string{"hello", "arg1", "foo"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret3, err = comp1.DoOneBiDiStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)
}
