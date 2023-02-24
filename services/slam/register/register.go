// Package register registers all relevant slam models and also subtype specific functions
package register

import (
	// for slam models.
	_ "go.viam.com/rdk/services/slam/builtin"
	_ "go.viam.com/rdk/services/slam/fake"
)
