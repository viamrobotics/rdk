package vision

import (
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"

	"github.com/viamrobotics/robotcore/utils"
)

func hsvfrom(point clusters.Coordinates) utils.HSV {
	return utils.HSV{point[0], point[1], point[2]}
}

type HSVObservation struct {
	hsv utils.HSV
}

func (o HSVObservation) Coordinates() clusters.Coordinates {
	return clusters.Coordinates{o.hsv.H, o.hsv.S, o.hsv.V}
}

func (o HSVObservation) Distance(point clusters.Coordinates) float64 {
	return o.hsv.Distance(hsvfrom(point))
}

func ClusterHSV(data []utils.HSV, numClusters int) ([]utils.HSV, error) {
	all := []clusters.Observation{}
	for _, c := range data {
		all = append(all, HSVObservation{c})
	}

	km := kmeans.New()

	clusters, err := km.Partition(all, numClusters)
	if err != nil {
		return nil, err
	}

	res := []utils.HSV{}
	for _, c := range clusters {
		res = append(res, hsvfrom(c.Center))
	}

	return res, nil
}
