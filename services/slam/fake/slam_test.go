package fake

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
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

	t.Run("Test getting valid PCD map", func(t *testing.T) {
		mimeType, im, vObj, err := slamSvc.GetMap(context.Background(), slamSvc.Name, rdkutils.MimeTypePCD, pInFrame, true, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, rdkutils.MimeTypePCD)
		test.That(t, im, test.ShouldBeNil)
		test.That(t, vObj, test.ShouldNotBeNil)
	})

	t.Run("Test getting valid JPEG map", func(t *testing.T) {
		mimeType, im, vObj, err := slamSvc.GetMap(context.Background(), slamSvc.Name, rdkutils.MimeTypeJPEG, pInFrame, true, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mimeType, test.ShouldEqual, rdkutils.MimeTypeJPEG)
		test.That(t, vObj, test.ShouldBeNil)
		test.That(t, im, test.ShouldNotBeNil)
	})

	t.Run("Test getting invalid PNG map", func(t *testing.T) {
		mimeType, im, vObj, err := slamSvc.GetMap(context.Background(), slamSvc.Name, rdkutils.MimeTypePNG, pInFrame, true, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, mimeType, test.ShouldEqual, "")
		test.That(t, vObj, test.ShouldBeNil)
		test.That(t, im, test.ShouldBeNil)
	})
}
