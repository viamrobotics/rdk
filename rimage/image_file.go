package rimage

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/ppm"
)

func ReadImageFromFile(path string) (image.Image, error) {
	if strings.HasSuffix(path, ".both.gz") {
		return BothReadFromFile(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func NewImageFromFile(fn string) (*Image, error) {
	img, err := ReadImageFromFile(fn)
	if err != nil {
		return nil, err
	}

	return ConvertImage(img), nil
}

func WriteImageToFile(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	switch filepath.Ext(path) {
	case ".png":
		return png.Encode(f, img)
	case ".ppm":
		return ppm.Encode(f, img)
	default:
		return fmt.Errorf("rimage.WriteImageToFile unsupported format: %s", filepath.Ext(path))
	}

}

func ConvertImage(img image.Image) *Image {
	ii, ok := img.(*Image)
	if ok {
		return ii
	}

	iwd, ok := img.(*ImageWithDepth)
	if ok {
		return iwd.Color
	}

	b := img.Bounds()
	ii = NewImage(b.Max.X, b.Max.Y)
	for x := 0; x < ii.width; x++ {
		for y := 0; y < ii.height; y++ {
			ii.SetXY(x, y, NewColorFromColor(img.At(x, y)))
		}
	}
	return ii
}

func IsImageFile(fn string) bool {
	extensions := []string{".both.gz", "ppm", "png", "jpg", "jpeg", "gif"}
	for _, suffix := range extensions {
		if strings.HasSuffix(fn, suffix) {
			return true
		}
	}
	return false
}
