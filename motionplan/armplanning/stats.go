package armplanning

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.viam.com/rdk/motionplan"
)

// JointDeltaStats holds statistics about delta values for a single joint.
type JointDeltaStats struct {
	Component string
	JointIdx  int
	Count     int
	Mean      float64
	StdDev    float64
	Outside1  int // count of values more than 1 std dev from mean
	Outside2  int // count of values more than 2 std dev from mean
}

// TrajectoryDeltaStats computes delta statistics for each joint across all steps in a trajectory.
// Returns nil if trajectory has fewer than 2 steps.
func TrajectoryDeltaStats(trajectory motionplan.Trajectory) []JointDeltaStats {
	if len(trajectory) < 2 {
		return nil
	}

	// Collect deltas for each component:joint
	// key is "component:jointIdx", value is slice of delta values
	allDeltas := map[string][]float64{}

	for idx := 1; idx < len(trajectory); idx++ {
		curr := trajectory[idx]
		prev := trajectory[idx-1]

		for component, currInputs := range curr {
			prevInputs, ok := prev[component]
			if !ok || len(prevInputs) == 0 || len(currInputs) == 0 {
				continue
			}

			for i, currVal := range currInputs {
				if i >= len(prevInputs) {
					break
				}
				key := fmt.Sprintf("%s:%d", component, i)
				delta := currVal - prevInputs[i]
				allDeltas[key] = append(allDeltas[key], delta)
			}
		}
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(allDeltas))
	for k := range allDeltas {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Compute statistics for each joint
	results := make([]JointDeltaStats, 0, len(keys))
	for _, key := range keys {
		deltas := allDeltas[key]
		n := len(deltas)
		if n == 0 {
			continue
		}

		// Parse component and joint index from key (format: "component:jointIdx")
		lastColon := strings.LastIndex(key, ":")
		component := key[:lastColon]
		jointIdx, err := strconv.Atoi(key[lastColon+1:])
		if err != nil {
			// Should not happen since we create the keys ourselves
			continue
		}

		// Calculate mean
		sum := 0.0
		for _, d := range deltas {
			sum += d
		}
		mean := sum / float64(n)

		// Calculate standard deviation
		sumSqDiff := 0.0
		for _, d := range deltas {
			diff := d - mean
			sumSqDiff += diff * diff
		}
		stdDev := math.Sqrt(sumSqDiff / float64(n))

		// Count values outside 1 and 2 standard deviations
		outside1 := 0
		outside2 := 0
		for _, d := range deltas {
			diff := math.Abs(d - mean)
			if diff > stdDev {
				outside1++
			}
			if diff > 2*stdDev {
				outside2++
			}
		}

		results = append(results, JointDeltaStats{
			Component: component,
			JointIdx:  jointIdx,
			Count:     n,
			Mean:      mean,
			StdDev:    stdDev,
			Outside1:  outside1,
			Outside2:  outside2,
		})
	}

	return results
}
