package keypoints

import (
	"fmt"
	"go.viam.com/rdk/utils"
	"gonum.org/v1/gonum/mat"
)

func rangeInt(u, l, step int) ([]int, error) {
	n := (u - l) / step
	out := make([]int, n)
	current := u
	for i := 1; i < n; i++ {
		current += step
		out[i] = current
	}
	return out, nil
}

type MatchingConfig struct {
	DoCrossCheck bool
	MaxDist      float64
	DistRatio    float64
}

// MatchKeypoints takes 2 sets of decriptors and performs a matching
func MatchKeypoints(kps1, kps2 KeyPoints, cfg MatchingConfig) [][]KeyPoint {
	distances, err := utils.PairwiseDistance(nil, nil, utils.Hamming)
	if err != nil {
		return nil
	}
	indices1, err := rangeInt(0, len(kps1), 1)
	if err != nil {
		return nil
	}
	indices2 := utils.GetArgMinDistancesPerRow(distances)
	fmt.Println(len(indices1) == len(indices2))
	// mask for valid indices
	maskIdx := make([]int, len(kps1))
	for i := range maskIdx {
		maskIdx[i] = 1
	}
	if cfg.DoCrossCheck {
		// transpose distances
		distT := mat.NewDense(len(kps2), len(kps1), nil)
		distTM := distances.T()
		distT.Copy(distTM)
		fmt.Println(distT.Dims())
		// compute argmin cols
		matches1 := utils.GetArgMinDistancesPerRow(distT)
		fmt.Println(len(matches1) == len(kps2))
	}
	if cfg.MaxDist > 0 {

	}

	return nil
}
