package keypoints

import "errors"

// Descriptor is an alias for a slice of uint64.
type Descriptor = []uint64

// DescriptorsHammingDistance computes the pairwise distances between 2 descriptor arrays.
func DescriptorsHammingDistance(descs1, descs2 []Descriptor) ([][]int, error) {
	var m int
	var n int
	m = len(descs1)
	n = len(descs2)

	// Instantiate distances array.
	distances := make([][]int, m)
	for i := range distances {
		distances[i] = make([]int, n)
	}

	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			d, err := descHammingDistance(descs1[i], descs2[j])
			if err != nil {
				return nil, err
			}
			distances[i][j] = d
		}
	}
	return distances, nil
}

func descHammingDistance(desc1, desc2 Descriptor) (int, error) {
	if len(desc1) != len(desc2) {
		return 0, errors.New("descriptors must have same length")
	}
	var x uint64
	var y uint64
	var dist int
	for i := 0; i < len(desc1); i++ {
		x = desc1[i]
		y = desc2[i]
		// ^= is bitwise XOR
		x ^= y
		for x > 0 {
			dist++
			x &= x - 1
		}
	}
	return dist, nil
}
