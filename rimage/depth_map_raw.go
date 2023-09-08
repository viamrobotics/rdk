//go:build !notc

package rimage

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// MagicNumIntVersionX is the magic number (as an int) for VERSIONX.
const MagicNumIntVersionX = 6363110499870197078

// MagicNumIntViamType is the magic number (as an int) for the custom Viam depth type.
// magic number for ViamCustomType is uint64([]byte("DEPTHMAP")), Big Endian.
const MagicNumIntViamType = 4919426490892632400

// MagicNumIntViamTypeLittleEndian is "PAMHTPED" for the ReadDepthMap function which uses LittleEndian to read.
const MagicNumIntViamTypeLittleEndian = 5782988369567958340

func _readNext(r io.Reader) (int64, error) {
	data := make([]byte, 8)
	x, err := r.Read(data)
	if x == 8 {
		return int64(binary.LittleEndian.Uint64(data)), nil
	}

	return 0, errors.Wrapf(err, "got %d bytes", x)
}

// ParseRawDepthMap parses a depth map from the given file. It knows
// how to handle compressed files as well.
func ParseRawDepthMap(fn string) (*DepthMap, error) {
	var f io.Reader

	//nolint:gosec
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}

	if filepath.Ext(fn) == ".gz" {
		f, err = gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
	}

	return ReadDepthMap(bufio.NewReader(f))
}

// ReadDepthMap returns a depth map from the given reader.
func ReadDepthMap(r io.Reader) (*DepthMap, error) {
	// We expect the first 8 bytes to be a magic number assigned to a depthmap type
	firstBytes, err := _readNext(r)
	if err != nil {
		return nil, err
	}
	switch firstBytes {
	case MagicNumIntVersionX: // magic number for VERSIONX
		return readDepthMapVersionX(r)
	case MagicNumIntViamTypeLittleEndian: // magic number for ViamCustomType PAMHTPED (LittleEndian DEPTHMAP)
		return readDepthMapViam(r)
	default:
		return readDepthMapRaw(r, firstBytes)
	}
}

func readDepthMapRaw(ff io.Reader, firstBytes int64) (*DepthMap, error) {
	f := bufio.NewReader(ff)
	dm := DepthMap{}

	dm.width = int(firstBytes)

	rawHeight, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.height = int(rawHeight)

	return setRawDepthMapValues(f, &dm)
}

func readDepthMapViam(ff io.Reader) (*DepthMap, error) {
	f := bufio.NewReader(ff)
	dm := &DepthMap{}
	data := make([]byte, 8)

	_, err := f.Read(data)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read vnd.viam.dep width")
	}
	rawWidth := binary.BigEndian.Uint64(data)
	dm.width = int(rawWidth)

	_, err = f.Read(data)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read vnd.viam.dep height")
	}
	rawHeight := binary.BigEndian.Uint64(data)
	dm.height = int(rawHeight)

	// dump the rest of the bytes in a depth slice
	datSlice := make([]Depth, dm.height*dm.width)
	err = binary.Read(f, binary.BigEndian, &datSlice)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read vnd.viam.dep data slice")
	}
	dm.data = datSlice
	return dm, nil
}

func readDepthMapVersionX(rr io.Reader) (*DepthMap, error) {
	dm := DepthMap{}

	r := bufio.NewReader(rr)
	// get past garbade
	_, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}

	bytesPerPixelString, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	bytesPerPixelString = strings.TrimSpace(bytesPerPixelString)

	if bytesPerPixelString != "2" {
		return nil, errors.Errorf("i only know how to handle 2 bytes per pixel in new format, not %s", bytesPerPixelString)
	}

	unitsString, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	unitsString = strings.TrimSpace(unitsString)
	units, err := strconv.ParseFloat(unitsString, 64)
	if err != nil {
		return nil, err
	}
	units *= 1000 // meters to millis

	widthString, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	widthString = strings.TrimSpace(widthString)
	x, err := strconv.ParseInt(widthString, 10, 64)
	dm.width = int(x)
	if err != nil {
		return nil, err
	}

	heightString, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	heightString = strings.TrimSpace(heightString)
	x, err = strconv.ParseInt(heightString, 10, 64)
	dm.height = int(x)
	if err != nil {
		return nil, err
	}

	if dm.width <= 0 || dm.width >= 100000 || dm.height <= 0 || dm.height >= 100000 {
		return nil, errors.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
	}

	temp := make([]byte, 2)
	dm.data = make([]Depth, dm.width*dm.height)

	for y := 0; y < dm.height; y++ {
		for x := 0; x < dm.width; x++ {
			n, err := r.Read(temp)
			if n == 1 {
				b2, err2 := r.ReadByte()
				if err2 != nil {
					err = err2
				} else {
					n++
				}
				temp[1] = b2
			}

			if n != 2 || err != nil {
				return nil, errors.Wrapf(err, "didn't read 2 bytes, got: %d x,y: %d,%x", n, x, y)
			}

			dm.Set(x, y, Depth(units*float64(binary.LittleEndian.Uint16(temp))))
		}
	}

	return &dm, nil
}

// setRawDepthMapValues read out values 8 bytes at a time, converting the 8 bytes into a 2 byte depth value.
func setRawDepthMapValues(f *bufio.Reader, dm *DepthMap) (*DepthMap, error) {
	if dm.width <= 0 || dm.width >= 100000 || dm.height <= 0 || dm.height >= 100000 {
		return nil, errors.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
	}

	dm.data = make([]Depth, dm.width*dm.height)

	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			temp, err := _readNext(f)
			if err != nil {
				return nil, err
			}
			dm.Set(x, y, Depth(temp))
		}
	}

	return dm, nil
}

// WriteRawDepthMapToFile writes the raw depth map to the given file.
func WriteRawDepthMapToFile(dm image.Image, fn string) (err error) {
	//nolint:gosec
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, f.Close())
	}()

	var gout *gzip.Writer
	var out io.Writer = f

	if filepath.Ext(fn) == ".gz" {
		gout = gzip.NewWriter(f)
		out = gout
		defer func() {
			err = multierr.Combine(err, gout.Close())
		}()
	}

	if strings.HasSuffix(fn, ".vnd.viam.dep") {
		_, err = WriteViamDepthMapTo(dm, out)
		if err != nil {
			return err
		}
	} else {
		_, err = WriteRawDepthMapTo(dm, out)
		if err != nil {
			return err
		}
	}

	if gout != nil {
		if err := gout.Flush(); err != nil {
			return err
		}
	}

	return f.Sync()
}

// WriteRawDepthMapTo writes this depth map to the given writer.
// the raw depth map type writes 8 bytes of width, 8 bytes of height, and 8 bytes per pixel.
func WriteRawDepthMapTo(img image.Image, out io.Writer) (int64, error) {
	buf := make([]byte, 8)
	var totalN int64
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	binary.LittleEndian.PutUint64(buf, uint64(width))
	n, err := out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}

	binary.LittleEndian.PutUint64(buf, uint64(height))
	n, err = out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			switch dm := img.(type) {
			case *DepthMap:
				binary.LittleEndian.PutUint64(buf, uint64(dm.GetDepth(x, y)))
			case *image.Gray16:
				binary.LittleEndian.PutUint64(buf, uint64(dm.Gray16At(x, y).Y))
			default:
				return totalN, errors.Errorf("cannot convert image type %T to a raw depth format", dm)
			}
			n, err = out.Write(buf)
			totalN += int64(n)
			if err != nil {
				return totalN, err
			}
		}
	}

	return totalN, nil
}

// WriteViamDepthMapTo writes depth map or gray16 image to the given writer as vnd.viam.dep bytes.
// the Viam custom depth type writes 8 bytes of "magic number", 8 bytes of width, 8 bytes of height, and 2 bytes per pixel.
func WriteViamDepthMapTo(img image.Image, out io.Writer) (int64, error) {
	if lazy, ok := img.(*LazyEncodedImage); ok {
		lazy.decode()
		if lazy.decodeErr != nil {
			return 0, errors.Errorf("could not decode LazyEncodedImage to a depth image: %v", lazy.decodeErr)
		}
		img = lazy.decodedImage
	}
	buf := make([]byte, 8)
	var totalN int64
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	binary.BigEndian.PutUint64(buf, uint64(MagicNumIntViamType))
	n, err := out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}
	binary.BigEndian.PutUint64(buf, uint64(width))
	n, err = out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}

	binary.BigEndian.PutUint64(buf, uint64(height))
	n, err = out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}
	switch dm := img.(type) {
	case *DepthMap:
		err = binary.Write(out, binary.BigEndian, dm.data)
		if err != nil {
			return totalN, err
		}
		totalN += int64(len(dm.data) * 2) // uint16 data
	case *image.Gray16:
		grayBuf := make([]byte, 2)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				i := dm.PixOffset(x, y)
				z := uint16(dm.Pix[i+0])<<8 | uint16(dm.Pix[i+1])
				binary.BigEndian.PutUint16(grayBuf, z)
				n, err = out.Write(grayBuf)
				totalN += int64(n)
				if err != nil {
					return totalN, err
				}
			}
		}
	default:
		return totalN, errors.Errorf("cannot convert image type %T to image/vnd.viam.dep depth format", dm)
	}
	return totalN, nil
}
