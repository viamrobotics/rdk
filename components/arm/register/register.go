// Package register registers all relevant arms
package register

import (
	// register arms.
	_ "github.com/viam-modules/viam-ufactory-xarm/arm"

	_ "go.viam.com/rdk/components/arm/eva"
	_ "go.viam.com/rdk/components/arm/fake"
	_ "go.viam.com/rdk/components/arm/universalrobots"
	_ "go.viam.com/rdk/components/arm/wrapper"
)
