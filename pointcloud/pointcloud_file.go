package pointcloud

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edaniels/lidario"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/spatialmath"
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
func NewFromFile(fn string, logger logging.Logger) (PointCloud, error) {
	switch filepath.Ext(fn) {
	case ".las":
		return NewFromLASFile(fn, logger)
	case ".pcd":
		f, err := os.Open(filepath.Clean(fn))
		if err != nil {
			return nil, err
		}
		return ReadPCD(f)
	default:
		return nil, errors.Errorf("do not know how to read file %q", fn)
	}
}

// pointValueDataTag encodes if the point has value data.
const pointValueDataTag = "rc|pv"

// NewFromLASFile returns a point cloud from reading a LAS file. If any
// lossiness of points could occur from reading it in, it's reported but is not
// an error.
func NewFromLASFile(fn string, logger logging.Logger) (PointCloud, error) {
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

	//nolint:nakedret
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

// ToPCD writes out a point cloud to a PCD file of the specified type.
func ToPCD(cloud PointCloud, out io.Writer, outputType PCDType) error {
	var err error

	_, err = fmt.Fprintf(out, "VERSION .7\n")
	if err != nil {
		return err
	}
	if cloud.MetaData().HasColor {
		_, err = fmt.Fprintf(out, "FIELDS x y z rgb\n"+

			"SIZE 4 4 4 4\n"+
			//nolint:dupword
			"TYPE F F F I\n"+

			"COUNT 1 1 1 1\n")
	} else {
		_, err = fmt.Fprintf(out, "FIELDS x y z\n"+

			"SIZE 4 4 4\n"+
			//nolint:dupword
			"TYPE F F F\n"+

			"COUNT 1 1 1\n")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "WIDTH %d\n"+
		"HEIGHT %d\n"+ // TODO (aidanglickman): If we support structured PointClouds, update this

		"VIEWPOINT 0 0 0 1 0 0 0\n"+ // TODO (aidanglickman): When PointClouds support transform metadata update this
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
		return errors.New("compressed PCD not yet implemented")
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
		// Converts RDK units (millimeters) to meters for PCD
		x := pos.X / 1000.
		y := pos.Y / 1000.
		z := pos.Z / 1000.
		if cloud.MetaData().HasColor {
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
			case PCDCompressed:
				return false // TODO(aidanglickman): Implement compressed PCD
			default:
				return false
			}
		} else {
			switch pcdtype {
			case PCDBinary:
				buf := make([]byte, 12)
				binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(x)))
				binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(float32(y)))
				binary.LittleEndian.PutUint32(buf[8:], math.Float32bits(float32(z)))
				_, err = out.Write(buf)
			case PCDAscii:
				_, err = fmt.Fprintf(out, "%f %f %f\n", x, y, z)
			case PCDCompressed:
				return false // TODO(aidanglickman): Implement compressed PCD
			default:
				return false
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

type pcdHeader struct {
	fields    pcdFieldType
	size      []uint64
	valTypes  []string
	count     []uint64
	width     uint64
	height    uint64
	viewpoint spatialmath.Pose
	points    uint64
	data      PCDType
}

const pcdCommentChar = "#"

var pcdHeaderFields = []string{"VERSION", "FIELDS", "SIZE", "TYPE", "COUNT", "WIDTH", "HEIGHT", "VIEWPOINT", "POINTS", "DATA"}

func parsePCDHeaderLine(line string, index int, pcdHeader *pcdHeader) error {
	var err error
	name := pcdHeaderFields[index]
	field, value, _ := strings.Cut(line, " ")
	tokens := strings.Split(value, " ")
	if field != name {
		return fmt.Errorf("line is supposed to start with %s but is %s", name, line)
	}

	switch name {
	case "VERSION":
		if value != ".7" && value != "0.7" { // This can be expanded later if desired, though I doubt we will need/want that
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
			return fmt.Errorf("unexpected number of fields %d in SIZE line", len(tokens))
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
			return fmt.Errorf("unexpected number of fields %d in TYPE line", len(tokens))
		}
		copy(pcdHeader.valTypes, tokens)

	case "COUNT":
		if len(tokens) != int(pcdHeader.fields) {
			return fmt.Errorf("unexpected number of fields %d in COUNT line", len(tokens))
		}
		pcdHeader.count = make([]uint64, len(tokens))
		for i, token := range tokens {
			pcdHeader.count[i], err = strconv.ParseUint(token, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid COUNT field %s: %w", token, err)
			}
		}
	case "WIDTH":
		pcdHeader.width, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid WIDTH field %s: %w", value, err)
		}
	case "HEIGHT":
		pcdHeader.height, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid HEIGHT field %s: %w", value, err)
		}
	case "VIEWPOINT":
		if len(tokens) != 7 {
			return fmt.Errorf("unexpected number of fields in VIEWPOINT line. Expected 7, got %d", len(tokens))
		}
		viewpoint := [7]float64{}
		for i, token := range tokens {
			viewpoint[i], err = strconv.ParseFloat(token, 64)
			if err != nil {
				return fmt.Errorf("invalid VIEWPOINT field %s: %w", token, err)
			}
		}
		pcdHeader.viewpoint = spatialmath.NewPose(
			r3.Vector{X: viewpoint[0], Y: viewpoint[1], Z: viewpoint[2]},
			spatialmath.QuatToOV(quat.Number{Real: viewpoint[3], Imag: viewpoint[4], Jmag: viewpoint[5], Kmag: viewpoint[6]}),
		)
	case "POINTS":
		var points uint64
		points, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid POINTS field %s: %w", value, err)
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
		default:
			return fmt.Errorf("unsupported data type %s", value)
		}
	}

	return nil
}

func parsePCDHeader(in *bufio.Reader) (*pcdHeader, error) {
	header := &pcdHeader{}
	headerLineCount := 0
	for headerLineCount < len(pcdHeaderFields) {
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("error reading header line %d: %w", headerLineCount, err)
		}
		line, _, _ = strings.Cut(line, pcdCommentChar)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		err = parsePCDHeaderLine(line, headerLineCount, header)
		if err != nil {
			return nil, err
		}
		headerLineCount++
	}
	return header, nil
}

// PCType is the type of point cloud to read the PCD file into.
type PCType int

const (
	// BasicType is a selector for a pointcloud backed by a BasicPointCloud.
	BasicType PCType = 0
	// KDTreeType is a selector for a pointcloud backed by a KD Tree.
	KDTreeType PCType = 1
	// BasicOctreeType is a selector for a pointcloud backed by a Basic Octree.
	BasicOctreeType PCType = 2
)

// ReadPCD reads a PCD file into a pointcloud.
func ReadPCD(inRaw io.Reader) (PointCloud, error) {
	return readPCDHelper(inRaw, BasicType)
}

// ReadPCDToKDTree reads a PCD file into a KD Tree pointcloud.
func ReadPCDToKDTree(inRaw io.Reader) (*KDTree, error) {
	cloud, err := readPCDHelper(inRaw, KDTreeType)
	if err != nil {
		return nil, err
	}
	kd, ok := (cloud).(*KDTree)
	if !ok {
		return nil, fmt.Errorf("pointcloud %v is not a KD Tree", cloud)
	}
	return kd, nil
}

// ReadPCDToBasicOctree reads a PCD file into a basic octree.
func ReadPCDToBasicOctree(inRaw io.Reader) (*BasicOctree, error) {
	cloud, err := readPCDHelper(inRaw, BasicOctreeType)
	if err != nil {
		return nil, err
	}
	basicOct, ok := (cloud).(*BasicOctree)
	if !ok {
		return nil, errors.Errorf("pointcloud %v is not a basic octree", cloud)
	}
	return basicOct, nil
}

func readPCDHelper(inRaw io.Reader, pctype PCType) (PointCloud, error) {
	var pc PointCloud
	in := bufio.NewReader(inRaw)
	header, err := parsePCDHeader(in)
	if err != nil {
		return nil, err
	}
	switch pctype {
	case BasicType:
		pc = NewWithPrealloc(int(header.points))
	case KDTreeType:
		pc = NewKDTreeWithPrealloc(int(header.points))
	case BasicOctreeType:

		// Extract data from bufio.Reader to make a copy for metadata acquisition
		buf, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		in.Reset(bufio.NewReader(bytes.NewReader(buf)))

		meta, err := parsePCDMetaData(*bufio.NewReader(bytes.NewReader(buf)), *header)
		if err != nil {
			return nil, err
		}

		pc, err = NewBasicOctree(getCenterFromPcMetaData(meta), getMaxSideLengthFromPcMetaData(meta))
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported point cloud type %d", pctype)
	}
	switch header.data {
	case PCDAscii:
		return readPCDASCII(in, *header, pc)
	case PCDBinary:
		return readPCDBinary(in, *header, pc)
	case PCDCompressed:
		// return readPCDCompressed(in, header)
		return nil, errors.New("compressed pcd not yet supported")
	default:
		return nil, fmt.Errorf("unsupported pcd data type %v", header.data)
	}
}

func extractPCDPointASCII(in *bufio.Reader, header pcdHeader, i int) (PointAndData, error) {
	line, err := in.ReadString('\n')
	if err != nil {
		return PointAndData{}, err
	}
	line = strings.TrimSpace(line)
	tokens := strings.Split(line, " ")
	if len(tokens) != int(header.fields) {
		return PointAndData{}, fmt.Errorf("unexpected number of fields in point %d", i)
	}
	point := make([]float64, len(tokens))
	for j, token := range tokens {
		point[j], err = strconv.ParseFloat(token, 64)
		if err != nil {
			return PointAndData{}, fmt.Errorf("invalid point %d field %s: %w", i, token, err)
		}
	}
	pcPoint, data, err := readSliceToPoint(point, header)
	if err != nil {
		return PointAndData{}, err
	}

	return PointAndData{P: pcPoint, D: data}, nil
}

func readPCDASCII(in *bufio.Reader, header pcdHeader, pc PointCloud) (PointCloud, error) {
	for i := 0; i < int(header.points); i++ {
		pd, err := extractPCDPointASCII(in, header, i)
		if err != nil {
			return nil, err
		}
		err = pc.Set(pd.P, pd.D)
		if err != nil {
			return nil, err
		}
	}
	return pc, nil
}

func extractPCDPointBinary(in *bufio.Reader, header pcdHeader) (PointAndData, error) {
	var err error
	pointBuf := make([]float64, 3)
	colorData := NewBasicData()
	for j := 0; j < 3; j++ {
		buf, err := readBuffer(in, header, j)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return PointAndData{}, err
		}
		pointBuf[j] = readFloat(binary.LittleEndian.Uint32(buf))
	}

	// Converts PCD units (meters) to millimeters for RDK
	point := r3.Vector{X: 1000. * pointBuf[0], Y: 1000. * pointBuf[1], Z: 1000. * pointBuf[2]}

	if header.fields == pcdPointColor && !errors.Is(err, io.EOF) {
		buf, err := readBuffer(in, header, 3)
		if err != nil {
			return PointAndData{}, err
		}
		colorBuf := int(binary.LittleEndian.Uint32(buf))
		colorData = NewColoredData(_pcdIntToColor(colorBuf))
	}

	return PointAndData{P: point, D: colorData}, nil
}

func readPCDBinary(in *bufio.Reader, header pcdHeader, pc PointCloud) (PointCloud, error) {
	for i := 0; i < int(header.points); i++ {
		pd, err := extractPCDPointBinary(in, header)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		err = pc.Set(pd.P, pd.D)
		if err != nil {
			return nil, err
		}
	}
	return pc, nil
}

func parsePCDMetaData(in bufio.Reader, header pcdHeader) (MetaData, error) {
	meta := NewMetaData()
	switch header.data {
	case PCDAscii:
		for i := 0; i < int(header.points); i++ {
			pd, err := extractPCDPointASCII(&in, header, i)
			if err != nil {
				return MetaData{}, err
			}
			meta.Merge(pd.P, pd.D)
		}

	case PCDBinary:
		for i := 0; i < int(header.points); i++ {
			pd, err := extractPCDPointBinary(&in, header)
			if err != nil {
				return MetaData{}, err
			}
			meta.Merge(pd.P, pd.D)
		}
	case PCDCompressed:
		// return readPCDCompressed(in, header)
		return MetaData{}, errors.New("compressed pcd not yet supported")
	default:
		return MetaData{}, fmt.Errorf("unsupported pcd data type %v", header.data)
	}

	return meta, nil
}

// GetPCDMetaData returns the metadata for the PCD read from the provided reader.
func GetPCDMetaData(inRaw io.Reader) (MetaData, error) {
	in := bufio.NewReader(inRaw)
	header, err := parsePCDHeader(in)
	if err != nil {
		return MetaData{}, err
	}
	return parsePCDMetaData(*in, *header)
}

// reads a specified amount of bytes from a buffer. The number of bytes specified is defined from the pcd.
func readBuffer(in *bufio.Reader, header pcdHeader, index int) ([]byte, error) {
	buf := make([]byte, header.size[index])
	read, err := io.ReadFull(in, buf)
	if err != nil {
		return nil, err
	}
	if read != int(header.size[index]) {
		return nil, fmt.Errorf("unexpected number of bytes read %d", read)
	}
	return buf, nil
}

func readSliceToPoint(slice []float64, header pcdHeader) (r3.Vector, Data, error) {
	// multiply by 1000 as RDK uses millimeters and PCD expects meters
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
