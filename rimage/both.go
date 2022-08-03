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

// ReadBothFromReader reads the given data as an image that contains depth.
func ReadBothFromReader(reader *bufio.Reader) (*Image, *DepthMap, error) {
	depth, err := ReadDepthMap(reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't read depth map (both)")
	}

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, nil, err
	}

	return ConvertImage(img), depth, nil
}

// ReadBothFromBytes reads the given data as an image that contains depth.
func ReadBothFromBytes(allData []byte) (*Image, *DepthMap, error) {
	reader := bufio.NewReader(bytes.NewReader(allData))
	return ReadBothFromReader(reader)
}

// ReadBothFromFile reads the given file as an image that contains depth.
func ReadBothFromFile(fn string) (*Image, *DepthMap, error) {
	if !strings.HasSuffix(fn, ".both.gz") {
		return nil, nil, errors.New("bad extension")
	}

	//nolint:gosec
	f, err := os.Open(fn)
	if err != nil {
		return nil, nil, err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	in, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}

	defer utils.UncheckedErrorFunc(in.Close)

	return ReadBothFromReader(bufio.NewReader(in))
}

// WriteBothToFile writes the image with depth to the given file.
func WriteBothToFile(i *imageWithDepth, fn string) (err error) {
	if !strings.HasSuffix(fn, ".both.gz") {
		return errors.New("imageWithDepth WriteTo only supports both.gz")
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
func EncodeBoth(i *imageWithDepth, out io.Writer) error {
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
