package keypoints

import (
	"fmt"
	"image"
	"sort"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	"github.com/gonum/floats"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

var logger = golog.NewLogger("matching")

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
	DoCrossCheck bool    `json:"do_cross_check"`
	MaxDist      float64 `json:"max_dist"`
}

// DescriptorMatch contains the index of a match in the first and second set of descriptors.
type DescriptorMatch struct {
	Idx1 int
	Idx2 int
}

// DescriptorMatches contains the descriptors and their matches.
type DescriptorMatches struct {
	Indices      []DescriptorMatch
	Descriptors1 Descriptors
	Descriptors2 Descriptors
}

func convertDescriptorsToFloats(desc Descriptors) [][]float64 {
	out := make([][]float64, len(desc))
	for i := range out {
		out[i] = []float64(desc[i])
	}
	return out
}

func indexCount(list []int) []string {
	m := map[int]int{}
	for _, n := range list {
		if _, ok := m[n]; !ok {
			m[n] = 0
		} else {
			m[n]++
		}
	}
	// then sort by value order
	type kv struct {
		Key   int
		Value int
	}

	var ss []kv
	for k, v := range m {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})
	text := make([]string, len(ss))
	for i, kv := range ss {
		text[i] = fmt.Sprintf("%d: %d", kv.Key, kv.Value)
	}
	return text
}

// MatchKeypoints takes 2 sets of descriptors and performs matching.
func MatchKeypoints(desc1, desc2 Descriptors, cfg *MatchingConfig, logger golog.Logger) *DescriptorMatches {
	d1 := convertDescriptorsToFloats(desc1)
	d2 := convertDescriptorsToFloats(desc2)
	distances, err := utils.PairwiseDistance(d1, d2, utils.Hamming)
	if err != nil {
		return nil
	}
	r, c := distances.Dims()
	logger.Debugf("size of distances (%d, %d) and matrix size (r=%d, c=%d)", len(d1), len(d2), r, c)
	logger.Debugf("descriptor 1524 of d1: %v", d1[1524])
	logger.Debugf("descriptor 2015 of d2: %v", d2[2015])
	logger.Debugf("descriptor 2025 of d2: %v", d2[2025])
	indices1 := rangeInt(len(desc1), 0, 1)
	indices2 := utils.GetArgMinDistancesPerRow(distances)
	logger.Debugf("index count for 2:\n %v", indexCount(indices2))
	// mask for valid indices
	maskIdx := make([]int, len(desc1))
	for i := range maskIdx {
		maskIdx[i] = 1
	}
	if cfg.DoCrossCheck {
		// transpose distances
		distT := mat.NewDense(len(desc2), len(desc1), nil)
		distTM := distances.T()
		distT.Copy(distTM)
		// compute argmin per rows on transposed mat
		matches1 := utils.GetArgMinDistancesPerRow(distT)
		// create mask for indices in cross check
		for i := range indices1 {
			if indices1[i] == matches1[indices2[i]] {
				maskIdx[i] *= 1
			} else {
				maskIdx[i] *= 0
			}
		}
	}
	if cfg.MaxDist > 0 {
		for i := range indices1 {
			if distances.At(indices1[i], indices2[i]) < cfg.MaxDist {
				maskIdx[i] *= 1
			} else {
				maskIdx[i] = 0
			}
		}
	}
	// masked indices
	idx1 := make([]int, 0, len(desc1))
	idx2 := make([]int, 0, len(desc1))
	for i := range desc1 {
		if maskIdx[i] == 1 {
			idx1 = append(idx1, indices1[i])
			idx2 = append(idx2, indices2[i])
		}
	}
	// get minimum distances per selected pair of descriptor
	dist := make([]float64, len(idx1))
	for i := range dist {
		dist[i] = distances.At(idx1[i], idx2[i])
	}
	// sort
	sortedIndices := make([]int, len(idx1))
	floats.Argsort(dist, sortedIndices)
	// fill matches
	matches := make([]DescriptorMatch, len(idx1))
	for i, idx := range sortedIndices {
		matches[i] = DescriptorMatch{idx1[idx], idx2[idx]}
	}

	return &DescriptorMatches{matches, desc1, desc2}
}

// GetMatchingKeyPoints takes the matches and the keypoints and returns the corresponding keypoints that are matched.
func GetMatchingKeyPoints(matches *DescriptorMatches, kps1, kps2 KeyPoints) (KeyPoints, KeyPoints, error) {
	if len(kps1) < len(matches.Indices) {
		err := errors.New("there are more matches than keypoints in first set")
		return nil, nil, err
	}
	if len(kps2) < len(matches.Indices) {
		err := errors.New("there are more matches than keypoints in second set")
		return nil, nil, err
	}
	matchedKps1 := make(KeyPoints, len(matches.Indices))
	matchedKps2 := make(KeyPoints, len(matches.Indices))
	for i, match := range matches.Indices {
		matchedKps1[i] = kps1[match.Idx1]
		matchedKps2[i] = kps1[match.Idx2]
	}
	return matchedKps1, matchedKps2, nil
}

func sortDescriptorsByPoint(desc Descriptors, kps KeyPoints, logger golog.Logger) (Descriptors, KeyPoints, error) {
	if len(desc) != len(kps) {
		return nil, nil, errors.Errorf("number of descriptors (%d) does not equal number of keypoints (%d)", len(desc), len(kps))
	}
	// sort by point order
	type ptdesc struct {
		Kp  image.Point
		Des Descriptor
	}

	sorted := make([]ptdesc, 0, len(kps))
	for i := range kps {
		sorted = append(sorted, ptdesc{kps[i], desc[i]})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Kp.X > sorted[j].Kp.X
	})
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Kp.Y > sorted[j].Kp.Y
	})
	logger.Debugf("sorted points:\n %v", sorted[:10])
	sortedDesc := make(Descriptors, 0, len(desc))
	sortedKps := make(KeyPoints, 0, len(kps))
	for i := range sorted {
		sortedDesc = append(sortedDesc, sorted[i].Des)
		sortedKps = append(sortedKps, sorted[i].Kp)
	}
	return sortedDesc, sortedKps, nil
}
