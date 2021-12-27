// Package register registers all relevant cameras and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"

	// for camera.
	_ "go.viam.com/rdk/component/camera/fake"

	// for camera.
	_ "go.viam.com/rdk/component/camera/gopro"

	// for camera.
	_ "go.viam.com/rdk/component/camera/imagesource"

	// for camera.
	_ "go.viam.com/rdk/component/camera/velodyne"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(camera.Subtype, registry.ResourceSubtype{
		Reconfigurable: camera.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.CameraService_ServiceDesc,
				camera.NewServer(subtypeSvc),
				pb.RegisterCameraServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return camera.NewClientFromConn(conn, name, logger)
		},
	})
}
