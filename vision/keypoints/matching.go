package keypoints

import (
	"sort"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

var logger = logging.NewLogger("matching")

// rangeInt generates a sliced of integers from l to u-1, with step size step.
func rangeInt(u, l, step int) []int {
	if u < l {
		logger.Info("Upper bound u is lower than the lower bound l. Inverting u and l.")
		u, l = l, u
	}
	n := (u - l) / step
	out := make([]int, n)
	current := l
	out[0] = l
	for i := 1; i < n; i++ {
		current += step
		out[i] = current
	}
	return out
}

// MatchingConfig contains the parameters for matching descriptors.
type MatchingConfig struct {
	DoCrossCheck bool `json:"do_cross_check"`
	MaxDist      int  `json:"max_dist_bits"`
}

// DescriptorMatch contains the index of a match in the first and second set of descriptors, and their score.
type DescriptorMatch struct {
	Idx1        int
	Idx2        int
	Score       int
	Descriptor1 Descriptor
	Descriptor2 Descriptor
}

// MatchDescriptors takes 2 sets of descriptors and performs matching.
// Order orders: desc1 are being matched to desc2.
func MatchDescriptors(desc1, desc2 []Descriptor, cfg *MatchingConfig, logger logging.Logger) []DescriptorMatch {
	distances, err := DescriptorsHammingDistance(desc1, desc2)
	if err != nil {
		return nil
	}
	indices1 := rangeInt(len(desc1), 0, 1)
	matchedIn2 := utils.GetArgMinDistancesPerRowInt(distances)
	// mask for valid indices
	maskIdx := make([]int, len(indices1))
	for i := range maskIdx {
		maskIdx[i] = 1
	}
	if cfg.DoCrossCheck {
		// transpose distances
		distT := utils.Transpose(distances)
		// compute argmin per rows on transposed mat
		matchedIn1 := utils.GetArgMinDistancesPerRowInt(distT)
		// create mask for indices in cross check
		for _, idx := range indices1 {
			if indices1[idx] == matchedIn1[matchedIn2[idx]] {
				maskIdx[idx] *= 1
			} else {
				maskIdx[idx] *= 0
			}
		}
	}
	if cfg.MaxDist > 0 {
		for _, idx := range indices1 {
			if distances[indices1[idx]][matchedIn2[idx]] < cfg.MaxDist {
				maskIdx[idx] *= 1
			} else {
				maskIdx[idx] *= 0
			}
		}
	}
	// get the reduced set of matched indices, which will be less than or equal to len(desc1)
	dm := make([]DescriptorMatch, 0, len(desc1))
	for i := range desc1 {
		if maskIdx[i] == 1 {
			dm = append(dm, DescriptorMatch{
				Idx1:        indices1[i],
				Idx2:        matchedIn2[i],
				Score:       distances[indices1[i]][matchedIn2[i]],
				Descriptor1: desc1[indices1[i]],
				Descriptor2: desc2[matchedIn2[i]],
			})
		}
	}
	// sort by Score, highest to lowest
	sort.Slice(dm, func(i, j int) bool {
		return dm[j].Score < dm[i].Score
	})
	// fill matches, skip over points in 1 that have already been matched
	alreadyMatched := make([]bool, len(indices1))
	matches := make([]DescriptorMatch, 0, len(dm))
	for _, match := range dm {
		if !alreadyMatched[match.Idx1] {
			matches = append(matches, match)
			alreadyMatched[match.Idx1] = true
		}
	}
	return matches
}

// GetMatchingKeyPoints takes the matches and the keypoints and returns the corresponding keypoints that are matched.
func GetMatchingKeyPoints(matches []DescriptorMatch, kps1, kps2 KeyPoints) (KeyPoints, KeyPoints, error) {
	if len(kps1) < len(matches) {
		err := errors.New("there are more matches than keypoints in first set")
		return nil, nil, err
	}
	if len(kps2) < len(matches) {
		err := errors.New("there are more matches than keypoints in second set")
		return nil, nil, err
	}
	matchedKps1 := make(KeyPoints, len(matches))
	matchedKps2 := make(KeyPoints, len(matches))
	for i, match := range matches {
		matchedKps1[i] = kps1[match.Idx1]
		matchedKps2[i] = kps2[match.Idx2]
	}
	return matchedKps1, matchedKps2, nil
}
