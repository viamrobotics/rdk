// Package slam implements simultaneous localization and mapping
package slam

import (
	"context"
	"path/filepath"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
)

var slamLibraries = map[string]SLAM{
	"cartographer": denseAlgo{metadata: cartographerMetadata},
	"orbslamv3":    sparseAlgo{metadata: orbslamv3Metadata},
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	SlamMode:       map[string]bool{"2d": true, "3d": false},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	SlamMode:       map[string]bool{"mono": true, "rgbd": true},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	SlamMode       map[string]bool
	BinaryLocation string
}

// SparseSlamAlgo is a data structure for all sparse slam libraries/algorithms.
type sparseAlgo struct {
	metadata
}

// DenseSlamAlgo is a data structure for all dense slam libraries/algorithms.
type denseAlgo struct {
	metadata
}

// SLAM interface includes definitions for SLAM functions that may vary based on library.
type SLAM interface {
	getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dataDirectory string, logger golog.Logger) (string, error)
	getMetadata() metadata
}

// Gets metadata for the implemented SLAM library/algorithm.
func (s metadata) getMetadata() metadata {
	return s
}

// TODO 05/06/2022: Data processing loop in new PR (see slam.go startDataProcessing function)
// getAndSaveData implements the data extraction for dense algos and saving to the specified directory.
func (algo denseAlgo) getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, l golog.Logger) (string, error) {
	return filepath.Join(dd, "temp.txt"), nil
}

// TODO 05/03/2022: Data processing loop in new PR (see slam.go startDataProcessing function)
// getAndSaveData implements the data extraction for sparse algos and saving to the specified directory.
func (algo sparseAlgo) getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, l golog.Logger) (string, error) {
	return filepath.Join(dd, "temp.txt"), nil
}
