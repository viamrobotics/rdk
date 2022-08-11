package rimage

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
)

// Depth TODO.
type Depth uint16

// MaxDepth TODO.
const MaxDepth = Depth(math.MaxUint16)

// DepthMap TODO.
type DepthMap struct {
	width  int
	height int

	data []Depth
}

// NewEmptyDepthMap returns an unset depth map with the given dimensions.
func NewEmptyDepthMap(width, height int) *DepthMap {
	dm := &DepthMap{
		width:  width,
		height: height,
		data:   make([]Depth, width*height),
	}

	return dm
}

// Clone makes a copy of the depth map.
func (dm *DepthMap) Clone() *DepthMap {
	ddm := NewEmptyDepthMap(dm.Width(), dm.Height())
	copy(ddm.data, dm.data)
	return ddm
}

func (dm *DepthMap) kxy(x, y int) int {
	return (y * dm.width) + x
}

// Width TODO.
func (dm *DepthMap) Width() int {
	return dm.width
}

// Height TODO.
func (dm *DepthMap) Height() int {
	return dm.height
}

// Bounds TODO.
func (dm *DepthMap) Bounds() image.Rectangle {
	return image.Rect(0, 0, dm.width, dm.height)
}

// Get TODO.
func (dm *DepthMap) Get(p image.Point) Depth {
	return dm.data[dm.kxy(p.X, p.Y)]
}

// GetDepth TODO.
func (dm *DepthMap) GetDepth(x, y int) Depth {
	return dm.data[dm.kxy(x, y)]
}

// Set TODO.
func (dm *DepthMap) Set(x, y int, val Depth) {
	dm.data[dm.kxy(x, y)] = val
}

// Contains returns whether or not a point is within bounds of the depth map.
func (dm *DepthMap) Contains(x, y int) bool {
	return x >= 0 && y >= 0 && x < dm.width && y < dm.height
}

// At returns the depth value as a color.Color so DepthMap can implement image.Image.
func (dm *DepthMap) At(x, y int) color.Color {
	return color.Gray16{uint16(dm.GetDepth(x, y))}
}

// ColorModel for DepthMap so that it implements image.Image.
func (dm *DepthMap) ColorModel() color.Model { return &TheDepthModel{} }

// TheDepthModel is the color model used to convert other colors to its own color.
type TheDepthModel struct{}

// Convert will use the Gray16 model as a stand-in for the depth model.
func (tdm *TheDepthModel) Convert(c color.Color) color.Color {
	return color.Gray16Model.Convert(c)
}

// SubImage TODO.
func (dm *DepthMap) SubImage(r image.Rectangle) *DepthMap {
	if r.Empty() {
		return &DepthMap{}
	}
	xmin, xmax := utils.MinInt(dm.width, r.Min.X), utils.MinInt(dm.width, r.Max.X)
	ymin, ymax := utils.MinInt(dm.height, r.Min.Y), utils.MinInt(dm.height, r.Max.Y)
	if xmin == xmax || ymin == ymax { // return empty DepthMap
		return &DepthMap{width: utils.MaxInt(0, xmax-xmin), height: utils.MaxInt(0, ymax-ymin), data: []Depth{}}
	}
	width := xmax - xmin
	height := ymax - ymin
	newData := make([]Depth, 0, width*height)
	for y := ymin; y < ymax; y++ {
		begin, end := (y*dm.width)+xmin, (y*dm.width)+xmax
		newData = append(newData, dm.data[begin:end]...)
	}
	return &DepthMap{width: width, height: height, data: newData}
}

func _readNext(r io.Reader) (int64, error) {
	data := make([]byte, 8)
	x, err := r.Read(data)
	if x == 8 {
		return int64(binary.LittleEndian.Uint64(data)), nil
	}

	return 0, errors.Wrapf(err, "got %d bytes", x)
}

// ParseDepthMap parses a depth map from the given file. It knows
// how to handle compressed files as well.
func ParseDepthMap(fn string) (*DepthMap, error) {
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
func ReadDepthMap(f *bufio.Reader) (*DepthMap, error) {
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

// ConvertImageToDepthMap takes an image and figures out if it's already a DepthMap
// or if it can be converted into one.
func ConvertImageToDepthMap(img image.Image) (*DepthMap, error) {
	switch ii := img.(type) {
	case *DepthMap:
		return ii, nil
	case *imageWithDepth:
		return ii.Depth, nil
	case *image.Gray16:
		return imageToDepthMap(ii), nil
	default:
		return nil, errors.Errorf("don't know how to make DepthMap from %T", img)
	}
}

// WriteToFile writes this depth map to the given file.
func (dm *DepthMap) WriteToFile(fn string) (err error) {
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

	_, err = dm.WriteTo(out)
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

// WriteTo writes this depth map to the given writer.
func (dm *DepthMap) WriteTo(out io.Writer) (int64, error) {
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

// MinMax TODO.
func (dm *DepthMap) MinMax() (Depth, Depth) {
	min := MaxDepth
	max := Depth(0)

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			if z < min {
				min = z
			}
			if z > max {
				max = z
			}
		}
	}

	return min, max
}

// ToGray16Picture converts this depth map into a grayscale image of the
// same dimensions.
func (dm *DepthMap) ToGray16Picture() image.Image {
	grayScale := image.NewGray16(image.Rect(0, 0, dm.Width(), dm.Height()))

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			val := dm.GetDepth(x, y)
			grayColor := color.Gray16{uint16(val)}
			grayScale.Set(x, y, grayColor)
		}
	}

	return grayScale
}

// ToPrettyPicture TODO.
func (dm *DepthMap) ToPrettyPicture(hardMin, hardMax Depth) *Image {
	min, max := dm.MinMax()

	if hardMin > 0 && min < hardMin {
		min = hardMin
	}
	if hardMax > 0 && max > hardMax {
		max = hardMax
	}

	img := NewImage(dm.Width(), dm.Height())

	span := float64(max) - float64(min)

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			p := image.Point{x, y}
			z := dm.Get(p)
			if z == 0 {
				continue
			}

			if z < min {
				z = min
			}
			if z > max {
				z = max
			}

			ratio := float64(z-min) / span

			hue := 30 + (200.0 * ratio)
			img.SetXY(x, y, NewColorFromHSV(hue, 1.0, 1.0))
		}
	}

	return img
}

// Rotate rotates a copy of this depth map clockwise by the given amount.
func (dm *DepthMap) Rotate(amount int) *DepthMap {
	if amount == 180 {
		return dm.Rotate180()
	}

	if amount == 90 {
		return dm.Rotate90(true)
	}

	if amount == -90 {
		return dm.Rotate90(false)
	}

	// made this a panic
	panic("vision.DepthMap can only rotate 180 degrees right now")
}

// Rotate90 rotates a copy of this depth map either by 90 degrees clockwise or counterclockwise.
func (dm *DepthMap) Rotate90(clockwise bool) *DepthMap {
	newWidth := dm.height
	newHeight := dm.width

	dm2 := &DepthMap{
		width:  newWidth,
		height: newHeight,
		data:   make([]Depth, newWidth*newHeight),
	}

	newCol, newRow := 0, 0
	if clockwise {
		for oldRow := dm.height - 1; oldRow >= 0; oldRow-- {
			newRow = 0
			for oldCol := 0; oldCol < dm.width; oldCol++ {
				val := dm.GetDepth(oldCol, oldRow)
				dm2.Set(newCol, newRow, val)
				newRow++
			}
			newCol++
		}
	} else { // counter-clockwise
		for oldCol := dm.width - 1; oldCol >= 0; oldCol-- {
			newCol = 0
			for oldRow := 0; oldRow < dm.height; oldRow++ {
				val := dm.GetDepth(oldCol, oldRow)
				dm2.Set(newCol, newRow, val)
				newCol++
			}
			newRow++
		}
	}
	return dm2
}

// Rotate180 rotates a copy of this depth map by 180 degrees.
func (dm *DepthMap) Rotate180() *DepthMap {
	dm2 := &DepthMap{
		width:  dm.width,
		height: dm.height,
		data:   make([]Depth, dm.width*dm.height),
	}

	k := 0 // optimization
	for y := 0; y < dm.height; y++ {
		for x := 0; x < dm.width; x++ {
			val := dm.GetDepth(dm.width-1-x, dm.height-1-y)
			dm2.data[k] = val
			// if k != dm2.kxy(x,y) { panic("oops") }
			k++
		}
	}
	return dm2
}

// AverageDepthAndStats returns average distance, average distance to avg.
// TODO(erh): should this be std. dev?
func (dm *DepthMap) AverageDepthAndStats(p image.Point, radius int) (float64, float64) {
	total := 0.0

	heights := []Depth{}

	for x := p.X - radius; x <= p.X+radius; x++ {
		if x < 0 || x >= dm.width {
			continue
		}
		for y := p.Y - radius; y <= p.Y+radius; y++ {
			if y < 0 || y >= dm.height {
				continue
			}

			h := dm.GetDepth(x, y)
			if h == 0 {
				continue
			}

			heights = append(heights, h)
			total += float64(h)
		}
	}

	if len(heights) == 0 {
		return 0.0, 0.0
	}

	avg := total / float64(len(heights))

	total = 0.0 // re-using for avg distance
	for _, h := range heights {
		d := math.Abs(float64(h) - avg)
		total += d
	}

	return avg, total / float64(len(heights))
}

// InterestingPixels TODO.
func (dm *DepthMap) InterestingPixels(t float64) *image.Gray {
	out := image.NewGray(dm.Bounds())

	for x := 0; x < dm.width; x += 3 {
		for y := 0; y < dm.height; y += 3 {
			_, avgDistance := dm.AverageDepthAndStats(image.Point{x + 1, y + 1}, 1)

			clr := color.Gray{0}
			if avgDistance > t {
				clr = color.Gray{255}
			}

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					out.SetGray(xx, yy, clr)
				}
			}
		}
	}

	return out
}

// ToPointCloud returns a lazy read only pointcloud.
func (dm *DepthMap) ToPointCloud(p Projector) pointcloud.PointCloud {
	return newDMPointCloudAdapter(dm, p)
}

type dmWarpConnector struct {
	In  *DepthMap
	Out *DepthMap
}

func (w *dmWarpConnector) Get(x, y int, buf []float64) bool {
	d := w.In.GetDepth(x, y)
	if d == 0 {
		return false
	}
	buf[0] = float64(d)
	return true
}

func (w *dmWarpConnector) Set(x, y int, data []float64) {
	w.Out.Set(x, y, Depth(data[0]))
}

func (w *dmWarpConnector) OutputDims() (int, int) {
	return w.Out.width, w.Out.height
}

func (w *dmWarpConnector) NumFields() int {
	return 1
}

// Warp returns a copy of this depth map warped by the given transformation matrix
// into a new size.
func (dm *DepthMap) Warp(m TransformationMatrix, newSize image.Point) *DepthMap {
	conn := &dmWarpConnector{dm, NewEmptyDepthMap(newSize.X, newSize.Y)}
	Warp(conn, m)
	return conn.Out
}

// ConvertDepthMapToLuminanceFloat converts this depth map into a grayscale image of the
// same dimensions.
func (dm *DepthMap) ConvertDepthMapToLuminanceFloat() *mat.Dense {
	out := mat.NewDense(dm.height, dm.width, nil)
	utils.ParallelForEachPixel(image.Point{dm.width, dm.height}, func(x int, y int) {
		d := dm.GetDepth(x, y)
		out.Set(y, x, float64(d))
	})
	return out
}
