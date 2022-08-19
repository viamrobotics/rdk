package rimage

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

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

	return ReadRawDepthMap(bufio.NewReader(f))
}

// ReadRawDepthMap returns a depth map from the given reader.
func ReadRawDepthMap(f *bufio.Reader) (*DepthMap, error) {
	var err error
	dm := DepthMap{}

	rawWidth, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.width = int(rawWidth)

	if rawWidth == 6363110499870197078 { // magic number for VERSIONX
		return readDepthMapFormat2(f)
	}

	rawHeight, err := _readNext(f)
	if err != nil {
		return nil, err
	}
	dm.height = int(rawHeight)

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

	return &dm, nil
}

func readDepthMapFormat2(r *bufio.Reader) (*DepthMap, error) {
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

// WriteRawDepthMapToFile writes the raw depth map to the given file.
func WriteRawDepthMapToFile(dm *DepthMap, fn string) (err error) {
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

// WriteRawDepthMapTo writes this depth map to the given writer.
func WriteRawDepthMapTo(dm *DepthMap, out io.Writer) (int64, error) {
	buf := make([]byte, 8)

	var totalN int64
	binary.LittleEndian.PutUint64(buf, uint64(dm.width))
	n, err := out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}

	binary.LittleEndian.PutUint64(buf, uint64(dm.height))
	n, err = out.Write(buf)
	totalN += int64(n)
	if err != nil {
		return totalN, err
	}

	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			binary.LittleEndian.PutUint64(buf, uint64(dm.GetDepth(x, y)))
			n, err = out.Write(buf)
			totalN += int64(n)
			if err != nil {
				return totalN, err
			}
		}
	}

	return totalN, nil
}
