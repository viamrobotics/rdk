// Package register registers all relevant Boards and also subtype specific functions
package register

import (
	"go.viam.com/core/component/board"

	_ "go.viam.com/core/component/board/arduino" // for board
	_ "go.viam.com/core/component/board/fake"    // for board
	_ "go.viam.com/core/component/board/jetson"  // for board
	_ "go.viam.com/core/component/board/numato"  // for board

	// TODO(https://github.com/viamrobotics/core/issues/377): build constraints exclude
	// all go files, so this cannot simply be imported here
	// _ "go.viam.com/core/component/board/pi"      // for board

	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
)

func init() {
	registry.RegisterResourceSubtype(board.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return board.WrapWithReconfigurable(r)
		},
	})
}
