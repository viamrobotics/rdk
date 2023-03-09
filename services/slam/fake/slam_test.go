package fake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

func TestFakeSLAMGetPosition(t *testing.T) {
	expectedComponentReference := ""
	slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}

	p, componentReference, err := slamSvc.GetPosition(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, componentReference, test.ShouldEqual, expectedComponentReference)

	// spatialmath.PoseAlmostEqual is used here as tiny differences were observed
	// in floating point values between M1 mac & arm64 linux which
	// were causing tests to pass on M1 mac but fail on ci.
	expectedPose := spatialmath.NewPose(
		r3.Vector{X: -0.005666600181385561, Y: -6.933830159344678e-10, Z: -0.013030459250151614},
		&spatialmath.Quaternion{Real: 0.9999999087728241, Imag: 0, Jmag: 0.0005374749356603168, Kmag: 0})
	test.That(t, spatialmath.PoseAlmostEqual(p, expectedPose), test.ShouldBeTrue)

	p2, componentReference, err := slamSvc.GetPosition(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, componentReference, test.ShouldEqual, expectedComponentReference)
	test.That(t, p, test.ShouldResemble, p2)
}

func TestFakeSLAMStateful(t *testing.T) {
	t.Run("Test getting a PCD map via streaming APIs advances the test data", func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
		verifyGetPointCloudMapStreamStateful(t, slamSvc)
	})
}

func TestFakeSLAMGetInternalStateStream(t *testing.T) {
	testName := "Returns a callback function which, returns the current fake internal state in chunks"
	t.Run(testName, func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}

		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(internalStateTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)

		data := getDataFromStream(t, slamSvc.GetInternalStateStream, slamSvc.Name)
		test.That(t, len(data), test.ShouldBeGreaterThan, 0)
		test.That(t, data, test.ShouldResemble, expectedData)

		data2 := getDataFromStream(t, slamSvc.GetInternalStateStream, slamSvc.Name)
		test.That(t, len(data2), test.ShouldBeGreaterThan, 0)
		test.That(t, data, test.ShouldResemble, data2)
		test.That(t, data2, test.ShouldResemble, expectedData)
	})
}

func TestFakeSLAMGetPointMapStream(t *testing.T) {
	testName := "Returns a callback function which, returns the current fake pointcloud map state in chunks and advances the dataset"
	t.Run(testName, func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}

		data := getDataFromStream(t, slamSvc.GetPointCloudMapStream, slamSvc.Name)
		test.That(t, len(data), test.ShouldBeGreaterThan, 0)

		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, data, test.ShouldResemble, expectedData)

		data2 := getDataFromStream(t, slamSvc.GetPointCloudMapStream, slamSvc.Name)
		test.That(t, len(data2), test.ShouldBeGreaterThan, 0)

		path2 := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDirectory, slamSvc.getCount())))
		expectedData2, err := os.ReadFile(path2)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, data2, test.ShouldResemble, expectedData2)
		// Doesn't resemble as every call returns the next data set.
		test.That(t, data, test.ShouldNotResemble, data2)
	})
}

func getDataFromStream(
	t *testing.T,
	sFunc func(ctx context.Context, name string) (func() ([]byte, error), error),
	name string,
) []byte {
	f, err := sFunc(context.Background(), name)
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

func verifyGetPointCloudMapStreamStateful(t *testing.T, slamSvc *SLAM) {
	testDataCount := maxDataCount
	getPointCloudMapResults := []float64{}
	getPositionResults := []spatialmath.Pose{}
	getInternalStateStreamResults := []int{}

	// Call GetPointCloudMapStream twice for every testData artifact
	for i := 0; i < testDataCount*2; i++ {
		p, _, err := slamSvc.GetPosition(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		getPositionResults = append(getPositionResults, p)

		f, err := slamSvc.GetInternalStateStream(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		internalState, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateStreamResults = append(getInternalStateStreamResults, len(internalState))

		f, err = slamSvc.GetPointCloudMapStream(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		pcd, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		pc, err := pointcloud.ReadPCD(bytes.NewReader(pcd))
		test.That(t, err, test.ShouldBeNil)

		getPointCloudMapResults = append(getPointCloudMapResults, pc.MetaData().MaxX)
		test.That(t, err, test.ShouldBeNil)
	}

	getPositionResultsFirst := getPositionResults[len(getPositionResults)/2:]
	getPositionResultsLast := getPositionResults[:len(getPositionResults)/2]

	getInternalStateStreamResultsFirst := getInternalStateStreamResults[len(getInternalStateStreamResults)/2:]
	getInternalStateStreamResultsLast := getInternalStateStreamResults[:len(getInternalStateStreamResults)/2]

	getPointCloudMapResultsFirst := getPointCloudMapResults[len(getPointCloudMapResults)/2:]
	getPointCloudMapResultsLast := getPointCloudMapResults[:len(getPointCloudMapResults)/2]

	// Confirm that the first half of the
	// results equal the last.
	// This proves that each call to GetPointCloudMapStream
	// advances the test data (both for GetPointCloudMapStream & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldResemble, getPositionResultsLast)
	test.That(t, getInternalStateStreamResultsFirst, test.ShouldResemble, getInternalStateStreamResultsLast)
	test.That(t, getPointCloudMapResultsFirst, test.ShouldResemble, getPointCloudMapResultsLast)

	// Confirm that the first half of the
	// results do NOT equal the last half in reverse.
	// This proves that each call to GetPointCloudMapStream
	// advances the test data (both for GetPointCloudMapStream & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldNotResemble, reverse(getPositionResultsLast))
	test.That(t, getInternalStateStreamResultsFirst, test.ShouldNotResemble, reverse(getInternalStateStreamResultsLast))
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
