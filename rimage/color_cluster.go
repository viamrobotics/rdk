package rimage

import (
	"image"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
)

func colorFrom(point clusters.Coordinates) Color {
	return NewColorFromArray(point)
}

// HSVObservation TODO.
type HSVObservation struct {
	c Color
}

// Coordinates TODO.
func (o HSVObservation) Coordinates() clusters.Coordinates {
	return o.c.RawFloatArray()
}

// Distance TODO.
func (o HSVObservation) Distance(point clusters.Coordinates) float64 {
	return o.c.Distance(colorFrom(point))
}

// ClusterFromImage TODO.
func ClusterFromImage(img *Image, numClusters int) ([]Color, error) {
	return ClusterHSV(img.data, numClusters)
}

// ClusterHSV TODO.
func ClusterHSV(data []Color, numClusters int) ([]Color, error) {
	all := []clusters.Observation{}
	for _, c := range data {
		all = append(all, HSVObservation{c})
	}

	km := kmeans.New()

	clusters, err := km.Partition(all, numClusters)
	if err != nil {
		return nil, err
	}

	res := []Color{}
	for _, c := range clusters {
		res = append(res, colorFrom(c.Center))
	}

	return res, nil
}

// ClusterImage TODO.
func ClusterImage(clusters []Color, img *Image) *image.RGBA {
	palette := colorful.FastWarmPalette(len(clusters))

	clustered := image.NewRGBA(img.Bounds())

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			p := image.Point{x, y}
			idx, _, _ := img.Get(p).Closest(clusters)
			clustered.Set(x, y, palette[idx])
		}
	}

	return clustered
}
