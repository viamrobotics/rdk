package video

import (
	"context"
	"time"

	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/video/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

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
	stream    pb.VideoService_GetVideoServer
	requestID string
}

func (w streamWriter) Write(p []byte) (int, error) {
	if err := w.stream.Send(&pb.GetVideoResponse{VideoData: p, RequestId: w.requestID}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// GetVideo streams video data from the specified source to the provided writer.
// func (server *serviceServer) GetVideo(ctx context.Context, req *pb.GetVideoRequest, stream pb.VideoService_GetVideoServer) error {
// 	ctx, span := trace.StartSpan(ctx, "video::server::GetVideo")
// 	defer span.End()

// 	svc, err := server.coll.Resource(req.Name)
// 	if err != nil {
// 		return err
// 	}

// 	var start time.Time
// 	if req.StartTimestamp != nil {
// 		start = req.StartTimestamp.AsTime()
// 	}
// 	var end time.Time
// 	if req.EndTimestamp != nil {
// 		end = req.EndTimestamp.AsTime()
// 	}

// 	extra := map[string]interface{}{}
// 	if req.Extra != nil {
// 		for k, v := range req.Extra.GetFields() {
// 			extra[k] = v.AsInterface()
// 		}
// 	}

// 	// Stream directly via the interface to avoid buffering the entire video in memory.
// 	return svc.GetVideo(
// 		stream.Context(),
// 		start,
// 		end,
// 		req.VideoCodec,
// 		req.VideoContainer,
// 		req.RequestId,
// 		extra,
// 		streamWriter{stream: stream, requestID: req.RequestId},
// 	)
// }

func (server *serviceServer) GetVideo(req *pb.GetVideoRequest, stream pb.VideoService_GetVideoServer) error {
	// ctx, span := trace.StartSpan(stream.Context(), "video::server::GetVideo")
	// defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	var start time.Time
	if req.StartTimestamp != nil {
		start = req.StartTimestamp.AsTime()
	}
	var end time.Time
	if req.EndTimestamp != nil {
		end = req.EndTimestamp.AsTime()
	}

	extra := map[string]interface{}{}
	if req.Extra != nil {
		for k, v := range req.Extra.GetFields() {
			extra[k] = v.AsInterface()
		}
	}

	// Stream directly via the interface to avoid buffering the entire video in memory.
	return svc.GetVideo(
		stream.Context(),
		start,
		end,
		req.VideoCodec,
		req.VideoContainer,
		req.RequestId,
		extra,
		&streamWriter{stream: stream, requestID: req.RequestId},
	)
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
