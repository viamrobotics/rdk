// Package register registers all relevant cameras and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/core/component/camera"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	_ "go.viam.com/core/component/camera/fake"        // for camera
	_ "go.viam.com/core/component/camera/gopro"       // for camera
	_ "go.viam.com/core/component/camera/imagesource" // for camera
	_ "go.viam.com/core/component/camera/velodyne"    // for camera
)

func init() {
	registry.RegisterResourceSubtype(camera.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return camera.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.CameraService_ServiceDesc,
				camera.NewServer(subtypeSvc),
				pb.RegisterCameraServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
			return camera.NewClientFromConn(conn, name, logger)
		},
	})
}
