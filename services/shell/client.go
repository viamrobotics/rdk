package shell

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

// client implements ShellServiceClient.
type client struct {
	name                    string
	conn                    rpc.ClientConn
	client                  pb.ShellServiceClient
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	grpcClient := pb.NewShellServiceClient(conn)
	c := &client{
		name:   name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c
}

func (c *client) Shell(ctx context.Context) (chan<- string, <-chan Output, error) {
	client, err := c.client.Shell(ctx)
	if err != nil {
		return nil, nil, err
	}
	c.activeBackgroundWorkers.Add(2)

	input := make(chan string)
	output := make(chan Output)

	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundWorkers.Done()

		for {
			select {
			case dataIn, ok := <-input:
				if ok {
					if err := client.Send(&pb.ShellRequest{
						Name:   c.name,
						DataIn: dataIn,
					}); err != nil {
						c.logger.Errorw("error sending data", "error", err)
						return
					}
				} else {
					if err := client.CloseSend(); err != nil {
						c.logger.Errorw("error closing input via CloseSend", "error", err)
						return
					}
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})

	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundWorkers.Done()

		for {
			resp, err := client.Recv()
			if err != nil {
				select {
				case output <- Output{
					EOF: true,
				}:
				case <-ctx.Done():
				}
				close(output)
				return
			}

			select {
			case output <- Output{
				Output: resp.DataOut,
				Error:  resp.DataErr,
				EOF:    resp.Eof,
			}:
			case <-ctx.Done():
				close(output)
				return
			}
		}
	})

	return input, output, nil
}
