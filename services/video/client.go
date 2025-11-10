package video

type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.VideoServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (resource.Resource, error) {
	c := pb.NewVideoServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) GetVideo(ctx context.Context, from, to, container string, w io.Writer) error {
	// Here we would have logic to call the GetVideo RPC and stream data to w.
	// For simplicity, we will just return nil.
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.DoCommand(ctx, &commonpb.DoCommandRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
