// Package register registers all relevant Boards and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/board"
	"go.viam.com/utils/rpc"

	// for board.
	_ "go.viam.com/rdk/component/board/arduino"

	// for board.
	_ "go.viam.com/rdk/component/board/fake"

	// for board.
	_ "go.viam.com/rdk/component/board/jetson"

	// for board.
	_ "go.viam.com/rdk/component/board/numato"

	// _ "go.viam.com/rdk/component/board/pi"      // for board.

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
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return board.NewClientFromConn(conn, name, logger)
		},
	})
}
