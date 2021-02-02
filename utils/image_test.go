package utils

import (
	"image"
	"os"
	"testing"
)

func TestCanny1(t *testing.T) {
	img, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Canny(img, 8, 8, 1)
	if err != nil {
		t.Fatal(err)
	}

	spotsPiece := CountBrightSpots(out, image.Point{50, 50}, 25, 255)
	if spotsPiece < 100 {
		t.Errorf("spotsPiece too low %d", spotsPiece)
	}

	spotsEmpty := CountBrightSpots(out, image.Point{50, 250}, 25, 255)
	if spotsEmpty > 100 {
		t.Errorf("spotsEmpty too high %d", spotsEmpty)
	}

	spotsPiece2 := CountBrightSpots(out, image.Point{50, 750}, 25, 255)
	if spotsPiece2 < 100 {
		t.Errorf("spotsPieces too low %d", spotsPiece2)
	}

	os.MkdirAll("out", 0775)
	err = WriteImageToFile("out/canny1.png", out)
	if err != nil {
		t.Fatal(err)
	}

}
