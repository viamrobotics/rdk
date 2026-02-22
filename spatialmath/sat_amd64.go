//go:build amd64

package spatialmath

//go:noescape
func obbSATMaxGap(input *[27]float64) float64
