//go:build windows || no_cgo || !viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"errors"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

// Run is the fallback for builds without trajex (windows, no_cgo, or missing the
// viam_rdk_cgo_have_cxx20_rt opt-in): arm streaming is unavailable, so a session
// fails immediately with a clear error instead of at link time.
func Run(
	ctx context.Context,
	a arm.Arm,
	opts StreamOptions,
	jpCh <-chan JointPositionsChItem,
	seed []referenceframe.Input,
) error {
	return errors.New("arm streaming requires a cgo build with trajex support (build tag viam_rdk_cgo_have_cxx20_rt)")
}
