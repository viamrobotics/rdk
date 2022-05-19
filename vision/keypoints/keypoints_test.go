package keypoints

import (
	"go.viam.com/rdk/utils"
	"image"
	"math/rand"
	"testing"

	"go.viam.com/test"
)

func generateRandomKeypoint(max int) image.Point {
	x := utils.AbsInt(rand.Intn(max))
	y := utils.AbsInt(rand.Intn(max))
	return image.Point{x, y}
}

func TestRescaleKeypoints(t *testing.T) {
	kps := make(KeyPoints, 10)
	rescaledKeypoints := RescaleKeypoints(kps, 2)
	test.That(t, rescaledKeypoints[0], test.ShouldResemble, kps[0])

	// test on slice of random keypoints
	kps2 := make(KeyPoints, 2)
	kps2[0] = generateRandomKeypoint(320)
	kps2[1] = generateRandomKeypoint(320)
	rescaledKeypoints1 := RescaleKeypoints(kps2, 1)
	test.That(t, rescaledKeypoints1[0], test.ShouldResemble, kps2[0])
	test.That(t, rescaledKeypoints1[1], test.ShouldResemble, kps2[1])
	rescaledKeypoints2 := RescaleKeypoints(kps2, 2)
	test.That(t, rescaledKeypoints2[0].X, test.ShouldEqual, kps2[0].X*2)
	test.That(t, rescaledKeypoints2[0].Y, test.ShouldEqual, kps2[0].Y*2)
	test.That(t, rescaledKeypoints2[1].X, test.ShouldEqual, kps2[1].X*2)
	test.That(t, rescaledKeypoints2[1].Y, test.ShouldEqual, kps2[1].Y*2)
}
