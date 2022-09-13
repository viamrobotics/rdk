// Package register registers all relevant arms
package register

import (
	// register arms.
	_ "go.viam.com/rdk/components/arm/eva"
	_ "go.viam.com/rdk/components/arm/fake"
	_ "go.viam.com/rdk/components/arm/trossen"
	_ "go.viam.com/rdk/components/arm/universalrobots"
<<<<<<< HEAD
=======
	_ "go.viam.com/rdk/components/arm/varm"
>>>>>>> c59516e7b516ee489512669cb6f0564e308643c1
	_ "go.viam.com/rdk/components/arm/wrapper"
	_ "go.viam.com/rdk/components/arm/xarm"
	_ "go.viam.com/rdk/components/arm/yahboom"
)
