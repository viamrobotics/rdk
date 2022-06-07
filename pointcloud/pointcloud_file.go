package pointcloud

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/edaniels/golog"
	"github.com/edaniels/lidario"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
)

// PCDType is the format of a pcd file.
type PCDType int

const (
	// PCDAscii ascii format for pcd.
	PCDAscii PCDType = 0
	// PCDBinary binary format for pcd.
	PCDBinary PCDType = 1
)

// NewFromFile returns a pointcloud read in from the given file.
func NewFromFile(fn string, logger golog.Logger) (PointCloud, error) {
	switch filepath.Ext(fn) {
	case ".las":
		return NewFromLASFile(fn, logger)
	default:
		return nil, errors.Errorf("do not know how to read file %q", fn)
	}
}

// pointValueDataTag encodes if the point has value data.
const pointValueDataTag = "rc|pv"

// NewFromLASFile returns a point cloud from reading a LAS file. If any
// lossiness of points could occur from reading it in, it's reported but is not
// an error.
func NewFromLASFile(fn string, logger golog.Logger) (PointCloud, error) {
	lf, err := lidario.NewLasFile(fn, "r")
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(lf.Close)

	var hasValue bool
	var valueData []byte
	for _, d := range lf.VlrData {
		if d.Description == pointValueDataTag {
			hasValue = true
			valueData = d.BinaryData
			break
		}
	}

	pc := New()
	for i := 0; i < lf.Header.NumberPoints; i++ {
		p, err := lf.LasPoint(i)
		if err != nil {
			return nil, err
		}
		data := p.PointData()

		x, y, z := data.X, data.Y, data.Z
		if x < minPreciseFloat64 || x > maxPreciseFloat64 ||
			y < minPreciseFloat64 || y > maxPreciseFloat64 ||
			z < minPreciseFloat64 || z > maxPreciseFloat64 {
			logger.Warnf("potential floating point lossiness for LAS point",
				"point", data, "range", fmt.Sprintf("[%f,%f]", minPreciseFloat64, maxPreciseFloat64))
		}

		v := r3.Vector{x, y, z}
		var dd Data
		if lf.Header.PointFormatID == 2 && p.RgbData() != nil {
			r := uint8(p.RgbData().Red / 256)
			g := uint8(p.RgbData().Green / 256)
			b := uint8(p.RgbData().Blue / 256)
			dd = NewColoredData(color.NRGBA{r, g, b, 255})
		}

		if hasValue {
			value := int(binary.LittleEndian.Uint64(valueData[i*8 : (i*8)+8]))
			if dd == nil {
				dd = NewBasicData()
			}
			dd.SetValue(value)
		}

		if err := pc.Set(v, dd); err != nil {
			return nil, err
		}
	}
	return pc, nil
}

// WriteToLASFile writes the point cloud out to a LAS file.
func WriteToLASFile(cloud PointCloud, fn string) (err error) {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return
	}
	defer func() {
		cerr := lf.Close()
		err = multierr.Combine(err, cerr)
	}()

	meta := cloud.MetaData()

	pointFormatID := 0
	if meta.HasColor {
		pointFormatID = 2
	}
	if err = lf.AddHeader(lidario.LasHeader{
		PointFormatID: byte(pointFormatID),
	}); err != nil {
		return
	}

	var pVals []int
	if meta.HasValue {
		pVals = make([]int, 0, cloud.Size())
	}
	var lastErr error
	cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
		var lp lidario.LasPointer
		pr0 := &lidario.PointRecord0{
			// floating point lossiness validated/warned from set/load
			X: pos.X,
			Y: pos.Y,
			Z: pos.Z,
			BitField: lidario.PointBitField{
				Value: (1) | (1 << 3) | (0 << 6) | (0 << 7),
			},
			ClassBitField: lidario.ClassificationBitField{
				Value: 0,
			},
			ScanAngle:     0,
			UserData:      0,
			PointSourceID: 1,
		}
		lp = pr0

		if d != nil {
			pr0.Intensity = d.Intensity()
		}

		if meta.HasColor {
			red, green, blue := 255, 255, 255
			if d != nil && d.HasColor() {
				r, g, b := d.RGB255()
				red, green, blue = int(r), int(g), int(b)
			}
			lp = &lidario.PointRecord2{
				PointRecord0: pr0,
				RGB: &lidario.RgbData{
					Red:   uint16(red * 256),
					Green: uint16(green * 256),
					Blue:  uint16(blue * 256),
				},
			}
		}
		if meta.HasValue {
			if d != nil && d.HasValue() {
				pVals = append(pVals, d.Value())
			} else {
				pVals = append(pVals, 0)
			}
		}
		if lerr := lf.AddLasPoint(lp); lerr != nil {
			lastErr = lerr
			return false
		}
		return true
	})
	if meta.HasValue {
		var buf bytes.Buffer
		for _, v := range pVals {
			bytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(bytes, uint64(v))
			buf.Write(bytes)
		}
		if err = lf.AddVLR(lidario.VLR{
			UserID:                  "",
			Description:             pointValueDataTag,
			BinaryData:              buf.Bytes(),
			RecordLengthAfterHeader: buf.Len(),
		}); err != nil {
			return
		}
	}
	if lastErr != nil {
		err = lastErr
		return
	}

	// nolint:nakedret
	return
}

func _colorToPCDInt(pt Data) int {
	if pt == nil || !pt.HasColor() {
		return 255 << 16 // TODO(erh): this doesn't feel great
	}

	r, g, b := pt.RGB255()
	x := 0

	x |= (int(r) << 16)
	x |= (int(g) << 8)
	x |= (int(b) << 0)
	return x
}

func _pcdIntToColor(c int) color.NRGBA {
	r := uint8(0xFF & (c >> 16))
	g := uint8(0xFF & (c >> 8))
	b := uint8(0xFF & (c >> 0))
	return color.NRGBA{r, g, b, 255}
}

// ToPCD Writes a PointCloud in PCD format.
func ToPCD(cloud PointCloud, out io.Writer, outputType PCDType) error {
	var err error

	_, err = fmt.Fprintf(out, "VERSION .7\n"+
		"FIELDS x y z rgb\n"+
		"SIZE 4 4 4 4\n"+
		"TYPE F F F I\n"+
		"COUNT 1 1 1 1\n"+
		"WIDTH %d\n"+
		"HEIGHT %d\n"+
		"VIEWPOINT 0 0 0 1 0 0 0\n"+
		"POINTS %d\n",
		cloud.Size(),
		1,
		cloud.Size(),
	)

	if err != nil {
		return err
	}

	switch outputType {
	case PCDAscii:
		_, err = out.Write([]byte("DATA ascii\n"))
		if err != nil {
			return err
		}

		cloud.Iterate(0, 0, func(position r3.Vector, d Data) bool {
			// Our point clouds are in mm, PCD files expect meters
			width := position.X / 1000.
			height := position.Y / 1000.
			depth := position.Z / 1000.

			_, err = fmt.Fprintf(out, "%f %f %f %d\n",
				width,
				height,
				depth,
				_colorToPCDInt(d))
			return err == nil
		})
	case PCDBinary:
		_, err = out.Write([]byte("DATA binary\n"))
		if err != nil {
			return err
		}

		buf := make([]byte, 16)
		cloud.Iterate(0, 0, func(position r3.Vector, d Data) bool {
			// Our point clouds are in mm, PCD files expect meters
			width := float32(position.X / 1000.0)
			height := float32(position.Y / 1000.)
			depth := float32(position.Z / 1000.)

			binary.LittleEndian.PutUint32(buf, math.Float32bits(width))
			binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(height))
			binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(depth))
			binary.LittleEndian.PutUint32(buf[12:], uint32(_colorToPCDInt(d)))
			_, err = out.Write(buf)
			return err == nil
		})
	default:
		return fmt.Errorf("unknown pcd type %d", outputType)
	}

	if err != nil {
		return err
	}
	return nil
}

func readPcdHeaderLine(in *bufio.Reader, name string) (string, error) {
	l, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(l, name) {
		return "", fmt.Errorf("line is supposed to start with %s but is %s", name, l)
	}

	return strings.TrimSpace(l[len(name)+1:]), nil
}

func readPcdHeaderLineCheck(in *bufio.Reader, name string, value string) error {
	l, err := readPcdHeaderLine(in, name)
	if err != nil {
		return err
	}
	if l != value {
		return fmt.Errorf("header (%s) supposed to be %s but is %s", name, value, l)
	}
	return nil
}

func readFloat(n uint32) float64 {
	f := float64(math.Float32frombits(n))
	return math.Round(f*10000) / 10000
}

// ReadPCD reads a pcd file format and returns a pointcloud. Very restrictive on the format for now.
func ReadPCD(inRaw io.Reader) (PointCloud, error) {
	in := bufio.NewReader(inRaw)

	err := readPcdHeaderLineCheck(in, "VERSION", ".7")
	if err != nil {
		return nil, err
	}

	err = readPcdHeaderLineCheck(in, "FIELDS", "x y z rgb")
	if err != nil {
		return nil, err
	}

	err = readPcdHeaderLineCheck(in, "SIZE", "4 4 4 4")
	if err != nil {
		return nil, err
	}

	err = readPcdHeaderLineCheck(in, "TYPE", "F F F I")
	if err != nil {
		return nil, err
	}

	err = readPcdHeaderLineCheck(in, "COUNT", "1 1 1 1")
	if err != nil {
		return nil, err
	}

	_, err = readPcdHeaderLine(in, "WIDTH")
	if err != nil {
		return nil, err
	}

	_, err = readPcdHeaderLine(in, "HEIGHT")
	if err != nil {
		return nil, err
	}

	err = readPcdHeaderLineCheck(in, "VIEWPOINT", "0 0 0 1 0 0 0")
	if err != nil {
		return nil, err
	}

	_, err = readPcdHeaderLine(in, "POINTS")
	if err != nil {
		return nil, err
	}

	outputTypeString, err := readPcdHeaderLine(in, "DATA")
	if err != nil {
		return nil, err
	}

	pc := New()

	switch outputTypeString {
	case "ascii":
		for {
			l, err := in.ReadString('\n')
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}

			var x, y, z float64
			var color int

			n, err := fmt.Sscanf(l, "%f %f %f %d", &x, &y, &z, &color)
			if err != nil {
				return nil, err
			}
			if n != 4 {
				return nil, fmt.Errorf("didn't find the correct number of things, got %d", n)
			}

			err = pc.Set(r3.Vector{x * 1000, y * 1000, z * 1000}, NewColoredData(_pcdIntToColor(color)))
			if err != nil {
				return nil, err
			}
		}
	case "binary":
		buf := make([]byte, 16)
		for {
			read, err := in.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if read < 16 {
				read2, err := in.Read(buf[read:])
				if err != nil {
					return nil, err
				}
				read += read2
			}
			if read != 16 {
				return nil, fmt.Errorf("invalid pcd binary, read %d", read)
			}

			x := readFloat(binary.LittleEndian.Uint32(buf))
			y := readFloat(binary.LittleEndian.Uint32(buf[4:]))
			z := readFloat(binary.LittleEndian.Uint32(buf[8:]))
			color := int(binary.LittleEndian.Uint32(buf[12:]))

			err = pc.Set(r3.Vector{x * 1000, y * 1000, z * 1000}, NewColoredData(_pcdIntToColor(color)))
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unknown pcd data type: %s", outputTypeString)
	}

	return pc, nil
}
