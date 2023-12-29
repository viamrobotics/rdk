package fake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

func TestFakeSLAMPosition(t *testing.T) {
	expectedComponentReference := ""
	slamSvc := NewSLAM(slam.Named("test"), logging.NewTestLogger(t))

	p, componentReference, err := slamSvc.Position(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, componentReference, test.ShouldEqual, expectedComponentReference)

	// spatialmath.PoseAlmostEqual is used here as tiny differences were observed
	// in floating point values between M1 mac & arm64 linux which
	// were causing tests to pass on M1 mac but fail on ci.
	expectedPose := spatialmath.NewPose(
		r3.Vector{X: 5.921536787524187, Y: 13.296696037491639, Z: 0.0000000000000},
		&spatialmath.Quaternion{Real: 0.9999997195238413, Imag: 0, Jmag: 0, Kmag: 0.0007489674483818071})
	test.That(t, spatialmath.PoseAlmostEqual(p, expectedPose), test.ShouldBeTrue)

	p2, componentReference, err := slamSvc.Position(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, componentReference, test.ShouldEqual, expectedComponentReference)
	test.That(t, p, test.ShouldResemble, p2)
}

func TestFakeProperties(t *testing.T) {
	slamSvc := NewSLAM(slam.Named("test"), logging.NewTestLogger(t))

	prop, err := slamSvc.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop.CloudSlam, test.ShouldBeFalse)
	test.That(t, prop.MappingMode, test.ShouldEqual, slam.MappingModeNewMap)
}

func TestFakeSLAMStateful(t *testing.T) {
	t.Run("Test getting a PCD map via streaming APIs advances the test data", func(t *testing.T) {
		orgMaxDataCount := maxDataCount
		defer func() {
			maxDataCount = orgMaxDataCount
		}()
		// maxDataCount lowered under test to reduce test runtime
		maxDataCount = 5
		slamSvc := &SLAM{Named: slam.Named("test").AsNamed(), logger: logging.NewTestLogger(t)}
		verifyPointCloudMapStateful(t, slamSvc)
	})
}

func TestFakeSLAMInternalState(t *testing.T) {
	testName := "Returns a callback function which, returns the current fake internal state in chunks"
	t.Run(testName, func(t *testing.T) {
		slamSvc := NewSLAM(slam.Named("test"), logging.NewTestLogger(t))

		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(internalStateTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)

		data := getDataFromStream(t, slamSvc.InternalState)
		test.That(t, len(data), test.ShouldBeGreaterThan, 0)
		test.That(t, data, test.ShouldResemble, expectedData)

		data2 := getDataFromStream(t, slamSvc.InternalState)
		test.That(t, len(data2), test.ShouldBeGreaterThan, 0)
		test.That(t, data, test.ShouldResemble, data2)
		test.That(t, data2, test.ShouldResemble, expectedData)
	})
}

func TestFakeSLAMPointMap(t *testing.T) {
	testName := "Returns a callback function which, returns the current fake pointcloud map state in chunks and advances the dataset"
	t.Run(testName, func(t *testing.T) {
		slamSvc := NewSLAM(slam.Named("test"), logging.NewTestLogger(t))

		data := getDataFromStream(t, slamSvc.PointCloudMap)
		test.That(t, len(data), test.ShouldBeGreaterThan, 0)

		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, data, test.ShouldResemble, expectedData)

		data2 := getDataFromStream(t, slamSvc.PointCloudMap)
		test.That(t, len(data2), test.ShouldBeGreaterThan, 0)

		path2 := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData2, err := os.ReadFile(path2)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, data2, test.ShouldResemble, expectedData2)
		// Doesn't resemble as every call returns the next data set.
		test.That(t, data, test.ShouldNotResemble, data2)
	})
}

func getDataFromStream(t *testing.T, sFunc func(ctx context.Context) (func() ([]byte, error), error)) []byte {
	f, err := sFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f, test.ShouldNotBeNil)
	data, err := helperConcatenateChunksToFull(f)
	test.That(t, err, test.ShouldBeNil)
	return data
}

func reverse[T any](slice []T) []T {
	for i := len(slice)/2 - 1; i >= 0; i-- {
		opp := len(slice) - 1 - i
		slice[i], slice[opp] = slice[opp], slice[i]
	}
	return slice
}

func verifyPointCloudMapStateful(t *testing.T, slamSvc *SLAM) {
	testDataCount := maxDataCount
	getPointCloudMapResults := []float64{}
	getPositionResults := []spatialmath.Pose{}
	getInternalStateResults := []int{}

	// Call GetPointCloudMap twice for every testData artifact
	for i := 0; i < testDataCount*2; i++ {
		f, err := slamSvc.PointCloudMap(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		pcd, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		pc, err := pointcloud.ReadPCD(bytes.NewReader(pcd))
		test.That(t, err, test.ShouldBeNil)

		getPointCloudMapResults = append(getPointCloudMapResults, pc.MetaData().MaxX)
		test.That(t, err, test.ShouldBeNil)

		p, _, err := slamSvc.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		getPositionResults = append(getPositionResults, p)

		f, err = slamSvc.InternalState(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		internalState, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateResults = append(getInternalStateResults, len(internalState))
	}

	getPositionResultsFirst := getPositionResults[len(getPositionResults)/2:]
	getPositionResultsLast := getPositionResults[:len(getPositionResults)/2]

	getInternalStateResultsFirst := getInternalStateResults[len(getInternalStateResults)/2:]
	getInternalStateResultsLast := getInternalStateResults[:len(getInternalStateResults)/2]

	getPointCloudMapResultsFirst := getPointCloudMapResults[len(getPointCloudMapResults)/2:]
	getPointCloudMapResultsLast := getPointCloudMapResults[:len(getPointCloudMapResults)/2]

	// Confirm that the first half of the
	// results equal the last.
	// This proves that each call to GetPointCloudMap
	// advances the test data (both for GetPointCloudMap & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldResemble, getPositionResultsLast)
	test.That(t, getInternalStateResultsFirst, test.ShouldResemble, getInternalStateResultsLast)
	test.That(t, getPointCloudMapResultsFirst, test.ShouldResemble, getPointCloudMapResultsLast)

	// Confirm that the first half of the
	// results do NOT equal the last half in reverse.
	// This proves that each call to GetPointCloudMap
	// advances the test data (both for GetPointCloudMap & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldNotResemble, reverse(getPositionResultsLast))
	test.That(t, getInternalStateResultsFirst, test.ShouldNotResemble, reverse(getInternalStateResultsLast))
	test.That(t, getPointCloudMapResultsFirst, test.ShouldNotResemble, reverse(getPointCloudMapResultsLast))
}

func helperConcatenateChunksToFull(f func() ([]byte, error)) ([]byte, error) {
	var fullBytes []byte
	for {
		chunk, err := f()
		if errors.Is(err, io.EOF) {
			return fullBytes, nil
		}
		if err != nil {
			return nil, err
		}

		fullBytes = append(fullBytes, chunk...)
	}
}
