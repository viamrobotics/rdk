// Package register registers all relevant bases
package register

import (

	// register bases.
	_ "go.viam.com/rdk/component/base/agilex"
	_ "go.viam.com/rdk/component/base/boat"
	_ "go.viam.com/rdk/component/base/fake"
	_ "go.viam.com/rdk/component/base/wheeled"
)
