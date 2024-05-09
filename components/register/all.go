// Package register registers all components
package register

import (
	// register components.
	_ "go.viam.com/rdk/components/encoder/register"
	_ "go.viam.com/rdk/components/generic/register"
	_ "go.viam.com/rdk/components/input/register"

	// register APIs without implementations directly.
	_ "go.viam.com/rdk/components/powersensor/register"
)
