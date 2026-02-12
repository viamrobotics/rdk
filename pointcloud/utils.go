package pointcloud

import (
	"image/color"

	"github.com/golang/geo/r3"
)

// VectorsToPointCloud converts a list of r3.Vectors into a pointcloud with the specified color.
func VectorsToPointCloud(vectors []r3.Vector, c color.NRGBA) (PointCloud, error) {
	// initialize empty pointcloud
	cloud := basicPointCloud{
		points: &matrixStorage{points: make([]PointAndData, 0, len(vectors)), indexMap: make(map[r3.Vector]uint, len(vectors))},
		meta:   NewMetaData(),
	}
	// TODO: the for loop below can be made concurrent
	// iterate thought the vector list and add to the pointcloud
	for _, v := range vectors {
		data := &basicData{hasColor: true, c: c}
		if err := cloud.Set(v, data); err != nil {
			return &cloud, err
		}
	}
	return &cloud, nil
}
