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
// magic number for ViamCustomType is int64([]byte("DEPTHMAP")).
const MagicNumIntViamType = 5782988369567958340

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
		return readDepthMapVersionX(r.(*bufio.Reader))
	case MagicNumIntViamType: // magic number for ViamCustomType is int64([]byte("DEPTHMAP"))
		return readDepthMapViam(r.(*bufio.Reader))
	default:
		return readDepthMapRaw(r.(*bufio.Reader), firstBytes)
	}
}

func readDepthMapRaw(f *bufio.Reader, firstBytes int64) (*DepthMap, error) {
	dm := DepthMap{}

	dm.width = int(firstBytes)

	rawHeight, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.height = int(rawHeight)

	return setRawDepthMapValues(f, &dm)
}

func readDepthMapViam(f *bufio.Reader) (*DepthMap, error) {
	dm := DepthMap{}

	rawWidth, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.width = int(rawWidth)
	rawHeight, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.height = int(rawHeight)

	return setRawDepthMapValues(f, &dm)
}

func readDepthMapVersionX(r *bufio.Reader) (*DepthMap, error) {
	dm := DepthMap{}

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
		_, err := out.Write(DepthMapMagicNumber)
		if err != nil {
			return err
		}
	}

	_, err = WriteRawDepthMapTo(dm, out)
	if err != nil {
		return err
	}

	if gout != nil {
		if err := gout.Flush(); err != nil {
			return err
		}
	}

	return f.Sync()
}

// WriteRawDepthMapTo writes depth map or gray16 image to the given writer.
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
