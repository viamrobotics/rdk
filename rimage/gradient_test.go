package rimage

import (
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSobelGradient(t *testing.T) {
	depthImg, err := ReadImageFromFile("data/circle.png")
	if err != nil {
		t.Fatal(err)
	}
	dm, err := ConvertImageToDepthMap(depthImg)
	if err != nil {
		t.Fatal(err)
	}

	gradients := SobelFilter(dm)
	assert.Equal(t, dm.Height()-2, gradients.Height())
	assert.Equal(t, dm.Width()-2, gradients.Width())
	// circle is 150 pixels in diameter, centered at (150,100)
	assert.Equal(t, 0., gradients.GetGradient(223, 100).Direction())           //(223,100) should have direction 0 degrees
	assert.Equal(t, math.Pi/2., gradients.GetGradient(149, 173).Direction())   //(149,173) should have direction 90 degrees
	assert.Equal(t, math.Pi, gradients.GetGradient(76, 100).Direction())       //(149,173) should have direction 180 degrees
	assert.Equal(t, 3.*math.Pi/2., gradients.GetGradient(149, 26).Direction()) //(149,173) should have direction 270 degrees

	img := gradients.ToPrettyPicture()

	os.MkdirAll("out", 0775)
	file, err := os.Create("out/circle_gradient.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	png.Encode(file, img)
}
