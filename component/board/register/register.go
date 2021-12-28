// Package register registers all relevant Boards and also subtype specific functions
package register

import (
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
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(board.Subtype, registry.ResourceSubtype{
		Reconfigurable: board.WrapWithReconfigurable,
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return board.NewClientFromConn(conn, name, logger)
		},
	})
}
