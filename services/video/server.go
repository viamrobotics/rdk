package video

import (
	"context"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedVideoServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a the video gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// GetVideo streams video data to the client.
func (server *serviceServer) GetVideo(req *pb.GetVideoRequest, stream pb.VideoService_GetVideoServer) error {
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
		extra = req.Extra.AsMap()
	}

	chunkChan, err := svc.GetVideo(
		stream.Context(),
		start,
		end,
		req.VideoCodec,
		req.VideoContainer,
		extra,
	)
	if err != nil {
		return err
	}

	// Stream video chunks
	for {
		select {
		case <-stream.Context().Done():
			return nil

		case chunk, ok := <-chunkChan:
			if !ok {
				return nil
			}

			resp := &pb.GetVideoResponse{
				VideoData:      chunk.Data,
				VideoContainer: chunk.Container,
				RequestId:      req.RequestId,
			}

			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

// DoCommand implements the generic command interface.
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
