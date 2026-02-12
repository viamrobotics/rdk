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

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	lzf "github.com/zhuyie/golzf"
	"gonum.org/v1/gonum/num/quat"

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
func NewFromFile(filename, pcStructureType string) (PointCloud, error) {
	cfg, err := Find(pcStructureType)
	if err != nil {
		return nil, err
	}

	switch filepath.Ext(filename) {
	case ".las":
		return newFromLASFile(filename, cfg)
	case ".pcd":
		f, err := os.Open(filepath.Clean(filename))
		if err != nil {
			return nil, err
		}
		return readPCD(f, cfg)
	default:
		return nil, errors.Errorf("do not know how to read file %q", filename)
	}
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

// ToBytes takes a pointcloud object and converts it to bytes.
func ToBytes(cloud PointCloud) ([]byte, error) {
	if cloud == nil {
		return nil, errors.New("pointcloud cannot be nil")
	}
	var buf bytes.Buffer
	buf.Grow(200 + (cloud.Size() * 4 * 4)) // 4 numbers per point, each 4 bytes, 200 is header size
	if err := ToPCD(cloud, &buf, PCDBinary); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
		_, err = fmt.Fprintf(out, "DATA binary_compressed\n")
		if err != nil {
			return err
		}
	}
	if outputType == PCDCompressed {
		err = writePCDCompressed(cloud, out)
	} else {
		err = writePCDData(cloud, out, outputType)
	}
	if err != nil {
		return err
	}
	return nil
}

func writePCDData(cloud PointCloud, out io.Writer, pcdtype PCDType) error {
	var err error
	cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
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
	return err
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

// ReadPCD reads pcd.
func ReadPCD(inRaw io.Reader, pcStructureType string) (PointCloud, error) {
	cfg, err := Find(pcStructureType)
	if err != nil {
		return nil, err
	}
	return readPCD(inRaw, cfg)
}

func readPCD(inRaw io.Reader, cfg TypeConfig) (PointCloud, error) {
	pc, err := readPCDHelper(inRaw, cfg)
	if err != nil {
		return nil, err
	}
	return pc.FinalizeAfterReading()
}

func readPCDHelper(inRaw io.Reader, cfg TypeConfig) (PointCloud, error) {
	in := bufio.NewReader(inRaw)

	header, err := parsePCDHeader(in)
	if err != nil {
		return nil, err
	}

	pc := cfg.NewWithParams(int(header.points))

	switch header.data {
	case PCDAscii:
		return readPCDASCII(in, *header, pc)
	case PCDBinary:
		return readPCDBinary(in, *header, pc)
	case PCDCompressed:
		return readPCDCompressed(in, *header, pc)
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

// reorganizeToStructureOfArrays converts point cloud data from array-of-structures
// to structure-of-arrays format for better compression.
func reorganizeToStructureOfArrays(cloud PointCloud) ([]byte, error) {
	size := cloud.Size()
	if size == 0 {
		return nil, errors.New("empty point cloud")
	}

	hasColor := cloud.MetaData().HasColor
	var data []byte

	if hasColor {
		// Reserve space for x, y, z, rgb arrays
		data = make([]byte, 0, size*16) // 4 float32s per point

		// Separate arrays for x, y, z, rgb
		xData := make([]byte, 0, size*4)
		yData := make([]byte, 0, size*4)
		zData := make([]byte, 0, size*4)
		rgbData := make([]byte, 0, size*4)

		cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
			// Convert RDK units (millimeters) to meters for PCD
			x := pos.X / 1000.
			y := pos.Y / 1000.
			z := pos.Z / 1000.

			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(x)))
			xData = append(xData, buf...)

			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(y)))
			yData = append(yData, buf...)

			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(z)))
			zData = append(zData, buf...)

			c := _colorToPCDInt(d)
			binary.LittleEndian.PutUint32(buf, uint32(c))
			rgbData = append(rgbData, buf...)

			return true
		})

		// Combine all arrays
		data = append(data, xData...)
		data = append(data, yData...)
		data = append(data, zData...)
		data = append(data, rgbData...)
	} else {
		// Reserve space for x, y, z arrays
		data = make([]byte, 0, size*12) // 3 float32s per point

		// Separate arrays for x, y, z
		xData := make([]byte, 0, size*4)
		yData := make([]byte, 0, size*4)
		zData := make([]byte, 0, size*4)

		cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
			// Convert RDK units (millimeters) to meters for PCD
			x := pos.X / 1000.
			y := pos.Y / 1000.
			z := pos.Z / 1000.

			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(x)))
			xData = append(xData, buf...)

			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(y)))
			yData = append(yData, buf...)

			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(z)))
			zData = append(zData, buf...)

			return true
		})

		// Combine all arrays
		data = append(data, xData...)
		data = append(data, yData...)
		data = append(data, zData...)
	}

	return data, nil
}

// writePCDCompressed writes compressed point cloud data using LZF compression.
func writePCDCompressed(cloud PointCloud, out io.Writer) error {
	// Reorganize data to structure-of-arrays format
	uncompressedData, err := reorganizeToStructureOfArrays(cloud)
	if err != nil {
		return err
	}

	// Compress the data using LZF
	// Allocate output buffer with maximum possible size
	compressedData := make([]byte, len(uncompressedData)+len(uncompressedData)/64+16+3)
	compressedBytes, err := lzf.Compress(uncompressedData, compressedData)
	if err != nil {
		return errors.Wrap(err, "failed to compress point cloud data")
	}
	compressedData = compressedData[:compressedBytes]

	// Write compressed size (4 bytes)
	compressedSize := uint32(len(compressedData))
	if err := binary.Write(out, binary.LittleEndian, compressedSize); err != nil {
		return errors.Wrap(err, "failed to write compressed size")
	}

	// Write uncompressed size (4 bytes)
	uncompressedSize := uint32(len(uncompressedData))
	if err := binary.Write(out, binary.LittleEndian, uncompressedSize); err != nil {
		return errors.Wrap(err, "failed to write uncompressed size")
	}

	// Write compressed data
	if _, err := out.Write(compressedData); err != nil {
		return errors.Wrap(err, "failed to write compressed data")
	}

	return nil
}

// readPCDCompressed reads compressed point cloud data using LZF decompression.
func readPCDCompressed(in *bufio.Reader, header pcdHeader, pc PointCloud) (PointCloud, error) {
	// Read compressed size (4 bytes)
	var compressedSize uint32
	if err := binary.Read(in, binary.LittleEndian, &compressedSize); err != nil {
		return nil, errors.Wrap(err, "failed to read compressed size")
	}

	// Read uncompressed size (4 bytes)
	var uncompressedSize uint32
	if err := binary.Read(in, binary.LittleEndian, &uncompressedSize); err != nil {
		return nil, errors.Wrap(err, "failed to read uncompressed size")
	}

	// Read compressed data
	compressedData := make([]byte, compressedSize)
	if _, err := io.ReadFull(in, compressedData); err != nil {
		return nil, errors.Wrap(err, "failed to read compressed data")
	}

	// Decompress the data
	uncompressedData := make([]byte, uncompressedSize)
	decompressedBytes, err := lzf.Decompress(compressedData, uncompressedData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decompress point cloud data")
	}
	if decompressedBytes != int(uncompressedSize) {
		return nil, fmt.Errorf("decompressed size mismatch: expected %d, got %d", uncompressedSize, decompressedBytes)
	}

	// Parse the decompressed data from structure-of-arrays format
	return parseStructureOfArrays(uncompressedData, header, pc)
}

// parseStructureOfArrays parses structure-of-arrays format data back to point cloud.
func parseStructureOfArrays(data []byte, header pcdHeader, pc PointCloud) (PointCloud, error) {
	numPoints := int(header.points)
	if numPoints == 0 {
		return pc, nil
	}

	hasColor := header.fields == pcdPointColor
	expectedSize := numPoints * 3 * 4 // 3 float32s per point minimum
	if hasColor {
		expectedSize = numPoints * 4 * 4 // 4 float32s per point with color
	}

	if len(data) != expectedSize {
		return nil, fmt.Errorf("unexpected data size: got %d, expected %d", len(data), expectedSize)
	}

	// Parse structure-of-arrays format
	offset := 0
	pointSize := 4 // 4 bytes per float32

	for i := 0; i < numPoints; i++ {
		// Read x coordinate
		xOffset := offset + i*pointSize
		x := math.Float32frombits(binary.LittleEndian.Uint32(data[xOffset : xOffset+4]))

		// Read y coordinate
		yOffset := offset + numPoints*pointSize + i*pointSize
		y := math.Float32frombits(binary.LittleEndian.Uint32(data[yOffset : yOffset+4]))

		// Read z coordinate
		zOffset := offset + 2*numPoints*pointSize + i*pointSize
		z := math.Float32frombits(binary.LittleEndian.Uint32(data[zOffset : zOffset+4]))

		// Convert PCD units (meters) to millimeters for RDK
		point := r3.Vector{X: 1000. * float64(x), Y: 1000. * float64(y), Z: 1000. * float64(z)}

		var colorData Data
		if hasColor {
			// Read RGB data
			rgbOffset := offset + 3*numPoints*pointSize + i*pointSize
			rgb := binary.LittleEndian.Uint32(data[rgbOffset : rgbOffset+4])
			colorData = NewColoredData(_pcdIntToColor(int(rgb)))
		} else {
			colorData = NewBasicData()
		}

		if err := pc.Set(point, colorData); err != nil {
			return nil, err
		}
	}

	return pc, nil
}
