// Package register registers all relevant Boards and also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/board"

	// for boards.
	_ "go.viam.com/rdk/component/board/arduino"
	_ "go.viam.com/rdk/component/board/fake"
	_ "go.viam.com/rdk/component/board/jetson"
	_ "go.viam.com/rdk/component/board/numato"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(board.Subtype, registry.ResourceSubtype{
		Reconfigurable: board.WrapWithReconfigurable,
	})
}
