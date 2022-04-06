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

	// not aligned error
	ii, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	_, err = pp.ImageWithDepthToPointCloud(ii)
	test.That(t, err, test.ShouldBeError, errors.New("input ImageWithDepth is not aligned"))
	// no depth error
	ii = &ImageWithDepth{ii.Color, nil, true}
	_, err = pp.ImageWithDepthToPointCloud(ii)
	test.That(t, err, test.ShouldBeError, errors.New("input ImageWithDepth has no depth channel to project"))

	// parallel projection
	iwd, err := NewImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)

	vec3d, err := pp.ImagePointTo3DPoint(image.Point{4, 5}, 10)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vec3d, test.ShouldResemble, r3.Vector{4, 5, 10})

	pc, err := pp.ImageWithDepthToPointCloud(iwd)
	test.That(t, err, test.ShouldBeNil)
	data, got := pc.At(140, 500, float64(iwd.Depth.GetDepth(140, 500)))
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, NewColorFromColor(data.Color()), test.ShouldResemble, iwd.Color.GetXY(140, 500))

	pc2, err := pp.ImageWithDepthToPointCloud(iwd, image.Rectangle{image.Point{130, 490}, image.Point{150, 510}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc2.Size(), test.ShouldEqual, 400)
	data, got = pc2.At(140, 500, float64(iwd.Depth.GetDepth(140, 500)))
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, NewColorFromColor(data.Color()), test.ShouldResemble, iwd.Color.GetXY(140, 500))

	_, err = pp.ImageWithDepthToPointCloud(iwd, image.Rectangle{image.Point{130, 490}, image.Point{150, 510}}, image.Rectangle{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one cropping rectangle")

	iwd2, err := pp.PointCloudToImageWithDepth(pc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd2.Color.GetXY(140, 500), test.ShouldResemble, iwd.Color.GetXY(140, 500))
	test.That(t, iwd2.Depth.GetDepth(140, 500), test.ShouldResemble, iwd.Depth.GetDepth(140, 500))
}
