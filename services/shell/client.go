package shell

import (
	"context"
	"errors"
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
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewShellServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) Shell(
	ctx context.Context,
	extra map[string]interface{},
) (chan<- string, chan<- map[string]interface{}, <-chan Output, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, nil, nil, err
	}
	client, err := c.client.Shell(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	c.activeBackgroundWorkers.Add(3)
	// prime the right service
	if err := client.Send(&pb.ShellRequest{
		Name:  c.name,
		Extra: ext, // send this once; all others are OOB
	}); err != nil {
		return nil, nil, nil, err
	}

	input := make(chan string)
	oobInput := make(chan map[string]interface{})
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
						c.logger.CErrorw(ctx, "error sending data", "error", err)
						return
					}
				} else {
					if err := client.CloseSend(); err != nil {
						c.logger.CErrorw(ctx, "error closing input via CloseSend", "error", err)
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
			select {
			case oob, ok := <-oobInput:
				if ok {
					oobExt, err := protoutils.StructToStructPb(oob)
					if err != nil {
						c.logger.CErrorw(ctx, "error sending out-of-band data", "error", err)
						continue
					}

					if err := client.Send(&pb.ShellRequest{
						Name:  c.name,
						Extra: oobExt,
					}); err != nil {
						c.logger.CErrorw(ctx, "error sending out-of-band data", "error", err)
						return
					}
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

	return input, oobInput, output, nil
}

// CopyFilesToMachine is the client side RPC implementation of copying files to a machine.
// It'll send the initial metadata of the request and pass back a FileCopier that the caller
// will use to copy files over. Once the caller is done copying, it MUST close the FileCopier.
func (c *client) CopyFilesToMachine(
	ctx context.Context,
	sourceType CopyFilesSourceType,
	destination string,
	preserve bool,
	extra map[string]interface{},
) (FileCopier, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	client, err := c.client.CopyFilesToMachine(ctx)
	if err != nil {
		return nil, err
	}

	// we won't get any meaningful service level errors until the first file send
	if err := client.Send(&pb.CopyFilesToMachineRequest{
		Request: &pb.CopyFilesToMachineRequest_Metadata{
			Metadata: &pb.CopyFilesToMachineRequestMetadata{
				Name:        c.name,
				SourceType:  sourceType.ToProto(),
				Destination: destination,
				Preserve:    preserve,
				Extra:       ext,
			},
		},
	}); err != nil {
		return nil, err
	}

	// create a FileCopier that has a Copy pipeline of:
	// File->ShellRPCFileCopier->CopyFilesToMachineClient
	// ShellRPCFileCopier does the heavy lifting for us by handling fragmentation
	// and ordering.
	return newShellRPCFileCopier(shellRPCCopyWriterTo{client}, preserve), nil
}

// CopyFilesFromMachine is the client side RPC implementation of copying files from a machine.
// It'll send the initial metadata for what the machine should search for. Then, once it gets
// an initial response back from the server, it'll start copying files into a FileCopier made
// by the FileCopyFactory one-by-one until complete.
func (c *client) CopyFilesFromMachine(
	ctx context.Context,
	paths []string,
	allowRecursion bool,
	preserve bool,
	copyFactory FileCopyFactory,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	client, err := c.client.CopyFilesFromMachine(ctx)
	if err != nil {
		return err
	}

	if err := client.Send(&pb.CopyFilesFromMachineRequest{
		Request: &pb.CopyFilesFromMachineRequest_Metadata{
			Metadata: &pb.CopyFilesFromMachineRequestMetadata{
				Name:           c.name,
				Paths:          paths,
				AllowRecursion: allowRecursion,
				Preserve:       preserve,
				Extra:          ext,
			},
		},
	}); err != nil {
		return err
	}

	mdResp, err := client.Recv()
	if err != nil {
		return err
	}
	md, ok := mdResp.Response.(*pb.CopyFilesFromMachineResponse_Metadata)
	if !ok {
		return errors.New("expected copy response metadata")
	}

	copier, err := copyFactory.MakeFileCopier(
		ctx,
		CopyFilesSourceTypeFromProto(md.Metadata.SourceType),
	)
	if err != nil {
		return err
	}

	// create a FileCopyReader that has a Read/Copy pipeline of:
	// CopyFilesFromMachineClient->ShellRPCFileReadCopier->copier
	// ShellRPCFileReadCopier does the heavy lifting for us by handling fragmentation
	// and ordering of files coming in.
	reader := newShellRPCFileReadCopier(shellRPCCopyReaderFrom{client}, copier)
	defer func() {
		utils.UncheckedError(reader.Close(ctx))
	}()
	return reader.ReadAll(ctx)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
