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
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/edaniels/lidario"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/num/quat"
)

// PCDType is the format of a pcd file.
type PCDType int

const (
	// PCDAscii ascii format for pcd.
	PCDAscii PCDType = 0
	// PCDBinary binary format for pcd.
	PCDBinary PCDType = 1
	// PCDCompressed binary format for pcd.
	PCDCompressed PCDType = 2
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

		v := r3.Vector{X: x, Y: y, Z: z}
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

func ToPCD(cloud PointCloud, out io.Writer, outputType PCDType) error {
	var err error

	_, err = fmt.Fprintf(out, "VERSION .7\n")
	if err != nil {
		return err
	}
	switch cloud.MetaData().HasColor {
	case true:
		_, err = fmt.Fprintf(out, "FIELDS x y z rgb\n"+
			"SIZE 4 4 4 4\n"+
			"TYPE F F F I\n"+
			"COUNT 1 1 1 1\n")
	case false:
		_, err = fmt.Fprintf(out, "FIELDS x y z\n"+
			"SIZE 4 4 4\n"+
			"TYPE F F F\n"+
			"COUNT 1 1 1\n")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "WIDTH %d\n"+
		"HEIGHT %d\n"+ // TODO (aidanglickman): If we support structured PointClouds, update this
		"VIEWPOINT 0 0 0 1 0 0 0\n"+ // TODO (aidanglickman): When PointClouds support transfom metadata update this
		"POINTS %d\n",
		cloud.Size(),
		1,
		cloud.Size())
	if err != nil {
		return err
	}

	switch outputType {
	case PCDBinary:
		_, err = fmt.Fprintf(out, "DATA binary\n")
		if err != nil {
			return err
		}
	case PCDAscii:
		_, err = fmt.Fprintf(out, "DATA ascii\n")
		if err != nil {
			return err
		}
	case PCDCompressed:
		// _, err = fmt.Fprintf(out, "DATA binary_compressed\n")
		// if err != nil {
		// 	return err
		// }
		return fmt.Errorf("compressed PCD not yet implemented")
	}
	err = writePCDData(cloud, out, outputType)
	if err != nil {
		return err
	}
	return nil
}

func writePCDData(cloud PointCloud, out io.Writer, pcdtype PCDType) error {
	cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
		var err error
		x := pos.X / 1000.
		y := pos.Y / 1000.
		z := pos.Z / 1000.
		switch cloud.MetaData().HasColor {
		case true:
			c := _colorToPCDInt(d)
			switch pcdtype {
			case PCDBinary:
				buf := make([]byte, 16)
				binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(x)))
				binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(float32(y)))
				binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(float32(z)))
				binary.LittleEndian.PutUint32(buf[12:], uint32(c))
				_, err = out.Write(buf)
			case PCDAscii:
				_, err = fmt.Fprintf(out, "%f %f %f %d\n", x, y, z, c)
			}
		case false:
			switch pcdtype {
			case PCDBinary:
				buf := make([]byte, 12)
				binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(x)))
				binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(float32(y)))
				binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(float32(z)))
				_, err = out.Write(buf)
			case PCDAscii:
				_, err = fmt.Fprintf(out, "%f %f %f\n", x, y, z)
			}
		}
		return err == nil
	})
	return nil
}

func readFloat(n uint32) float64 {
	f := float64(math.Float32frombits(n))
	return math.Round(f*10000) / 10000
}

type pcdFieldType int

const (
	pcdPointOnly  pcdFieldType = 3
	pcdPointColor pcdFieldType = 4
)

type pcdValType string

const (
	pcdValFloat pcdValType = "F"
	pcdValInt   pcdValType = "I"
	pcdValUInt  pcdValType = "U"
)

type pcdHeader struct {
	fields    pcdFieldType
	size      []uint64
	type_     []pcdValType
	count     []uint64
	width     uint64
	height    uint64
	viewpoint spatialmath.Pose
	points    uint64
	data      PCDType
}

const PCD_COMMENT_CHAR = "#"

var PCD_HEADER_FIELDS = []string{"VERSION", "FIELDS", "SIZE", "TYPE", "COUNT", "WIDTH", "HEIGHT", "VIEWPOINT", "POINTS", "DATA"}

func parsePCDHeaderLine(line string, index int, pcdHeader *pcdHeader) error {
	var err error
	name := PCD_HEADER_FIELDS[index]
	field, value, _ := strings.Cut(line, " ")
	tokens := strings.Split(value, " ")
	if field != name {
		return fmt.Errorf("line is supposed to start with %s but is %s", name, line)
	}

	switch name {
	case "VERSION":
		if value != ".7" { // This can be expanded later if desired, though I doubt we will need/want that
			return fmt.Errorf("unsupported pcd version %s", value)
		}
	case "FIELDS":
		switch value {
		case "x y z":
			pcdHeader.fields = pcdPointOnly
		case "x y z rgb":
			pcdHeader.fields = pcdPointColor
		default:
			return fmt.Errorf("unsupported pcd fields %s", value)
		}
	case "SIZE":
		if len(tokens) != int(pcdHeader.fields) {
			return fmt.Errorf("unexpected number of fields in SIZE line")
		}
		pcdHeader.size = make([]uint64, len(tokens))
		for i, token := range tokens {
			pcdHeader.size[i], err = strconv.ParseUint(token, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid SIZE field %s", token)
			}
		}
	case "TYPE":
		if len(tokens) != int(pcdHeader.fields) {
			return fmt.Errorf("unexpected number of fields in TYPE line")
		}
		pcdHeader.type_ = make([]pcdValType, len(tokens))
		for i, token := range tokens {
			pcdHeader.type_[i] = pcdValType(token)
		}
	case "COUNT":
		if len(tokens) != int(pcdHeader.fields) {
			return fmt.Errorf("unexpected number of fields in COUNT line")
		}
		pcdHeader.count = make([]uint64, len(tokens))
		for i, token := range tokens {
			pcdHeader.count[i], err = strconv.ParseUint(token, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid COUNT field %s: %s", token, err)
			}
		}
	case "WIDTH":
		pcdHeader.width, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid WIDTH field %s: %s", value, err)
		}
	case "HEIGHT":
		pcdHeader.height, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid HEIGHT field %s: %s", value, err)
		}
	case "VIEWPOINT":
		if len(tokens) != 7 {
			return fmt.Errorf("unexpected number of fields in VIEWPOINT line. Expected 7, got %d", len(tokens))
		}
		viewpoint := [7]float64{}
		for i, token := range tokens {
			viewpoint[i], err = strconv.ParseFloat(token, 64)
			if err != nil {
				return fmt.Errorf("invalid VIEWPOINT field %s: %s", token, err)
			}
		}
		pcdHeader.viewpoint = spatialmath.NewPoseFromOrientationVector(
			r3.Vector{X: viewpoint[0], Y: viewpoint[1], Z: viewpoint[2]},
			spatialmath.QuatToOV(quat.Number{Real: viewpoint[3], Imag: viewpoint[4], Jmag: viewpoint[5], Kmag: viewpoint[6]}),
		)
	case "POINTS":
		var points uint64
		points, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid POINTS field %s: %s", value, err)
		}
		if points != pcdHeader.width*pcdHeader.height {
			return fmt.Errorf("POINTS field %d does not match WIDTH*HEIGHT %d", points, pcdHeader.width*pcdHeader.height)
		}
		pcdHeader.points = points
	case "DATA":
		switch value {
		case "ascii":
			pcdHeader.data = PCDAscii
		case "binary":
			pcdHeader.data = PCDBinary
		case "binary_compressed":
			pcdHeader.data = PCDCompressed
		}
	}

	return nil
}

func ReadPCD(inRaw io.Reader) (PointCloud, error) {
	header := pcdHeader{}
	in := bufio.NewReader(inRaw)
	var line string
	var err error
	headerLineCount := 0
	for headerLineCount < len(PCD_HEADER_FIELDS) {
		line, err = in.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading header line %d: %s", headerLineCount, err)
		}
		line, _, _ = strings.Cut(line, PCD_COMMENT_CHAR)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		err := parsePCDHeaderLine(line, headerLineCount, &header)
		if err != nil {
			return nil, err
		}
		headerLineCount++
	}
	switch header.data {
	case PCDAscii:
		return readPCDAscii(in, header)
	case PCDBinary:
		return readPCDBinary(in, header)
	case PCDCompressed:
		// return readPCDCompressed(in, header)
		return nil, fmt.Errorf("compressed pcd not yet supported")
	default:
		return nil, fmt.Errorf("unsupported pcd data type %v", header.data)
	}
}

func readPCDAscii(in *bufio.Reader, header pcdHeader) (PointCloud, error) {
	pc := NewWithPrealloc(int(header.points))
	for i := 0; i < int(header.points); i++ {
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		tokens := strings.Split(line, " ")
		if len(tokens) != int(header.fields) {
			return nil, fmt.Errorf("unexpected number of fields in point %d", i)
		}
		point := make([]float64, len(tokens))
		for j, token := range tokens {
			point[j], err = strconv.ParseFloat(token, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid point %d field %s: %s", i, token, err)
			}
		}
		pcPoint, data, err := readSliceToPoint(point, header)
		if err != nil {
			return nil, err
		}
		pc.Set(pcPoint, data)
	}
	return pc, nil
}

func readPCDBinary(in *bufio.Reader, header pcdHeader) (PointCloud, error) {
	var err error
	var read int
	pc := NewWithPrealloc(int(header.points))
	for i := 0; i < int(header.points); i++ {
		pointBuf := make([]float64, int(header.fields))
		for j := 0; j < int(header.fields); j++ {
			buf := make([]byte, header.size[j])
			read, err = in.Read(buf)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			if read != int(header.size[j]) {
				return nil, fmt.Errorf("unexpected number of bytes read %d", read)
			}
			pointBuf[j] = readFloat(binary.LittleEndian.Uint32(buf))
		}
		point, data, err := readSliceToPoint(pointBuf, header)
		if err != nil {
			return nil, err
		}
		pc.Set(point, data)
	}
	return pc, nil
}

func readSliceToPoint(slice []float64, header pcdHeader) (r3.Vector, Data, error) {
	pos := r3.Vector{X: 1000. * slice[0], Y: 1000. * slice[1], Z: 1000. * slice[2]}
	switch header.fields {
	// This can be expanded to support more field types if needed.
	case pcdPointOnly:
		return pos, NewBasicData(), nil

	case pcdPointColor:
		color := NewColoredData(_pcdIntToColor(int(slice[3])))
		return pos, color, nil
	default:
		return r3.Vector{}, nil, fmt.Errorf("unsupported pcd field type %d", header.fields)
	}
}
