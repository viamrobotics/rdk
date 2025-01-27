// Package register registers all relevant Boards and also API specific functions
package register

import (
	// for boards.
	_ "go.viam.com/rdk/components/board/esp32"
	_ "go.viam.com/rdk/components/board/fake"
)
