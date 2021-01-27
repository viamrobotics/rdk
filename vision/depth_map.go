package vision

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

	"gocv.io/x/gocv"
)

type DepthMap struct {
	width  int
	height int

	min int
	max int

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

func myMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (dm *DepthMap) Smooth() {
	centerX := dm.width / 2
	centerY := dm.height / 2
	dm.max = 0
	dm.min = 100000
	if err := Walk(centerX, centerY, myMax(dm.width, dm.height), func(x, y int) error {
		temp := dm._getDepthOrEstimate(x, y)
		if temp > 0 && temp < dm.min {
			dm.min = temp
		}
		if temp > dm.max {
			dm.max = temp
		}
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

func (dm *DepthMap) ToMat() gocv.Mat {
	m := gocv.NewMatWithSize(dm.height, dm.width, gocv.MatTypeCV64F)
	raw, err := m.DataPtrFloat64()
	if err != nil {
		panic(err)
	}
	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			z := dm._getDepthOrEstimate(x, y)
			raw[y*dm.width+x] = float64(z)
		}
	}
	return m
}

func _readNext(r io.Reader) (int, error) {
	data := make([]byte, 8)
	x, err := r.Read(data)
	if x == 8 {
		return int(binary.LittleEndian.Uint64(data)), nil
	}

	return 0, fmt.Errorf("got %d bytes, and %s", x, err)
}

func ParseDepthMap(fn string) (DepthMap, error) {
	dm := DepthMap{}
	var f io.Reader

	f, err := os.Open(fn)
	if err != nil {
		return dm, err
	}

	if filepath.Ext(fn) == ".gz" {
		f, err = gzip.NewReader(f)
		if err != nil {
			return dm, err
		}
	}

	return ReadDepthMap(bufio.NewReader(f))
}

func ReadDepthMap(f *bufio.Reader) (DepthMap, error) {
	var err error
	dm := DepthMap{}

	dm.width, err = _readNext(f)
	if err != nil {
		return dm, err
	}

	if dm.width == 6363110499870197078 { // magic number for VERSIONX
		return readDepthMapFormat2(f)
	}

	dm.height, err = _readNext(f)
	if err != nil {
		return dm, err
	}

	if dm.width <= 0 || dm.width >= 100000 || dm.height <= 0 || dm.height >= 100000 {
		return dm, fmt.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
	}

	dm.data = make([][]int, dm.width)

	for x := 0; x < dm.width; x++ {
		dm.data[x] = make([]int, dm.height)
		for y := 0; y < dm.height; y++ {
			dm.data[x][y], err = _readNext(f)
			if err != nil {
				return dm, err
			}
		}
	}

	return dm, nil
}

func readDepthMapFormat2(r *bufio.Reader) (DepthMap, error) {
	dm := DepthMap{}

	// get past garbade
	_, err := r.ReadString('\n')
	if err != nil {
		return dm, err
	}

	bytesPerPixelString, err := r.ReadString('\n')
	if err != nil {
		return dm, err
	}
	bytesPerPixelString = strings.TrimSpace(bytesPerPixelString)

	if bytesPerPixelString != "2" {
		return dm, fmt.Errorf("i only know how to handle 2 bytes per pixel in new format, not %s", bytesPerPixelString)
	}

	unitsString, err := r.ReadString('\n')
	if err != nil {
		return dm, err
	}
	unitsString = strings.TrimSpace(unitsString)
	units, err := strconv.ParseFloat(unitsString, 64)
	if err != nil {
		return dm, err
	}
	units = units * 1000 // m to mm

	widthString, err := r.ReadString('\n')
	if err != nil {
		return dm, err
	}
	widthString = strings.TrimSpace(widthString)
	x, err := strconv.ParseInt(widthString, 10, 64)
	dm.width = int(x)
	if err != nil {
		return dm, err
	}

	heightString, err := r.ReadString('\n')
	if err != nil {
		return dm, err
	}
	heightString = strings.TrimSpace(heightString)
	x, err = strconv.ParseInt(heightString, 10, 64)
	dm.height = int(x)
	if err != nil {
		return dm, err
	}

	if dm.width <= 0 || dm.width >= 100000 || dm.height <= 0 || dm.height >= 100000 {
		return dm, fmt.Errorf("bad width or height for depth map %v %v", dm.width, dm.height)
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
				return dm, fmt.Errorf("didn't read 2 bytes, got: %d err: %s x,y: %d,%x", n, err, x, y)
			}

			dm.data[x][y] = int(units * float64(binary.LittleEndian.Uint16(temp)))

		}
	}

	return dm, nil
}

func NewDepthMapFromMat(mat gocv.Mat) DepthMap {
	dm := DepthMap{}

	dm.width = mat.Cols()
	dm.height = mat.Rows()

	dm.data = make([][]int, dm.width)

	raw, err := mat.DataPtrFloat64()
	if err != nil {
		panic(err)
	}

	if len(raw) != dm.width*dm.height {
		panic("wtf")
	}

	for x := 0; x < dm.width; x++ {
		dm.data[x] = make([]int, dm.height)
		for y := 0; y < dm.height; y++ {
			dm.data[x][y] = int(raw[y*dm.width+x])
		}
	}

	return dm
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
