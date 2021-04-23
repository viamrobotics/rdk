package rimage

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func BothReadFromBytes(allData []byte, isAligned bool) (*ImageWithDepth, error) {
	reader := bufio.NewReader(bytes.NewReader(allData))
	depth, err := ReadDepthMap(reader)
	if err != nil {
		return nil, fmt.Errorf("couldn't read depth map (both): %w", err)
	}

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	return &ImageWithDepth{ConvertImage(img), depth, isAligned, nil}, nil
}

func BothReadFromFile(fn string, isAligned bool) (*ImageWithDepth, error) {
	if !strings.HasSuffix(fn, ".both.gz") {
		return nil, fmt.Errorf("bad extension")
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	in, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	defer in.Close()

	allData, err := ioutil.ReadAll(in)

	if err != nil {
		return nil, err
	}

	return BothReadFromBytes(allData, isAligned)
}

func BothWriteToFile(i *ImageWithDepth, fn string) error {
	if !strings.HasSuffix(fn, ".both.gz") {
		return fmt.Errorf("vision.ImageWithDepth WriteTo only supports both.gz")
	}

	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	out := gzip.NewWriter(f)
	defer out.Close()

	err = BothEncode(i, out)
	if err != nil {
		return err
	}

	out.Flush()
	return f.Sync()
}

func BothEncode(i *ImageWithDepth, out io.Writer) error {
	err := i.Depth.WriteTo(out)
	if err != nil {
		return err
	}

	err = png.Encode(out, i.Color)
	if err != nil {
		return err
	}

	return nil
}
