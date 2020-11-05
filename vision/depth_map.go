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

func (dm *DepthMap) ToMat() gocv.Mat {
	m := gocv.NewMatWithSize(dm.height, dm.width, gocv.MatTypeCV64F)
	for x := 0; x < dm.width; x++ {
		for y := 0; y < dm.height; y++ {
			m.SetDoubleAt(y, x, float64(dm.data[x][y]))
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

	return dm, nil
}
