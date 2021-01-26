package support

import (
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"math/rand"

	"github.com/echolabsinc/robotcore/lidar"
)

type FakeLidar struct {
	posX, posY int
	started    bool
	Seed       int64
}

func (fl *FakeLidar) Start() {
	fl.started = true
}

func (fl *FakeLidar) Stop() {
	fl.started = false
}

func (fl *FakeLidar) Close() {

}

func (fl *FakeLidar) Scan() (lidar.Measurements, error) {
	if !fl.started {
		return nil, nil
	}
	h := fnv.New64()
	if _, err := h.Write([]byte(fmt.Sprintf("%d,%d", fl.posX, fl.posY))); err != nil {
		return nil, err
	}
	r := rand.NewSource(int64(h.Sum64()) + fl.Seed)
	measurements := make(lidar.Measurements, 0, 360)
	getFloat64 := func() float64 {
	again:
		f := float64(r.Int63()) / (1 << 63)
		if f == 1 {
			goto again // resample
		}
		return f
	}
	for i := 0; i < cap(measurements); i++ {
		measurements = append(measurements, lidar.NewMeasurement(getFloat64()*360*math.Pi/180, getFloat64()*float64(fl.Range())))
	}
	return measurements, nil
}

func (fl *FakeLidar) Range() int {
	return 25
}

func (fl *FakeLidar) Bounds() (image.Point, error) {
	return image.Point{fl.Range() * 2, fl.Range() * 2}, nil
}
