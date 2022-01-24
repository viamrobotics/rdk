// Package register registers all relevant gantries
package register

import (

	// for gantries.
	_ "go.viam.com/rdk/component/gantry/fake"
<<<<<<< HEAD
	_ "go.viam.com/rdk/component/gantry/simple"
=======
	_ "go.viam.com/rdk/component/gantry/multiAxis"
	_ "go.viam.com/rdk/component/gantry/oneAxis"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
>>>>>>> de97de24 (moved multiAxis tests to oneAxis tests)
)
