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
	ADDRESS    = ""
	API_KEY    = ""
	API_KEY_ID = ""

	DEFAULT_SOURCE_PORT      = 9090
	DEFAULT_DESTINATION_PORT = 3389
)

func main() {
	logger := logging.NewDebugLogger("client")
	var src int
	flag.IntVar(&src, "src", DEFAULT_SOURCE_PORT, "source address to listen on")

	var dest int
	flag.IntVar(&dest, "dest", DEFAULT_DESTINATION_PORT, "destination address to tunnel to")

	var addr string
	flag.StringVar(&addr, "addr", ADDRESS, "machine name to connect to")

	var apiKey string
	flag.StringVar(&apiKey, "api-key", apiKey, "api key to use to connect to machine")

	var apiKeyID string
	flag.StringVar(&apiKeyID, "api-key-id", apiKeyID, "api key id to use to connect to machine")

	flag.Parse()

	if addr == "" {
		logger.Error("please enter an address with flag --addr")
		return
	}

	logger.Infow("starting tunnel", "source address", src, "destination address", dest)
	ctx := context.Background()

	opts := []client.RobotClientOption{
		client.WithRefreshEvery(0),
		client.WithCheckConnectedEvery(0),
		client.WithDisableSessions(),
	}

	if apiKey != "" && apiKeyID != "" {
		opts = append(opts,
			client.WithDialOptions(rpc.WithEntityCredentials(
				apiKeyID,
				rpc.Credentials{
					Type:    rpc.CredentialsTypeAPIKey,
					Payload: apiKey,
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
	machine, err := client.New(ctx, addr, logger, opts...)
	if err != nil {
		logger.Error(err)
		return
	}

	defer machine.Close(context.Background())
	TunnelTraffic(ctx, machine, src, dest, logger)
}

func TunnelTraffic(ctx context.Context, machine *client.RobotClient, src, dest int, logger logging.Logger) {
	// create listener
	li, err := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(src)))
	if err != nil {
		logger.CErrorw(ctx, "failed to create listener", "err", err)
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
			if err := machine.Tunnel(ctx, conn, dest); err != nil {
				logger.CError(ctx, err)
			}
			conn.Close()
		}()
	}
	wg.Wait()
}
