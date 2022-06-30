package rimage

import (
	"image"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestParallelProjection(t *testing.T) {
	pp := ParallelProjection{}

	// load the images
	img, err := NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	img2, err := NewImageFromFile(artifact.MustPath("rimage/circle.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)
	// no image error
	_, err = pp.RGBDToPointCloud(nil, dm)
	test.That(t, err, test.ShouldBeError, errors.New("no rgb image to project to pointcloud"))
	// no depth error
	_, err = pp.RGBDToPointCloud(img, nil)
	test.That(t, err, test.ShouldBeError, errors.New("no depth map to project to pointcloud"))
	// not the same size
	_, err = pp.RGBDToPointCloud(img2, dm)
	test.That(t, err.String(), test.ShouldContainSubstring, "rgb image and depth map are not the same size")

	// parallel projection
	vec3d, err := pp.ImagePointTo3DPoint(image.Point{4, 5}, 10)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vec3d, test.ShouldResemble, r3.Vector{4, 5, 10})

	pc, err := pp.RGBDToPointCloud(img, dm)
	test.That(t, err, test.ShouldBeNil)
	data, got := pc.At(140, 500, float64(iwd.Depth.GetDepth(140, 500)))
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, NewColorFromColor(data.Color()), test.ShouldResemble, iwd.Color.GetXY(140, 500))

	pc2, err := pp.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{130, 490}, image.Point{150, 510}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc2.Size(), test.ShouldEqual, 400)
	data, got = pc2.At(140, 500, float64(iwd.Depth.GetDepth(140, 500)))
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, NewColorFromColor(data.Color()), test.ShouldResemble, iwd.Color.GetXY(140, 500))

	_, err = pp.RGBDToPointCloud(img, dm, image.Rectangle{image.Point{130, 490}, image.Point{150, 510}}, image.Rectangle{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one cropping rectangle")

	img3, dm3, err := pp.PointCloudToRGBD(pc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img3.GetXY(140, 500), test.ShouldResemble, img.GetXY(140, 500))
	test.That(t, dm3.GetDepth(140, 500), test.ShouldResemble, dm.GetDepth(140, 500))
}
