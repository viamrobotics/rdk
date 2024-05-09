// Package register registers all components
package register

import (
	// register components.
	_ "go.viam.com/rdk/components/generic/register"
	// register APIs without implementations directly.
	_ "go.viam.com/rdk/components/powersensor/register"
)
