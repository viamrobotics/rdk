package vision

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"

	"gocv.io/x/gocv"
)

type DepthMap struct {
	width  int
	height int

	min int
	max int

	data [][]int
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

func (dm *DepthMap) smooth() {
	centerX := dm.width / 2
	centerY := dm.height / 2
	dm.max = 0
	dm.min = 100000
	Walk(centerX, centerY, myMax(dm.width, dm.height), func(x, y int) error {
		temp := dm._getDepthOrEstimate(x, y)
		if temp > 0 && temp < dm.min {
			dm.min = temp
		}
		if temp > dm.max {
			dm.max = temp
		}
		return nil
	})
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

	return ReadDepthMap(f)
}

func ReadDepthMap(f io.Reader) (DepthMap, error) {
	var err error
	dm := DepthMap{}

	dm.width, err = _readNext(f)
	if err != nil {
		return dm, err
	}

	dm.height, err = _readNext(f)
	if err != nil {
		return dm, err
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

	dm.smooth()

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

	var gout *gzip.Writer = nil
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

	f.Sync()
	return nil
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
