package shell

import (
	"context"
	"sync"

	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements ShellServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name                    string
	conn                    rpc.ClientConn
	client                  pb.ShellServiceClient
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.ZapCompatibleLogger,
) (Service, error) {
	grpcClient := pb.NewShellServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		conn:   conn,
		client: grpcClient,
		logger: logging.FromZapCompatible(logger),
	}
	return c, nil
}

func (c *client) Shell(ctx context.Context, extra map[string]interface{}) (chan<- string, <-chan Output, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, nil, err
	}
	client, err := c.client.Shell(ctx)
	if err != nil {
		return nil, nil, err
	}
	c.activeBackgroundWorkers.Add(2)
	// prime the right service
	if err := client.Send(&pb.ShellRequest{
		Name:  c.name,
		Extra: ext,
	}); err != nil {
		return nil, nil, err
	}

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
						Extra:  ext,
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

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
