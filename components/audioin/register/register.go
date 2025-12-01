// Package register registers all relevant audio in components
package register

import (
	// for audio in components and collectors.
	_ "go.viam.com/rdk/components/audioin"
	_ "go.viam.com/rdk/components/audioin/fake"
)
