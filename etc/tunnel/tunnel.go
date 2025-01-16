// main TBD
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"strconv"
	"sync"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
)

var (
	ADDRESS    = "something-unique"
	API_KEY    = ""
	API_KEY_ID = ""

	DEFAULT_SOURCE_PORT      = 9090
	DEFAULT_DESTINATION_PORT = 3389
)

func main() {
	var src int
	flag.IntVar(&src, "src", DEFAULT_SOURCE_PORT, "source address to listen on")

	var dest int
	flag.IntVar(&dest, "dest", DEFAULT_DESTINATION_PORT, "destination address to tunnel to")

	flag.Parse()

	logger := logging.NewDebugLogger("client")
	logger.Infow("starting tunnel", "source address", src, "destination address", dest)
	ctx := context.Background()

	opts := []client.RobotClientOption{
		client.WithRefreshEvery(0),
		client.WithCheckConnectedEvery(0),
		client.WithDisableSessions(),
	}

	if API_KEY != "" && API_KEY_ID != "" {
		opts = append(opts,
			client.WithDialOptions(rpc.WithEntityCredentials(
				API_KEY_ID,
				rpc.Credentials{
					Type:    rpc.CredentialsTypeAPIKey,
					Payload: API_KEY,
				}),
			),
		)

	} else {
		opts = append(opts,
			client.WithDialOptions(
				rpc.WithInsecure(),
				rpc.WithDisableDirectGRPC(),
			),
		)
	}
	machine, err := client.New(ctx, ADDRESS, logger, opts...)
	if err != nil {
		logger.Info(err)
		return
	}

	defer machine.Close(context.Background())

	TunnelTraffic(ctx, machine, src, dest, logger)
}

func TunnelTraffic(ctx context.Context, machine *client.RobotClient, src, dest int, logger logging.Logger) {
	// create listener
	li, err := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(src)))
	if err != nil {
		logger.Errorf("failed to create listener: %v\n", err)
		return
	}
	defer li.Close()

	var wg sync.WaitGroup
	for {
		if ctx.Err() != nil {
			break
		}
		conn, err := li.Accept()
		if err != nil {
			fmt.Printf("failed to accept conn: %v\n", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// call tunnel once per connection
			machine.Tunnel(ctx, conn, dest)
			conn.Close()
		}()
	}
	wg.Wait()
}
