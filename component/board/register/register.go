// Package register registers all relevant Boards and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/board"

	// for boards.
	_ "go.viam.com/rdk/component/board/arduino"
	_ "go.viam.com/rdk/component/board/fake"
	_ "go.viam.com/rdk/component/board/jetson"
	_ "go.viam.com/rdk/component/board/numato"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(board.Subtype, registry.ResourceSubtype{
		Reconfigurable: board.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.BoardService_ServiceDesc,
				board.NewServer(subtypeSvc),
				pb.RegisterBoardServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return board.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
