package video

type serviceServer struct {
	pb.UnimplementedVideoServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a the video gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service]) interface{} {
	return &serviceServer{coll: coll}
}

type streamWriter struct {
	stream    servicepb.videoService_GetVideoServer
	requestID string
}

func (w streamWriter) Write(p []byte) (int, error) {
	if err := w.stream.Send(&servicepb.GetVideoResponse{VideoData: p, RequestId: w.requestID}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// GetVideo streams video data from the specified source to the provided writer.
func (server *serviceServer) GetVideo(ctx context.Context, req *pb.GetVideoRequest) (
	*pb.GetVideoResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "video::server::GetVideo")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	// Here we would have logic to stream video data to the writer.
	// For simplicity, we will just return an empty response.
	return &pb.GetVideoResponse{}, nil
}

func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "video::server::DoCommand")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
