package fake

import (
	"context"
	"image"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

func TestFakeSLAMPosition(t *testing.T) {
	slamSvc := &FakeSLAM{Name: "test"}
	pInFrame, err := slamSvc.Position(context.Background(), slamSvc.Name, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pInFrame.Parent(), test.ShouldEqual, slamSvc.Name)

	expectedPoint := r3.Vector{X: 1.8607751801785188, Y: 34.26593374183797, Z: 0}
	test.That(t, pInFrame.Pose().Point(), test.ShouldResemble, expectedPoint)

	expectedOri := spatialmath.NewR4AA()
	expectedOri.RZ = -1
	expectedOri.Theta = 3.0542483867902197
	test.That(t, pInFrame.Pose().Orientation().AxisAngles(), test.ShouldResemble, expectedOri)
}

func TestFakeSLAMGetInternalState(t *testing.T) {
	slamSvc := &FakeSLAM{Name: "test"}
	data, err := slamSvc.GetInternalState(context.Background(), slamSvc.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(data), test.ShouldBeGreaterThan, 0)
}

func TestFakeSLAMGetMap(t *testing.T) {
	slamSvc := &FakeSLAM{Name: "test"}
	pInFrame := referenceframe.NewPoseInFrame(slamSvc.Name, spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector()))
	extra := map[string]interface{}{}

	t.Run("Test getting a PCD map advances the test data", func(t *testing.T) {
		testDataCount := maxDataCount
		results := []*vision.Object{}
		// Call GetMap twice for every testData artifact
		for i := 0; i < testDataCount*2; i++ {
			_, _, vObj, err := slamSvc.GetMap(
				context.Background(),
				slamSvc.Name,
				rdkutils.MimeTypePCD,
				pInFrame,
				true,
				extra,
			)
			results = append(results, vObj)

			test.That(t, err, test.ShouldBeNil)
		}

		// Confirm that the first half of vObjs
		// is equal to the last
		// This proves that each call to GetMap
		// advances the test data over a finite
		// dataset that loops around
		test.That(t, results[len(results)/2:], test.ShouldResemble, results[:len(results)/2])
	})

	t.Run("Test getting a JPEG map advances the test data", func(t *testing.T) {
		testDataCount := maxDataCount
		results := []image.Image{}
		// Call GetMap twice for every testData artifact
		for i := 0; i < testDataCount*2; i++ {
			_, im, _, err := slamSvc.GetMap(
				context.Background(),
				slamSvc.Name,
				rdkutils.MimeTypeJPEG,
				pInFrame,
				true,
				extra,
			)
			test.That(t, err, test.ShouldBeNil)
			results = append(results, im)
		}

		// Confirm that the first half of vObjs
		// is equal to the last
		// This proves that each call to GetMap
		// advances the test data over a finite
		// dataset that loops around
		test.That(t, results[len(results)/2:], test.ShouldResemble, results[:len(results)/2])
	})

	t.Run("Test getting valid PCD map", func(t *testing.T) {
		mimeType, im, vObj, err := slamSvc.GetMap(
			context.Background(),
			slamSvc.Name,
			rdkutils.MimeTypePCD,
			pInFrame,
			true,
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, rdkutils.MimeTypePCD)
		test.That(t, im, test.ShouldBeNil)
		test.That(t, vObj, test.ShouldNotBeNil)
		test.That(t, vObj.Size(), test.ShouldEqual, 247)
	})

	t.Run("Test getting valid JPEG map", func(t *testing.T) {
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
		test.That(t, im.Bounds().Max.X, test.ShouldEqual, 1925)
		test.That(t, im.Bounds().Max.Y, test.ShouldEqual, 5299)
		test.That(t, im.Bounds().Min.X, test.ShouldEqual, 0)
		test.That(t, im.Bounds().Min.Y, test.ShouldEqual, 0)
	})

	t.Run("Test getting invalid PNG map", func(t *testing.T) {
		mimeType, im, vObj, err := slamSvc.GetMap(context.Background(), slamSvc.Name, rdkutils.MimeTypePNG, pInFrame, true, extra)
		test.That(t, err, test.ShouldBeError, "received invalid mimeType for GetMap call")
		test.That(t, mimeType, test.ShouldEqual, "")
		test.That(t, vObj, test.ShouldBeNil)
		test.That(t, im, test.ShouldBeNil)
	})
}
