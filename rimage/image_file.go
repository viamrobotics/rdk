package rimage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/ppm"
	"github.com/pkg/errors"
	"github.com/xfmoulet/qoi"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	ut "go.viam.com/rdk/utils"
)

// RGBABitmapMagicNumber represents the magic number for our custom header
// for raw RGBA data. The header is composed of this magic number followed by
// a 4-byte line of the width as a uint32 number and another for the height. Credit to
// Ben Zotto for inventing this formulation
// https://bzotto.medium.com/introducing-the-rgba-bitmap-file-format-4a8a94329e2c
var RGBABitmapMagicNumber = []byte("RGBA")

// DepthMapMagicNumber represents the magic number for our custom header
// for raw DEPTH data.
var DepthMapMagicNumber = []byte("DEPTHMAP")

// RawRGBAHeaderLength is the length of our custom header for raw RGBA data
// in bytes. See above as to why.
const RawRGBAHeaderLength = 12

// RawDepthHeaderLength is the length of our custom header for raw depth map
// data in bytes. Header contains 8 bytes worth of magic number, followed by 8 bytes
// for width and another 8bytes for height .
const RawDepthHeaderLength = 24

func init() {
	// Here we register the custom format above so that we can simply use image.Decode
	// so long as the raw RGBA data has the appropriate header
	image.RegisterFormat("vnd.viam.rgba", string(RGBABitmapMagicNumber),
		func(r io.Reader) (image.Image, error) {
			rawBytes, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			if len(rawBytes) < RawRGBAHeaderLength {
				return nil, io.EOF
			}
			header := rawBytes[:RawRGBAHeaderLength]
			width := int(binary.BigEndian.Uint32(header[4:8]))
			height := int(binary.BigEndian.Uint32(header[8:12]))
			img := image.NewNRGBA(image.Rect(0, 0, width, height))
			imgBytes := rawBytes[RawRGBAHeaderLength:]
			img.Pix = imgBytes
			return img, nil
		},
		func(r io.Reader) (image.Config, error) {
			imgBytes := make([]byte, RawRGBAHeaderLength)
			_, err := io.ReadFull(r, imgBytes)
			if err != nil {
				return image.Config{}, err
			}
			header := imgBytes[:RawRGBAHeaderLength]
			width := binary.BigEndian.Uint32(header[4:8])
			height := binary.BigEndian.Uint32(header[8:12])
			return image.Config{
				ColorModel: color.RGBAModel,
				Width:      int(width),
				Height:     int(height),
			}, nil
		},
	)

	// Here we register our format for depth images so that we can use
	// image.Decode as long as we have the appropriate header
	image.RegisterFormat("vnd.viam.dep", string(DepthMapMagicNumber),
		func(r io.Reader) (image.Image, error) {
			f := r.(*bufio.Reader)
			dm, err := ReadDepthMap(f)
			if err != nil {
				return nil, err
			}
			return dm, nil
		},
		func(r io.Reader) (image.Config, error) {
			// Using Gray 16 as underlying color model for depth
			f := r.(*bufio.Reader)
			firstBytes, err := _readNext(r)
			if err != nil || firstBytes != MagicNumIntViamType {
				return image.Config{}, errors.Wrap(err, "first image bytes do not match expected magic number")
			}
			rawWidth, err := _readNext(f)
			if err != nil {
				return image.Config{}, err
			}
			rawHeight, err := _readNext(f)
			if err != nil {
				return image.Config{}, err
			}
			return image.Config{
				ColorModel: color.Gray16Model,
				Width:      int(rawWidth),
				Height:     int(rawHeight),
			}, nil
		},
	)
} // end of init

// readImageFromFile extracts the RGB, Z16, or raw depth data from an image file.
func readImageFromFile(path string) (image.Image, error) {
	switch {
	case strings.HasSuffix(path, ".dat.gz"), strings.HasSuffix(path, ".dat"):
		return ParseRawDepthMap(path)
	default:
		//nolint:gosec
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer utils.UncheckedErrorFunc(f.Close)

		img, _, err := image.Decode(f)
		if err != nil {
			return nil, err
		}
		return img, nil
	}
}

// NewImageFromFile returns an image read in from the given file.
func NewImageFromFile(fn string) (*Image, error) {
	img, err := readImageFromFile(fn)
	if err != nil {
		return nil, err
	}
	return ConvertImage(img), nil
}

// NewDepthMapFromFile extract the depth map from a Z16 image file or a .dat image file.
func NewDepthMapFromFile(ctx context.Context, fn string) (*DepthMap, error) {
	img, err := readImageFromFile(fn)
	if err != nil {
		return nil, err
	}
	return ConvertImageToDepthMap(ctx, img)
}

// WriteImageToFile writes the given image to a file at the supplied path.
func WriteImageToFile(path string, img image.Image) (err error) {
	//nolint:gosec
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, f.Close())
	}()
	if dm, ok := img.(*DepthMap); ok {
		img = dm.ToGray16Picture()
	}

	switch filepath.Ext(path) {
	case ".png":
		return png.Encode(f, img)
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, nil)
	case ".ppm":
		return ppm.Encode(f, img)
	case ".qoi":
		return qoi.Encode(f, img)
	default:
		return errors.Errorf("rimage.WriteImageToFile unsupported format: %s", filepath.Ext(path))
	}
}

// ConvertImage converts a go image into our Image type.
func ConvertImage(img image.Image) *Image {
	ii, ok := img.(*Image)
	if ok {
		return ii
	}

	iwd, ok := img.(*imageWithDepth)
	if ok {
		return iwd.Color
	}

	b := img.Bounds()
	ii = NewImage(b.Max.X, b.Max.Y)

	switch orig := img.(type) {
	case *image.YCbCr:
		fastConvertYcbcr(ii, orig)
	case *image.RGBA:
		fastConvertRGBA(ii, orig)
	case *image.NRGBA:
		fastConvertNRGBA(ii, orig)
	default:
		for y := 0; y < ii.height; y++ {
			for x := 0; x < ii.width; x++ {
				ii.SetXY(x, y, NewColorFromColor(img.At(x, y)))
			}
		}
	}
	return ii
}

// CloneImage creates a copy of the input image.
func CloneImage(img image.Image) *Image {
	ii, ok := img.(*Image)
	if ok {
		return ii.Clone()
	}
	iwd, ok := img.(*imageWithDepth)
	if ok {
		return iwd.Clone().Color
	}

	return ConvertImage(img)
}

// SaveImage takes an image.Image and saves it to a jpeg at the given
// file location and also returns the location back.
func SaveImage(pic image.Image, loc string) error {
	//nolint:gosec
	f, err := os.Create(loc)
	if err != nil {
		return errors.Wrapf(err, "can't save at location %s", loc)
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	// Specify the quality, between 0-100
	opt := jpeg.Options{Quality: 90}
	err = jpeg.Encode(f, pic, &opt)
	if err != nil {
		return errors.Wrapf(err, "the 'image' will not encode")
	}
	return nil
}

// DecodeImage takes an image buffer and decodes it, using the mimeType
// and the dimensions, to return the image.
func DecodeImage(ctx context.Context, imgBytes []byte, mimeType string) (image.Image, error) {
	_, span := trace.StartSpan(ctx, "rimage::DecodeImage::"+mimeType)
	defer span.End()
	mimeType, returnLazy := ut.CheckLazyMIMEType(mimeType)
	if returnLazy {
		return NewLazyEncodedImage(imgBytes, mimeType), nil
	}
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// EncodeImage takes an image and mimeType as input and encodes it into a
// slice of bytes (buffer) and returns the bytes.
func EncodeImage(ctx context.Context, img image.Image, mimeType string) ([]byte, error) {
	_, span := trace.StartSpan(ctx, "rimage::EncodeImage::"+mimeType)
	defer span.End()

	actualOutMIME, _ := ut.CheckLazyMIMEType(mimeType)

	if lazy, ok := img.(*LazyEncodedImage); ok && lazy.MIMEType() == actualOutMIME {
		return lazy.imgBytes, nil
	}

	var buf bytes.Buffer
	bounds := img.Bounds()
	switch actualOutMIME {
	case ut.MimeTypeRawDepth:
		buf.Write(DepthMapMagicNumber)
		// WriteRawDepthMapTo encodes the height and width
		if _, err := WriteRawDepthMapTo(img.(*DepthMap), &buf); err != nil {
			return nil, err
		}
	case ut.MimeTypeRawRGBA:
		// Here we create a custom header to prepend to Raw RGBA data. Credit to
		// Ben Zotto for inventing this formulation
		// https://bzotto.medium.com/introducing-the-rgba-bitmap-file-format-4a8a94329e2c
		buf.Write(RGBABitmapMagicNumber)
		widthBytes := make([]byte, 4)
		heightBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(widthBytes, uint32(bounds.Dx()))
		binary.BigEndian.PutUint32(heightBytes, uint32(bounds.Dy()))
		buf.Write(widthBytes)
		buf.Write(heightBytes)
		imgStruct := image.NewNRGBA(bounds)
		draw.Draw(imgStruct, bounds, img, bounds.Min, draw.Src)
		buf.Write(imgStruct.Pix)
	case ut.MimeTypePNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	case ut.MimeTypeJPEG:
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case ut.MimeTypeQOI:
		if err := qoi.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("do not know how to encode %q", actualOutMIME)
	}

	return buf.Bytes(), nil
}

func fastConvertNRGBA(dst *Image, src *image.NRGBA) {
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			i := src.PixOffset(x, y)
			s := src.Pix[i : i+3 : i+3] // Small cap improves performance, see https://golang.org/issue/27857
			r, g, b := s[0], s[1], s[2]
			dst.SetXY(x, y, NewColor(r, g, b))
		}
	}
}

func fastConvertRGBA(dst *Image, src *image.RGBA) {
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			i := src.PixOffset(x, y)
			s := src.Pix[i : i+4 : i+4]
			r, g, b, a := s[0], s[1], s[2], s[3]

			if a == 255 {
				dst.SetXY(x, y, NewColor(r, g, b))
			} else {
				dst.SetXY(x, y, NewColorFromColor(color.RGBA{r, g, b, a}))
			}
		}
	}
}

func fastConvertYcbcr(dst *Image, src *image.YCbCr) {
	c := color.YCbCr{}

	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			yi := src.YOffset(x, y)
			ci := src.COffset(x, y)

			c.Y = src.Y[yi]
			c.Cb = src.Cb[ci]
			c.Cr = src.Cr[ci]

			r, g, b := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)

			dst.SetXY(x, y, NewColor(r, g, b))
		}
	}
}

// IsImageFile returns if the given file is an image file based on what
// we support.
func IsImageFile(fn string) bool {
	extensions := []string{"ppm", "png", "jpg", "jpeg", "gif"}
	for _, suffix := range extensions {
		if strings.HasSuffix(fn, suffix) {
			return true
		}
	}
	return false
}
