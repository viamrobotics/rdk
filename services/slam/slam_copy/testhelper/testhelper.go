// Package testhelper provides helper functions for testing implementations of slam libraries
package testhelper

import (
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/slam_copy/config"
)

const (
	dataBufferSize = 4
)

// CreateTempFolderArchitecture creates a new random temporary
// directory with the config, data, and map subdirectories needed
// to run the SLAM libraries.
func CreateTempFolderArchitecture(logger golog.Logger) (string, error) {
	tmpDir, err := os.MkdirTemp("", "*")
	if err != nil {
		return "nil", err
	}
	if err := config.SetupDirectories(tmpDir, logger); err != nil {
		return "", err
	}
	return tmpDir, nil
}

// ResetFolder removes all content in path and creates a new directory
// in its place.
func ResetFolder(path string) error {
	dirInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !dirInfo.IsDir() {
		return errors.Errorf("the path passed ResetFolder does not point to a folder: %v", path)
	}
	if err = os.RemoveAll(path); err != nil {
		return err
	}
	return os.Mkdir(path, dirInfo.Mode())
}

// CheckDeleteProcessedData compares the number of files found in a specified data
// directory with the previous number found and uses the useLiveData and
// deleteProcessedData values to evaluate this comparison. It returns the number of files
// currently in the data directory for the specified config. Future invocations should pass in this
// value. This function should be passed 0 as a default prev argument in order to get the
// number of files currently in the directory.
func CheckDeleteProcessedData(t *testing.T, slamMode slam.Mode, dir string, prev int, deleteProcessedData, useLiveData bool) int {
	switch slamMode {
	case slam.Mono:
		numFiles, err := checkDataDirForExpectedFiles(t, dir+"/data/rgb", prev, deleteProcessedData, useLiveData)
		test.That(t, err, test.ShouldBeNil)
		return numFiles
	case slam.Rgbd:
		numFilesRGB, err := checkDataDirForExpectedFiles(t, dir+"/data/rgb", prev, deleteProcessedData, useLiveData)
		test.That(t, err, test.ShouldBeNil)

		numFilesDepth, err := checkDataDirForExpectedFiles(t, dir+"/data/depth", prev, deleteProcessedData, useLiveData)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, numFilesRGB, test.ShouldEqual, numFilesDepth)
		return numFilesRGB
	case slam.Dim2d:
		numFiles, err := checkDataDirForExpectedFiles(t, dir+"/data", prev, deleteProcessedData, useLiveData)
		test.That(t, err, test.ShouldBeNil)
		return numFiles
	case slam.Dim3d:
		// TODO: Delete this when implementing models:
		// https://viam.atlassian.net/browse/RSDK-2015
		// https://viam.atlassian.net/browse/RSDK-2014
		return 0
	default:
		return 0
	}
}

func checkDataDirForExpectedFiles(t *testing.T, dir string, prev int, deleteProcessedData, useLiveData bool) (int, error) {
	files, err := os.ReadDir(dir)
	test.That(t, err, test.ShouldBeNil)

	if prev == 0 {
		return len(files), nil
	}
	if deleteProcessedData && useLiveData {
		test.That(t, prev, test.ShouldBeLessThanOrEqualTo, dataBufferSize+1)
	}
	if !deleteProcessedData && useLiveData {
		test.That(t, prev, test.ShouldBeLessThan, len(files))
	}
	if deleteProcessedData && !useLiveData {
		return 0, errors.New("the delete_processed_data value cannot be true when running SLAM in offline mode")
	}
	if !deleteProcessedData && !useLiveData {
		test.That(t, prev, test.ShouldEqual, len(files))
	}
	return len(files), nil
}
