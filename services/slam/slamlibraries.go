// Package slam implements simultaneous localization and mapping
package slam

import (
	"bufio"
	"context"
	"image/jpeg"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

var slamLibraries = map[string]SLAM{
	"cartographer": DenseSlamAlgo{Metadata: cartographerMetadata},
	"orbslamv3":    SparseSlamAlgo{Metadata: orbslamv3Metadata},
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
var cartographerMetadata = Metadata{
	AlgoName:       "cartographer",
	SlamType:       denseSlam,
	SlamMode:       map[string]bool{"2d": true, "3d": false},
	BinaryLocation: "",
}

var orbslamv3Metadata = Metadata{
	AlgoName:       "orbslamv3",
	SlamType:       sparseSlam,
	SlamMode:       map[string]bool{"mono": true, "rgbd": true},
	BinaryLocation: "",
}

// Metadata contains all pertinant information for defining a SLAM library/algorithm including the sparse/dense definition.
type Metadata struct {
	AlgoName       string
	SlamType       slamType
	SlamMode       map[string]bool
	BinaryLocation string
}

// SparseSlamAlgo is a data structure for all sparse slam libraries/algorithms.
type SparseSlamAlgo struct {
	Metadata
	SlamType slamType
}

// DenseSlamAlgo is a data structure for all dense slam libraries/algorithms.
type DenseSlamAlgo struct {
	Metadata
	SlamType slamType
}

// SLAM interface includes definitions for SLAM functions that may vary based on library.
type SLAM interface {
	GetAndSaveData(ctx context.Context, cam camera.Camera, mode string, dataDirectory string, logger golog.Logger) error
	GetMetadata() Metadata
}

// ------------------------------------------------------------------------------------

// GetMetadata for the implemented SLAM library/algorithm.
func (s Metadata) GetMetadata() Metadata {
	return s
}

// GetAndSaveData implements the data extraction for dense algos and saving to the specified directory.
func (algo DenseSlamAlgo) GetAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, logger golog.Logger) error {
	// Get NextPointCloud
	pointcloud, err := cam.NextPointCloud(ctx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			logger.Warnf("Skipping this scan due to error: %v", err)
			return nil
		}
		return err
	}

	// Get timestamp for file name
	timeStamp := time.Now()

	// Create file
	fileMode := algo.Metadata.SlamType.ModeFileType[mode]
	f, err := os.Create(dd + "/data/data_" + timeStamp.UTC().Format("2006-01-02T15_04_05.0000") + fileMode)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)

	// Write PCD file based on mode
	if err = pc.ToPCD(pointcloud, w, 1); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// GetAndSaveData implements the data extraction for sparse algos and saving to the specified directory.
func (algo SparseSlamAlgo) GetAndSaveData(ctx context.Context, cam camera.Camera, mode string, dd string, logger golog.Logger) error {
	// Get Image
	img, _, err := cam.Next(ctx)
	if err != nil {
		if err.Error() == "bad scan: OpTimeout" {
			logger.Warnf("Skipping this scan due to error: %v", err)
			return nil
		}
		return err
	}

	// Get timestamp for file name
	timeStamp := time.Now()

	// Create file
	fileMode := algo.Metadata.SlamType.ModeFileType[mode]
	f, err := os.Create(dd + "/data/data_" + timeStamp.UTC().Format("2006-01-02T15_04_05.0000") + fileMode)
	if err != nil {
		return err
	}

	// Write iamge file based on mode
	w := bufio.NewWriter(f)

	switch mode {
	case "mono":
		if err := jpeg.Encode(w, img, nil); err != nil {
			return err
		}
	case "rgbd":
		iwd, ok := img.(*rimage.ImageWithDepth)
		if !ok {
			return errors.Errorf("want %s but don't have %T", utils.MimeTypeBoth, iwd)
		}
		if err := rimage.EncodeBoth(iwd, w); err != nil {
			return err
		}
	}
	if err = w.Flush(); err != nil {
		return err
	}
	return f.Close()
}
