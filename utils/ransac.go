package utils

import (
	"fmt"
	"math/rand"
	"time"
)

// SelectNIndicesWithoutReplacement select N random indices from [0,nMax] without replacement (no duplicate indices).
func SelectNIndicesWithoutReplacement(nSamples, nMax int) ([]int, error) {
	if nSamples > nMax {
		return nil, fmt.Errorf("number of elements to be sampled greater than total number of elements: %v > %v", nSamples, nMax)
	}
	a := make([]int, nMax)
	for i := range a {
		a[i] = i
	}

	//nolint:gosec
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	return a[:nSamples], nil
}
