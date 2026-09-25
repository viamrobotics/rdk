package worksheet

import (
	"fmt"
	"sort"

	"github.com/golang/geo/r3"
	viz "github.com/viam-labs/motion-tools/client/client"

	"go.viam.com/rdk/spatialmath"
)

// boxDims is the asymmetric box dimensions (10x20x30) so orientation is visually obvious.
var boxDims = r3.Vector{X: 10, Y: 20, Z: 30}

// orderedColors are assigned to poses by sorted key order.
var orderedColors = []string{"blue", "green", "yellow", "purple", "cyan"}

// PoseColorByIndex returns the color for the nth pose (0-indexed).
func PoseColorByIndex(i int) string {
	if i < len(orderedColors) {
		return orderedColors[i]
	}
	return "purple"
}

// vizEnabled tracks whether visualization is available.
var vizEnabled = true

// DrawInputPoses draws the input poses as colored asymmetric boxes.
func DrawInputPoses(poses map[string]spatialmath.Pose) {
	if !vizEnabled {
		return
	}
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		//nolint:forbidigo
		fmt.Println("  (motion-tools not available, continuing text-only)")
		vizEnabled = false
		return
	}

	// Draw origin reference box in white
	originBox, err := spatialmath.NewBox(spatialmath.NewZeroPose(), boxDims, "origin")
	if err == nil {
		if err := viz.DrawGeometry(originBox, "white"); err != nil {
			vizEnabled = false
			return
		}
	}

	names := make([]string, 0, len(poses))
	for name := range poses {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		pose := poses[name]
		color := PoseColorByIndex(i)
		box, err := spatialmath.NewBox(pose, boxDims, name)
		if err != nil {
			continue
		}
		if err := viz.DrawGeometry(box, color); err != nil {
			vizEnabled = false
			return
		}
	}
}

// DrawResult draws the result pose as a red box (additive, does not clear existing objects).
func DrawResult(pose spatialmath.Pose) {
	if !vizEnabled {
		return
	}
	box, err := spatialmath.NewBox(pose, boxDims, "result")
	if err != nil {
		return
	}
	//nolint:errcheck
	viz.DrawGeometry(box, "red")
}

// ClearVisualization removes all objects from motion-tools.
func ClearVisualization() {
	if !vizEnabled {
		return
	}
	//nolint:errcheck
	viz.RemoveAllSpatialObjects()
}
