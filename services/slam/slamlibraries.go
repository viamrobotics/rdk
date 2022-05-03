// Package slam implements simultaneous localization and mapping
package slam

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
)

var slamLibraries = map[string]SLAM{
	"cartographer": denseSlamAlgo{metadata: cartographerMetadata},
	"orbslamv3":    sparseSlamAlgo{metadata: orbslamv3Metadata},
}

// Define general slam types. Sparse slam for RGB pixel images which need to go through
// feature extraction and dense slam that operates on pointclouds commonly produced by lidars.
var denseSlam = slamType{
	SupportedCameras: map[string][]string{"rplidar": {"2d"}, "velodyne": {"3d, 2d"}},
	ModeFileType:     map[string]string{"2d": ".pcd", "3d": ".pcd"},
}

var sparseSlam = slamType{
	SupportedCameras: map[string][]string{"intelrealsense": {"rgbd, mono"}},
	ModeFileType:     map[string]string{"mono": ".jpeg", "rgbd": ".both"},
}

type slamType struct {
	SupportedCameras map[string][]string
	ModeFileType     map[string]string
}

// Define currently implemented slam libraries.
var cartographerMetadata = metadata{
	AlgoName:       "cartographer",
	SlamType:       denseSlam,
	SlamMode:       map[string]bool{"2d": true, "3d": false},
	BinaryLocation: "",
}

var orbslamv3Metadata = metadata{
	AlgoName:       "orbslamv3",
	SlamType:       sparseSlam,
	SlamMode:       map[string]bool{"mono": true, "rgbd": true},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type metadata struct {
	AlgoName       string
	SlamType       slamType
	SlamMode       map[string]bool
	BinaryLocation string
}

// SparseSlamAlgo is a data structure for all sparse slam libraries/algorithms.
type sparseSlamAlgo struct {
	metadata
	SlamType slamType
}

// DenseSlamAlgo is a data structure for all dense slam libraries/algorithms.
type denseSlamAlgo struct {
	metadata
	SlamType slamType
}

// SLAM interface includes definitions for SLAM functions that may vary based on library.
type SLAM interface {
	getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dataDirectory string, logger golog.Logger) error
	getMetadata() metadata
}

// GetMetadata for the implemented SLAM library/algorithm.
func (s metadata) getMetadata() metadata {
	return s
}

// getAndSaveData implements the data extraction for dense algos and saving to the specified directory.
func (algo denseSlamAlgo) getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, logger golog.Logger) error {
	return nil
}

// TBD 05/03/2022: Data processing loop in new PR (see slam.go startDataProcessing function)
// getAndSaveData implements the data extraction for sparse algos and saving to the specified directory.
func (algo sparseSlamAlgo) getAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, logger golog.Logger) error {
	return nil
}
