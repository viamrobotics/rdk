//go:build arm

package javascript

import (
	"github.com/pkg/errors"

	functionvm "go.viam.com/rdk/function/vm"
)

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	functionvm.RegisterEngine(functionvm.EngineNameJavaScript, func() (functionvm.Engine, error) {
		return nil, errors.New("no WASM support on 32-bit")
	})
}
