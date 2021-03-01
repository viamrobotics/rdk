package rimage

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.viam.com/robotcore/utils"
)

type DepthMap struct {
	width  int
	height int

	data [][]int
}

func (dm *DepthMap) HasData() bool {
	return dm.width > 0 && dm.data != nil
}

func (dm *DepthMap) Width() int {
	return dm.width
}

func (dm *DepthMap) Height() int {
	return dm.height
}

func (dm *DepthMap) Cols() int {
	return dm.width
}

func (dm *DepthMap) Rows() int {
	return dm.height
}

func (dm *DepthMap) Get(p image.Point) int {
	return dm.data[p.X][p.Y]
}

func (dm *DepthMap) GetDepth(x, y int) int {
	return dm.data[x][y]
}

func (dm *DepthMap) Set(x, y, val int) {
	dm.data[x][y] = val
}

func myMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (dm *DepthMap) Smooth() {
	centerX := dm.width / 2
	centerY := dm.height / 2
	if err := utils.Walk(centerX, centerY, 1+(myMax(dm.width, dm.height)/2), func(x, y int) error {
		dm._getDepthOrEstimate(x, y)
		return nil
	}); err != nil {
		// shouldn't happen
		panic(err)
	}
}

func (dm *DepthMap) _getDepthOrEstimate(x, y int) int {
	if x < 0 || y < 0 || x >= dm.width || y >= dm.height {
		return 0
	}
	z := dm.data[x][y]
	if z > 0 {
		return z
	}

	total := 0.0
	num := 0.0

	offset := 0
	for offset = 1; offset < 1000 && num == 0; offset++ {
		startX := myMax(0, x-offset)
		startY := myMax(0, y-offset)

		for a := startX; a < x+offset && a < dm.width; a++ {
			for b := startY; b < y+offset && b < dm.height; b++ {
				temp := dm.data[a][b]
				if temp == 0 {
					continue
				}
				total += float64(temp)
				num++
			}
		}
	}

	if num == 0 {
		return 0
	}

	dm.data[x][y] = int(total / num)
	return dm.data[x][y]
}

func _readNext(r io.Reader) (int64, error) {
	data := make([]byte, 8)
	x, err := r.Read(data)
	if x == 8 {
		return int64(binary.LittleEndian.Uint64(data)), nil
	}

	return 0, fmt.Errorf("got %d bytes, and %s", x, err)
}

func ParseDepthMap(fn string) (*DepthMap, error) {
	var f io.Reader

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
		return nil, fmt.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
	}

	dm.data = make([][]int, dm.width)

	for x := 0; x < dm.width; x++ {
		dm.data[x] = make([]int, dm.height)
		for y := 0; y < dm.height; y++ {
			temp, err := _readNext(f)
			if err != nil {
				return nil, err
			}
			dm.data[x][y] = int(temp)
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
		return nil, fmt.Errorf("i only know how to handle 2 bytes per pixel in new format, not %s", bytesPerPixelString)
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
	units = units * 1000 // m to mm

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
		return nil, fmt.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
	}

	temp := make([]byte, 2)
	dm.data = make([][]int, dm.width)

	for x := 0; x < dm.width; x++ {
		dm.data[x] = make([]int, dm.height)
	}

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
				return nil, fmt.Errorf("didn't read 2 bytes, got: %d err: %s x,y: %d,%x", n, err, x, y)
			}

			dm.data[x][y] = int(units * float64(binary.LittleEndian.Uint16(temp)))

		}
	}

	return &dm, nil
}

func (dm *DepthMap) WriteToFile(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	var gout *gzip.Writer
	var out io.Writer = f

	if filepath.Ext(fn) == ".gz" {
		gout = gzip.NewWriter(f)
		out = gout
		defer gout.Close()
	}

	err = dm.WriteTo(out)
	if err != nil {
		return err
	}

	if gout != nil {
		gout.Flush()
	}

	return f.Sync()
}

func (dm *DepthMap) WriteTo(out io.Writer) error {
	buf := make([]byte, 8)

	binary.LittleEndian.PutUint64(buf, uint64(dm.width))
	_, err := out.Write(buf)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint64(buf, uint64(dm.height))
	_, err = out.Write(buf)
	if err != nil {
		return err
	}

	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			binary.LittleEndian.PutUint64(buf, uint64(dm.data[x][y]))
			_, err = out.Write(buf)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (dm *DepthMap) MinMax() (int, int) {
	min := 100000
	max := 0

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

func (dm *DepthMap) ToPrettyPicture(hardMin, hardMax int) image.Image {
	min, max := dm.MinMax()

	if min < hardMin {
		min = hardMin
	}
	if max > hardMax {
		max = hardMax
	}

	img := image.NewRGBA(image.Rect(0, 0, dm.Width(), dm.Height()))

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
			img.Set(x, y, NewColorFromHSV(hue, 1.0, 1.0))
		}
	}

	return img
}

func (dm *DepthMap) Rotate(amount int) *DepthMap {
	if amount != 180 {
		// made this a panic
		panic(fmt.Errorf("vision.DepthMap can only rotate 180 degrees right now"))
	}

	dm2 := &DepthMap{
		width:  dm.width,
		height: dm.height,
		data:   make([][]int, dm.width),
	}

	// these are new coordinates
	for x := 0; x < dm.width; x++ {
		dm2.data[x] = make([]int, dm.height)
		for y := 0; y < dm.height; y++ {
			val := dm.data[dm.width-1-x][dm.height-1-y]
			dm2.data[x][y] = val
		}
	}
	return dm2
}

func NewEmptyDepthMap(width, height int) DepthMap {
	dm := DepthMap{
		width:  width,
		height: height,
		data:   make([][]int, width),
	}

	for x := 0; x < dm.width; x++ {
		dm.data[x] = make([]int, dm.height)
	}

	return dm
}

type dmWarpConnector struct {
	In  *DepthMap
	Out DepthMap
}

func (w *dmWarpConnector) Get(x, y int, buf []float64) {
	buf[0] = float64(w.In.GetDepth(x, y))
}

func (w *dmWarpConnector) Set(x, y int, data []float64) {
	w.Out.Set(x, y, int(data[0]))
}

func (w *dmWarpConnector) OutputDims() (int, int) {
	return w.Out.width, w.Out.height
}

func (w *dmWarpConnector) NumFields() int {
	return 1
}

func (dm *DepthMap) Warp(m TransformationMatrix, newSize image.Point) DepthMap {
	conn := &dmWarpConnector{dm, NewEmptyDepthMap(newSize.X, newSize.Y)}
	Warp(conn, m)
	return conn.Out
}
