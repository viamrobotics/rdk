package rimage

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/golang/geo/r2"
	"github.com/lucasb-eyer/go-colorful"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestCanny1(t *testing.T) {
	t.Parallel()
	doCannyTest(t, "canny1")
}

func TestCanny2(t *testing.T) {
	t.Parallel()
	doCannyTest(t, "canny2")
}

func doCannyTest(t *testing.T, root string) {
	t.Helper()
	img, err := NewImageFromFile(artifact.MustPath(fmt.Sprintf("rimage/%s.png", root)))
	test.That(t, err, test.ShouldBeNil)

	goodOnes := 0

	for a := 0.01; a <= .05; a += .005 {
		outfn := fmt.Sprintf(t.TempDir()+"/%s-%v.png", root, int(1000*a))
		t.Log(outfn)

		out, err := SimpleEdgeDetection(img, a, 3.0)
		test.That(t, err, test.ShouldBeNil)

		err = WriteImageToFile(outfn, out)
		test.That(t, err, test.ShouldBeNil)

		bad := false

		for x := 50; x <= 750; x += 100 {
			for y := 50; y <= 750; y += 100 {
				spots := CountBrightSpots(out, image.Point{x, y}, 25, 255)

				if y < 200 || y > 600 {
					if spots < 100 {
						bad = true
						t.Logf("\t%v,%v %v\n", x, y, spots)
					}
				} else {
					if spots > 90 {
						bad = true
						t.Logf("\t%v,%v %v\n", x, y, spots)
					}
				}

				if bad {
					break
				}
			}

			if bad {
				break
			}
		}

		if bad {
			continue
		}

		goodOnes++

		break
	}

	test.That(t, goodOnes, test.ShouldNotEqual, 0)
}

func TestCloneImage(t *testing.T) {
	t.Parallel()
	img, err := readImageFromFile(artifact.MustPath("rimage/canny1.png"))
	test.That(t, err, test.ShouldBeNil)

	// Image path
	i := ConvertImage(img)
	ii := CloneImage(i)
	for y := 0; y < ii.Height(); y++ {
		for x := 0; x < ii.Width(); x++ {
			test.That(t, ii.GetXY(x, y), test.ShouldResemble, i.GetXY(x, y))
		}
	}
	ii.SetXY(0, 0, Red)
	test.That(t, ii.GetXY(0, 0), test.ShouldNotResemble, i.GetXY(0, 0))

	// ImageWithDepth path
	j := convertToImageWithDepth(img)
	ii = CloneImage(j)
	for y := 0; y < ii.Height(); y++ {
		for x := 0; x < ii.Width(); x++ {
			test.That(t, ii.GetXY(x, y), test.ShouldResemble, i.GetXY(x, y))
		}
	}
	ii.SetXY(0, 0, Red)
	test.That(t, ii.GetXY(0, 0), test.ShouldNotResemble, i.GetXY(0, 0))
}

func BenchmarkConvertImage(b *testing.B) {
	img, err := readImageFromFile(artifact.MustPath("rimage/canny1.png"))
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		ConvertImage(img)
	}
}

func TestConvertYCbCr(t *testing.T) {
	orig, err := readImageFromFile(artifact.MustPath("rimage/canny1.png"))
	test.That(t, err, test.ShouldBeNil)

	var yuvImg image.YCbCr
	ImageToYCbCrForTesting(&yuvImg, orig)

	err = WriteImageToFile(t.TempDir()+"/canny1-ycbcr.png", &yuvImg)
	test.That(t, err, test.ShouldBeNil)

	c1, b1 := colorful.MakeColor(orig.At(100, 100))
	c2, b2 := colorful.MakeColor(yuvImg.At(100, 100))
	test.That(t, b1 || b2, test.ShouldBeTrue)

	test.That(t, c2.Hex(), test.ShouldEqual, c1.Hex())
}

func BenchmarkConvertImageYCbCr(b *testing.B) {
	orig, err := readImageFromFile(artifact.MustPath("rimage/canny1.png"))
	test.That(b, err, test.ShouldBeNil)

	var yuvImg image.YCbCr
	ImageToYCbCrForTesting(&yuvImg, orig)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		ConvertImage(&yuvImg)
	}
}

func TestColorInterpolation(t *testing.T) {
	img := NewImage(2, 2)
	img.SetXY(0, 0, NewColorFromHSV(30, 0, 0))
	img.SetXY(1, 0, NewColorFromHSV(40, .5, .5))
	img.SetXY(0, 1, NewColorFromHSV(50, .5, .5))
	img.SetXY(1, 1, NewColorFromHSV(60, 1.0, 1.0))

	pt := r2.Point{0.5, 0.5}
	c := BilinearInterpolationColor(pt, img)
	h, s, v := c.HsvNormal()
	test.That(t, math.Abs(h-45), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(s-0.5), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(v-0.5), test.ShouldBeLessThan, .01)
	c = NearestNeighborColor(pt, img)
	test.That(t, *c, test.ShouldResemble, NewColorFromHSV(60, 1.0, 1.0))

	pt = r2.Point{0.75, 0.25}
	c = BilinearInterpolationColor(pt, img)
	h, s, v = c.HsvNormal()
	test.That(t, math.Abs(h-42.46), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(s-0.5), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(v-0.5), test.ShouldBeLessThan, .01)
	c = NearestNeighborColor(pt, img)
	test.That(t, *c, test.ShouldResemble, NewColorFromHSV(40, 0.5, 0.5))

	pt = r2.Point{1.0, 1.0}
	c = BilinearInterpolationColor(pt, img)
	h, s, v = c.HsvNormal()
	test.That(t, math.Abs(h-60), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(s-1.), test.ShouldBeLessThan, .01)
	test.That(t, math.Abs(v-1.), test.ShouldBeLessThan, .01)
	c = NearestNeighborColor(pt, img)
	test.That(t, *c, test.ShouldResemble, NewColorFromHSV(60, 1.0, 1.0))

	pt = r2.Point{1.1, 1.0}
	c = BilinearInterpolationColor(pt, img)
	test.That(t, c, test.ShouldBeNil)
	c = NearestNeighborColor(pt, img)
	test.That(t, c, test.ShouldBeNil)
}

func TestCanny(t *testing.T) {
	imgOriginal, err := readImageFromFile(artifact.MustPath("rimage/canny_test_1.jpg"))
	test.That(t, err, test.ShouldBeNil)
	img := ConvertImage(imgOriginal)
	test.That(t, err, test.ShouldBeNil)

	gtOriginal, err := readImageFromFile(artifact.MustPath("rimage/test_canny.png"))
	gt := ConvertImage(gtOriginal)
	test.That(t, err, test.ShouldBeNil)

	cannyDetector := NewCannyDericheEdgeDetector()
	edgesMat, _ := cannyDetector.DetectEdges(img, 0.5)
	edges := ConvertImage(edgesMat)
	test.That(t, len(gt.data), test.ShouldEqual, len(edges.data))
	test.That(t, gt.data, test.ShouldResemble, edges.data)
}

func TestCannyBlocks(t *testing.T) {
	// load test image and GT
	imgOriginal, err := readImageFromFile(artifact.MustPath("rimage/edge_test_image.png"))
	test.That(t, err, test.ShouldBeNil)
	img := ConvertImage(imgOriginal)
	test.That(t, err, test.ShouldBeNil)
	gtGradient, err := readImageFromFile(artifact.MustPath("rimage/edge_test_gradient.png"))
	gtGrad := ConvertImage(gtGradient)
	test.That(t, err, test.ShouldBeNil)
	gtNonMaxSup, err := readImageFromFile(artifact.MustPath("rimage/edge_test_nms.png"))
	gtNms := ConvertImage(gtNonMaxSup)
	test.That(t, err, test.ShouldBeNil)
	// Compute forward gradient
	imgGradient, _ := ForwardGradient(img, 0., false)
	magData := imgGradient.Magnitude.RawMatrix().Data
	magDataInt := make([]Color, len(magData))
	for idx := 0; idx < len(magData); idx++ {
		magDataInt[idx] = NewColor(uint8(math.Round(magData[idx])), uint8(math.Round(magData[idx])), uint8(math.Round(magData[idx])))
	}
	magOut := Image{
		data: magDataInt,
	}

	// NMS
	nms, _ := GradientNonMaximumSuppressionC8(imgGradient.Magnitude, imgGradient.Direction)
	nmsData := nms.RawMatrix().Data
	nmsDataInt := make([]Color, len(nmsData))
	for idx := 0; idx < len(magData); idx++ {
		nmsDataInt[idx] = NewColor(uint8(math.Round(nmsData[idx])), uint8(math.Round(nmsData[idx])), uint8(math.Round(nmsData[idx])))
	}
	nmsOut := Image{
		data: nmsDataInt,
	}

	// run tests
	tests := []struct {
		testName        string
		dataOut, dataGT []Color
	}{
		{"gradient", magOut.data, gtGrad.data},
		{"nms", nmsOut.data, gtNms.data},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			test.That(t, len(tt.dataOut), test.ShouldEqual, len(tt.dataGT))
			test.That(t, tt.dataOut, test.ShouldResemble, tt.dataGT)
		})
	}
}
