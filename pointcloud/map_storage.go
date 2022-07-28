package pointcloud

import "github.com/golang/geo/r3"

type mapStorage struct {
	points map[r3.Vector]Data
}

func (ms *mapStorage) Size() int {
	return len(ms.points)
}

func (ms *mapStorage) Set(v r3.Vector, d Data) error {
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	ms.points[v] = d
	return nil
}

func (ms *mapStorage) At(x, y, z float64) (Data, bool) {
	d, found := ms.points[r3.Vector{x, y, z}]
	return d, found
}

func (ms *mapStorage) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	if numBatches > 0 && myBatch > 0 {
		// TODO(erh) finish me
		return
	}
	for p, d := range ms.points {
		if cont := fn(p, d); !cont {
			return
		}
	}
}

func (ms *mapStorage) EditSupported() bool {
	return true
}

func (ms *mapStorage) IsOrdered() bool {
	return false
}
