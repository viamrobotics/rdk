// Package register registers all relevant Boards and also subtype specific functions
package register

import (

	// for boards.
	_ "go.viam.com/rdk/component/board/arduino"
	_ "go.viam.com/rdk/component/board/fake"
	_ "go.viam.com/rdk/component/board/jetson"
	_ "go.viam.com/rdk/component/board/numato"

	// detect pi.
	_ "go.viam.com/rdk/platformdetector/pi"
)
