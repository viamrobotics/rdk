package vision

import (
	"image/png"
	"os"
)

func WriteImageToFile(path string, img Image) error {
	goImg, err := img.ToImage()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, goImg)
}
