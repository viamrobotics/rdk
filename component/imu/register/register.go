// Package register registers all relevant IMUs
package register

import (
	// for IMUs.
	_ "go.viam.com/rdk/component/imu/fake"
	_ "go.viam.com/rdk/component/imu/vectornav"
)
