//go:build arm

package javascript

import (
	"github.com/pkg/errors"

	functionvm "go.viam.com/rdk/function/vm"
)

// init registers a failing function engine since wasmer is not supported on 32-bit systems
func init() {
	functionvm.RegisterEngine(functionvm.EngineNameJavaScript, func() (functionvm.Engine, error) {
		return nil, errors.New("no wasmer support on 32-bit")
	})
}
