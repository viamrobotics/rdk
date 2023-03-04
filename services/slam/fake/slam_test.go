package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFakeSLAMPosition(t *testing.T) {
	slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
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

func TestFakeSLAMGetInternalState(t *testing.T) {
	slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
	data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(data), test.ShouldBeGreaterThan, 0)
	data2, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data, test.ShouldResemble, data2)
}

func TestFakeSLAMStateful(t *testing.T) {
	t.Run("Test getting a PCD map advances the test data", func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
		extra := map[string]interface{}{}
		verifyGetMapStateful(t, rdkutils.MimeTypePCD, slamSvc, extra)
	})
}

func TestFakeSLAMGetMap(t *testing.T) {
	extra := map[string]interface{}{}

	t.Run("Test getting valid JPEG map", func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
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
		test.That(t, im.Bounds().Max.X, test.ShouldEqual, 3642)
		test.That(t, im.Bounds().Max.Y, test.ShouldEqual, 2323)
		test.That(t, im.Bounds().Min.X, test.ShouldEqual, 0)
		test.That(t, im.Bounds().Min.Y, test.ShouldEqual, 0)
	})

	t.Run("Test getting invalid PNG map", func(t *testing.T) {
		slamSvc := &SLAM{Name: "test", logger: golog.NewTestLogger(t)}
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

func verifyGetMapStateful(t *testing.T, mimeType string, slamSvc *SLAM, extra map[string]interface{}) {
	testDataCount := maxDataCount
	getMapPcdResults := []float64{}
	getPositionResults := []spatialmath.Pose{}
	getInternalStateResults := []int{}

	// Call GetMap twice for every testData artifact
	for i := 0; i < testDataCount*2; i++ {
		pInFrame, err := slamSvc.Position(context.Background(), slamSvc.Name, extra)
		test.That(t, err, test.ShouldBeNil)
		getPositionResults = append(getPositionResults, pInFrame.Pose())

		data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
		test.That(t, err, test.ShouldBeNil)
		getInternalStateResults = append(getInternalStateResults, len(data))

		_, _, vObj, err := slamSvc.GetMap(
			context.Background(),
			slamSvc.Name,
			mimeType,
			pInFrame,
			true,
			extra,
		)

		getMapPcdResults = append(getMapPcdResults, vObj.MetaData().MaxX)
		test.That(t, err, test.ShouldBeNil)
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
