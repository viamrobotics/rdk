package utils

import (
	"image"
	"image/png"
	"os"
)

func WriteImageToFile(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
