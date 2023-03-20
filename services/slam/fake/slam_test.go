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
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFakeSLAMPosition(t *testing.T) {
	slamSvc := NewSLAM("test", golog.NewTestLogger(t))
	pInFrame, err := slamSvc.Position(context.Background(), slamSvc.Name, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pInFrame.Parent(), test.ShouldEqual, slamSvc.Name)

	// test.ShouldBeBetween is used here as tiny differences were observed
	// in floating point values between M1 mac & arm64 linux which
	// were causing tests to pass on M1 mac but fail on ci.
	test.That(t, pInFrame.Pose().Point().X, test.ShouldBeBetween, -0.005885172861759, -0.005885172861758)
	test.That(t, pInFrame.Pose().Point().Y, test.ShouldBeBetween, 0.0132681742800635, 0.0132681742800636)
	test.That(t, pInFrame.Pose().Point().Z, test.ShouldEqual, 0)

	expectedOri := &spatialmath.Quaternion{Real: 0.9999998369888826, Imag: 0, Jmag: 0, Kmag: -0.0005709835448716814}
	test.That(t, pInFrame.Pose().Orientation(), test.ShouldResemble, expectedOri)

	pInFrame2, err := slamSvc.Position(context.Background(), slamSvc.Name, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pInFrame2, test.ShouldResemble, pInFrame2)
}

func TestFakeSLAMGetPosition(t *testing.T) {
	expectedComponentReference := ""
	slamSvc := NewSLAM("test", golog.NewTestLogger(t))

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

func TestFakeSLAMGetInternalState(t *testing.T) {
	slamSvc := NewSLAM("test", golog.NewTestLogger(t))
	data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(data), test.ShouldBeGreaterThan, 0)
	data2, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data, test.ShouldResemble, data2)
}

func TestFakeSLAMStateful(t *testing.T) {
	t.Run("Test getting a PCD map advances the test data", func(t *testing.T) {
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))
		extra := map[string]interface{}{}
		verifyGetMapStateful(t, rdkutils.MimeTypePCD, slamSvc, extra)
	})

	t.Run("Test getting a PCD map via streaming APIs advances the test data", func(t *testing.T) {
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))
		extra := map[string]interface{}{}
		verifyGetPointCloudMapStreamStateful(t, slamSvc, extra)
	})
}

func TestFakeSLAMGetMap(t *testing.T) {
	extra := map[string]interface{}{}

	t.Run("Test getting valid JPEG map", func(t *testing.T) {
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))
		pInFrame := referenceframe.NewPoseInFrame(
			slamSvc.Name,
			spatialmath.NewPose(
				r3.Vector{X: 0, Y: 0, Z: 0},
				spatialmath.NewOrientationVector(),
			),
		)
		mimeType, im, vObj, err := slamSvc.GetMap(
			context.Background(),
			slamSvc.Name,
			rdkutils.MimeTypeJPEG,
			pInFrame,
			true,
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, rdkutils.MimeTypeJPEG)
		test.That(t, vObj, test.ShouldBeNil)
		test.That(t, im, test.ShouldNotBeNil)
		test.That(t, im.Bounds().Max.X, test.ShouldEqual, 1265)
		test.That(t, im.Bounds().Max.Y, test.ShouldEqual, 785)
		test.That(t, im.Bounds().Min.X, test.ShouldEqual, 0)
		test.That(t, im.Bounds().Min.Y, test.ShouldEqual, 0)
	})

	t.Run("Test getting invalid PNG map", func(t *testing.T) {
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))
		pInFrame := referenceframe.NewPoseInFrame(
			slamSvc.Name,
			spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0},
				spatialmath.NewOrientationVector()),
		)
		mimeType, im, vObj, err := slamSvc.GetMap(
			context.Background(),
			slamSvc.Name,
			rdkutils.MimeTypePNG,
			pInFrame,
			true,
			extra,
		)
		test.That(t, err, test.ShouldBeError, "received invalid mimeType for GetMap call")
		test.That(t, mimeType, test.ShouldEqual, "")
		test.That(t, vObj, test.ShouldBeNil)
		test.That(t, im, test.ShouldBeNil)
	})
}

func TestFakeSLAMGetInternalStateStream(t *testing.T) {
	testName := "Returns a callback function which, returns the current fake internal state in chunks"
	t.Run(testName, func(t *testing.T) {
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))

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
		slamSvc := NewSLAM("test", golog.NewTestLogger(t))

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

func verifyGetMapStateful(t *testing.T, mimeType string, slamSvc *SLAM, extra map[string]interface{}) {
	testDataCount := maxDataCount
	getMapPcdResults := []float64{}
	getPositionResults := []spatialmath.Pose{}
	getInternalStateResults := []int{}

	// Call GetMap twice for every testData artifact
	for i := 0; i < testDataCount*2; i++ {
		_, _, vObj, err := slamSvc.GetMap(
			context.Background(),
			slamSvc.Name,
			mimeType,
			&referenceframe.PoseInFrame{},
			true,
			extra,
		)

		getMapPcdResults = append(getMapPcdResults, vObj.MetaData().MaxX)
		test.That(t, err, test.ShouldBeNil)

		pInFrame, err := slamSvc.Position(context.Background(), slamSvc.Name, extra)
		test.That(t, err, test.ShouldBeNil)
		getPositionResults = append(getPositionResults, pInFrame.Pose())

		data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateResults = append(getInternalStateResults, len(data))
	}

	getPositionResultsFirst := getPositionResults[len(getPositionResults)/2:]
	getPositionResultsLast := getPositionResults[:len(getPositionResults)/2]

	getInternalStateResultsFirst := getInternalStateResults[len(getInternalStateResults)/2:]
	getInternalStateResultsLast := getInternalStateResults[:len(getInternalStateResults)/2]

	// Confirm that the first half of the
	// results equal the last.
	// This proves that each call to GetMap
	// advances the test data (both for GetMap & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldResemble, getPositionResultsLast)
	test.That(t, getInternalStateResultsFirst, test.ShouldResemble, getInternalStateResultsLast)

	// Confirm that the first half of the
	// results does NOT equal the last half in reverse.
	// This proves that each call to GetMap
	// advances the test data (both for GetMap & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, getPositionResultsFirst, test.ShouldNotResemble, reverse(getPositionResultsLast))
	test.That(t, getInternalStateResultsFirst, test.ShouldNotResemble, reverse(getInternalStateResultsLast))

	supportedMimeTypes := []string{rdkutils.MimeTypePCD, rdkutils.MimeTypeJPEG}
	test.That(t, supportedMimeTypes, test.ShouldContain, mimeType)
	getMapResultsFirst := getMapPcdResults[len(getMapPcdResults)/2:]
	getMapResultsLast := getMapPcdResults[:len(getMapPcdResults)/2]
	test.That(t, getMapResultsFirst, test.ShouldResemble, getMapResultsLast)
	test.That(t, getMapResultsFirst, test.ShouldNotResemble, reverse(getMapResultsLast))
}

func reverse[T any](slice []T) []T {
	for i := len(slice)/2 - 1; i >= 0; i-- {
		opp := len(slice) - 1 - i
		slice[i], slice[opp] = slice[opp], slice[i]
	}
	return slice
}

func verifyGetPointCloudMapStreamStateful(t *testing.T, slamSvc *SLAM, extra map[string]interface{}) {
	testDataCount := maxDataCount
	getPointCloudMapResults := []float64{}
	positionResults := []spatialmath.Pose{}
	getPositionResults := []spatialmath.Pose{}
	getInternalStateResults := []int{}
	getInternalStateStreamResults := []int{}

	// Call GetPointCloudMapStream twice for every testData artifact
	for i := 0; i < testDataCount*2; i++ {
		f, err := slamSvc.GetPointCloudMapStream(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		pcd, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		pc, err := pointcloud.ReadPCD(bytes.NewReader(pcd))
		test.That(t, err, test.ShouldBeNil)

		getPointCloudMapResults = append(getPointCloudMapResults, pc.MetaData().MaxX)
		test.That(t, err, test.ShouldBeNil)

		pInFrame, err := slamSvc.Position(context.Background(), slamSvc.Name, extra)
		test.That(t, err, test.ShouldBeNil)
		positionResults = append(positionResults, pInFrame.Pose())

		p, _, err := slamSvc.GetPosition(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		getPositionResults = append(getPositionResults, p)

		data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateResults = append(getInternalStateResults, len(data))

		f, err = slamSvc.GetInternalStateStream(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, f, test.ShouldNotBeNil)
		internalState, err := helperConcatenateChunksToFull(f)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateStreamResults = append(getInternalStateStreamResults, len(internalState))
	}

	getPositionResultsFirst := getPositionResults[len(getPositionResults)/2:]
	getPositionResultsLast := getPositionResults[:len(getPositionResults)/2]

	positionResultsFirst := positionResults[len(positionResults)/2:]
	positionResultsLast := positionResults[:len(positionResults)/2]

	getInternalStateResultsFirst := getInternalStateResults[len(getInternalStateResults)/2:]
	getInternalStateResultsLast := getInternalStateResults[:len(getInternalStateResults)/2]

	getInternalStateStreamResultsFirst := getInternalStateStreamResults[len(getInternalStateStreamResults)/2:]
	getInternalStateStreamResultsLast := getInternalStateStreamResults[:len(getInternalStateStreamResults)/2]

	// Confirm that the first half of the
	// results equal the last.
	// This proves that each call to GetPointCloudMapStream
	// advances the test data (both for GetPointCloudMapStream & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, positionResultsFirst, test.ShouldResemble, positionResultsLast)
	test.That(t, getPositionResultsFirst, test.ShouldResemble, getPositionResultsLast)
	test.That(t, getInternalStateResultsFirst, test.ShouldResemble, getInternalStateResultsLast)
	test.That(t, getInternalStateStreamResultsFirst, test.ShouldResemble, getInternalStateStreamResultsLast)

	// Confirm that the first half of the
	// results do NOT equal the last half in reverse.
	// This proves that each call to GetPointCloudMapStream
	// advances the test data (both for GetPointCloudMapStream & other endpoints)
	// over a dataset of size maxDataCount that loops around.
	test.That(t, positionResultsFirst, test.ShouldNotResemble, reverse(positionResultsLast))
	test.That(t, getPositionResultsFirst, test.ShouldNotResemble, reverse(getPositionResultsLast))
	test.That(t, getInternalStateResultsFirst, test.ShouldNotResemble, reverse(getInternalStateResultsLast))
	test.That(t, getInternalStateStreamResultsFirst, test.ShouldNotResemble, reverse(getInternalStateStreamResultsLast))

	getMapResultsFirst := getPointCloudMapResults[len(getPointCloudMapResults)/2:]
	getMapResultsLast := getPointCloudMapResults[:len(getPointCloudMapResults)/2]
	test.That(t, getMapResultsFirst, test.ShouldResemble, getMapResultsLast)
	test.That(t, getMapResultsFirst, test.ShouldNotResemble, reverse(getMapResultsLast))
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
