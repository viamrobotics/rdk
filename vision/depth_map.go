package vision

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gocv.io/x/gocv"
)

type DepthMap struct {
	width  int
	height int

	data [][]int
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
	Walk(centerX, centerY, myMax(dm.width, dm.height), func(x, y int) error {
		dm._getDepthOrEstimate(x, y)
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
	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			z := dm._getDepthOrEstimate(x, y)
			m.SetDoubleAt(y, x, float64(z))
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
