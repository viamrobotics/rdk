// Package register registers all relevant world object store models and also API specific functions
package register

import (
	// for world state store models.
	_ "go.viam.com/rdk/services/worldstatestore/fake"
	_ "go.viam.com/rdk/services/worldstatestore/fakePointCloud"
)
