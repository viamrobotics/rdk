package vision

import (
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
)

func hsvfrom(point clusters.Coordinates) HSV {
	return HSV{point[0], point[1], point[2]}
}

type HSVObservation struct {
	hsv HSV
}

func (o HSVObservation) Coordinates() clusters.Coordinates {
	return clusters.Coordinates{o.hsv.H, o.hsv.S, o.hsv.V}
}

func (o HSVObservation) Distance(point clusters.Coordinates) float64 {
	return o.hsv.Distance(hsvfrom(point))
}

func ClusterHSV(data []HSV, numClusters int) ([]HSV, error) {
	all := []clusters.Observation{}
	for _, c := range data {
		all = append(all, HSVObservation{c})
	}

	km := kmeans.New()

	clusters, err := km.Partition(all, numClusters)
	if err != nil {
		return nil, err
	}

	res := []HSV{}
	for _, c := range clusters {
		res = append(res, hsvfrom(c.Center))
	}

	return res, nil
}
