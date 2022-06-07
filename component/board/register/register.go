// Package register registers all relevant Boards and also subtype specific functions
package register

import (

	// for boards.
	_ "go.viam.com/rdk/component/board/arduino"
	_ "go.viam.com/rdk/component/board/fake"
	_ "go.viam.com/rdk/component/board/hat/pca9685"
	_ "go.viam.com/rdk/component/board/jetson"
	_ "go.viam.com/rdk/component/board/numato"
	_ "go.viam.com/rdk/component/board/pi"
	_ "go.viam.com/rdk/component/board/ti"
)
