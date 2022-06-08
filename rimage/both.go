package rimage

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"image"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
)

// ReadBothFromReader reads the given data as an image that contains depth. isAligned
// notifies the reader if the image and depth is already aligned.
func ReadBothFromReader(reader *bufio.Reader, isAligned bool) (*ImageWithDepth, error) {
	depth, err := ReadDepthMap(reader)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't read depth map (both)")
	}

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	return &ImageWithDepth{ConvertImage(img), depth, isAligned}, nil
}

// ReadBothFromBytes reads the given data as an image that contains depth. isAligned
// notifies the reader if the image and depth is already aligned.
func ReadBothFromBytes(allData []byte, isAligned bool) (*ImageWithDepth, error) {
	reader := bufio.NewReader(bytes.NewReader(allData))
	return ReadBothFromReader(reader, isAligned)
}

// ReadBothFromFile reads the given file as an image that contains depth. isAligned
// notifies the reader if the image and depth is already aligned.
func ReadBothFromFile(fn string, isAligned bool) (*ImageWithDepth, error) {
	if !strings.HasSuffix(fn, ".both.gz") {
		return nil, errors.New("bad extension")
	}

	//nolint:gosec
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	in, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	defer utils.UncheckedErrorFunc(in.Close)

	return ReadBothFromReader(bufio.NewReader(in), isAligned)
}

// WriteBothToFile writes the image with depth to the given file.
func WriteBothToFile(i *ImageWithDepth, fn string) (err error) {
	if !strings.HasSuffix(fn, ".both.gz") {
		return errors.New("vision.ImageWithDepth WriteTo only supports both.gz")
	}

	//nolint:gosec
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, f.Close())
	}()

	out := gzip.NewWriter(f)
	defer func() {
		err = multierr.Combine(err, out.Close())
	}()

	err = EncodeBoth(i, out)
	if err != nil {
		return err
	}

	if err := out.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// EncodeBoth writes the image with depth to the given writer.
func EncodeBoth(i *ImageWithDepth, out io.Writer) error {
	_, err := i.Depth.WriteTo(out)
	if err != nil {
		return err
	}

	err = png.Encode(out, i.Color)
	if err != nil {
		return err
	}

	return nil
}
