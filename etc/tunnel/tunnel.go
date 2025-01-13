// main TBD
package main

import (
	"context"
	"fmt"
	"net"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := logging.NewDebugLogger("client")
	machine, err := client.New(
		context.Background(),
		"something-unique",
		logger,
		client.WithDialOptions(rpc.WithInsecure(), rpc.WithDisableDirectGRPC()),
		client.WithRefreshEvery(0),
		client.WithCheckConnectedEvery(0),
		client.WithDisableSessions(),
	)
	if err != nil {
		logger.Info(err)
		return
	}

	defer machine.Close(context.Background())

	// create listener
	li, err := net.Listen("tcp", net.JoinHostPort("localhost", "9090"))
	if err != nil {
		fmt.Printf("failed to make listener: %v\n", err)
	}
	defer li.Close()
	// call traffic once per connection
	// in a true tunnelling scenario
	// just keep calling traffic
	for {
		c1, err := li.Accept()
		if err != nil {
			fmt.Printf("failed to accept conn: %v\n", err)
		}
		go func() {
			machine.Traffic(context.Background(), c1)
			c1.Close()
		}()

	}

}
