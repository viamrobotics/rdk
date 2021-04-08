package rimage

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/stretchr/testify/assert"
)

func TestCanny1(t *testing.T) {
	doCannyTest(t, "canny1")
}

func TestCanny2(t *testing.T) {
	doCannyTest(t, "canny2")
}

func doCannyTest(t *testing.T, root string) {
	img, err := NewImageFromFile(fmt.Sprintf("data/%s.png", root))
	if err != nil {
		t.Fatal(err)
	}

	goodOnes := 0

	for a := 0.01; a <= .05; a += .005 {

		outfn := fmt.Sprintf("out/%s-%v.png", root, int(1000*a))
		fmt.Println(outfn)

		out, err := SimpleEdgeDetection(img, a, 3.0)
		if err != nil {
			t.Fatal(err)
		}

		os.MkdirAll("out", 0775)
		err = WriteImageToFile(outfn, out)
		if err != nil {
			t.Fatal(err)
		}

		bad := false

		for x := 50; x <= 750; x += 100 {
			for y := 50; y <= 750; y += 100 {

				spots := CountBrightSpots(out, image.Point{x, y}, 25, 255)

				if y < 200 || y > 600 {
					if spots < 100 {
						bad = true
						fmt.Printf("\t%v,%v %v\n", x, y, spots)
					}
				} else {
					if spots > 90 {
						bad = true
						fmt.Printf("\t%v,%v %v\n", x, y, spots)
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

	if goodOnes == 0 {
		t.Errorf("no good ones found for root %s", root)
	}
}

func BenchmarkConvertImage(b *testing.B) {
	img, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		ConvertImage(img)
	}
}

func imageToYCbCr(dst *image.YCbCr, src image.Image) {
	if dst == nil {
		panic("dst can't be nil")
	}

	yuvImg, ok := src.(*image.YCbCr)
	if ok {
		*dst = *yuvImg
		return
	}

	bounds := src.Bounds()
	dy := bounds.Dy()
	dx := bounds.Dx()
	flat := dy * dx

	if len(dst.Y)+len(dst.Cb)+len(dst.Cr) < 3*flat {
		i0 := 1 * flat
		i1 := 2 * flat
		i2 := 3 * flat
		if cap(dst.Y) < i2 {
			dst.Y = make([]uint8, i2)
		}
		dst.Y = dst.Y[:i0]
		dst.Cb = dst.Y[i0:i1]
		dst.Cr = dst.Y[i1:i2]
	}
	dst.SubsampleRatio = image.YCbCrSubsampleRatio444
	dst.YStride = dx
	dst.CStride = dx
	dst.Rect = bounds

	i := 0
	for yi := 0; yi < dy; yi++ {
		for xi := 0; xi < dx; xi++ {
			// TODO(erh): probably try to get the alpha value with something like
			// https://en.wikipedia.org/wiki/Alpha_compositing
			r, g, b, _ := src.At(xi, yi).RGBA()
			yy, cb, cr := color.RGBToYCbCr(uint8(r/256), uint8(g/256), uint8(b/256))
			dst.Y[i] = yy
			dst.Cb[i] = cb
			dst.Cr[i] = cr
			i++
		}
	}
}

func TestConvertYCbCr(t *testing.T) {
	orig, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		t.Fatal(err)
	}

	var yuvImg image.YCbCr
	imageToYCbCr(&yuvImg, orig)

	err = WriteImageToFile("out/canny1-ycbcr.png", &yuvImg)
	if err != nil {
		t.Fatal(err)
	}

	c1, b1 := colorful.MakeColor(orig.At(100, 100))
	c2, b2 := colorful.MakeColor(yuvImg.At(100, 100))
	if !b1 || !b2 {
		t.Fatalf("can't convert")
	}

	assert.Equal(t, c1.Hex(), c2.Hex())
}

func BenchmarkConvertImageYCbCr(b *testing.B) {
	orig, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		b.Fatal(err)
	}

	var yuvImg image.YCbCr
	imageToYCbCr(&yuvImg, orig)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		ConvertImage(&yuvImg)
	}
}

func TestCanny(t *testing.T) {
	imgOriginal, err := ReadImageFromFile("data/canny_test_1.jpg")
	img := ConvertImage(imgOriginal)
	if err != nil {
		t.Fatal(err)
	}

	gtOriginal, err := ReadImageFromFile("data/test_canny.png")
	gt := ConvertImage(gtOriginal)
	if err != nil {
		t.Fatal(err)
	}

	cannyDetector := NewCannyDericheEdgeDetector()
	edgesMat, _ := cannyDetector.DetectEdges(img, 0.5)
	edges := ConvertImage(edgesMat)
	assert.Equal(t, len(edges.data), len(gt.data))
	assert.Equal(t, edges.data, gt.data)

}

func TestCannyBlocks(t *testing.T) {
	// load test image and GT
	imgOriginal, err := ReadImageFromFile("data/edge_test_image.png")
	img := ConvertImage(imgOriginal)
	if err != nil {
		t.Fatal(err)
	}
	gtGradient, err := ReadImageFromFile("data/edge_test_gradient.png")
	gtGrad := ConvertImage(gtGradient)
	if err != nil {
		t.Fatal(err)
	}
	gtNonMaxSup, err := ReadImageFromFile("data/edge_test_nms.png")
	gtNms := ConvertImage(gtNonMaxSup)
	if err != nil {
		t.Fatal(err)
	}
	// Compute forward gradient
	imgGradient, _ := ForwardGradient(img, 0., false)
	magData := imgGradient.Magnitude.RawMatrix().Data
	magDataInt := make([]Color, len(magData))
	for idx := 0; idx < len(magData); idx++ {
		magDataInt[idx] = NewColor(uint8(math.Round(magData[idx])), uint8(math.Round(magData[idx])), uint8(math.Round(magData[idx])))
	}
	magOut := Image{
		data:   magDataInt,
		width:  gtGrad.Width(),
		height: gtGrad.Height(),
	}

	// NMS
	nms, _ := GradientNonMaximumSuppressionC8(imgGradient.Magnitude, imgGradient.Direction)
	nmsData := nms.RawMatrix().Data
	nmsDataInt := make([]Color, len(nmsData))
	for idx := 0; idx < len(magData); idx++ {
		nmsDataInt[idx] = NewColor(uint8(math.Round(nmsData[idx])), uint8(math.Round(nmsData[idx])), uint8(math.Round(nmsData[idx])))
	}
	nmsOut := Image{
		data:   nmsDataInt,
		width:  gtGrad.Width(),
		height: gtGrad.Height(),
	}

	// run tests
	var tests = []struct {
		testName        string
		dataOut, dataGT []Color
	}{
		{"gradient", magOut.data, gtGrad.data},
		{"nms", nmsOut.data, gtNms.data},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			assert.Equal(t, len(tt.dataOut), len(tt.dataGT))
			assert.Equal(t, tt.dataOut, tt.dataOut)
		})
	}
}
