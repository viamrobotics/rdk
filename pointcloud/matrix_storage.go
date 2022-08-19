package pointcloud

import "github.com/golang/geo/r3"

type matrixStorage struct {
	points   []PointAndData
	indexMap map[r3.Vector]uint // TODO (aidanglickman): when r3.Vector has a hash method update this to save space
}

func (ms *matrixStorage) Size() int {
	return len(ms.points)
}

func (ms *matrixStorage) Set(v r3.Vector, d Data) error {
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	if i, found := ms.indexMap[v]; found {
		ms.points[i].D = d
	} else {
		ms.points = append(ms.points, PointAndData{v, d})
		ms.indexMap[v] = uint(len(ms.points) - 1) // TODO (aidanglickman): update this to save space
	}
	return nil
}

func (ms *matrixStorage) At(x, y, z float64) (Data, bool) {
	// TODO (aidanglickman): Update this whole function with the new hashing
	v := r3.Vector{x, y, z}
	if i, found := ms.indexMap[v]; found {
		return ms.points[i].D, true
	}
	return nil, false
}

func (ms *matrixStorage) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	lowerBound := 0
	upperBound := ms.Size()
	if numBatches > 0 {
		batchSize := (ms.Size() + numBatches - 1) / numBatches
		lowerBound = myBatch * batchSize
		upperBound = (myBatch + 1) * batchSize
	}
	if upperBound > ms.Size() {
		upperBound = ms.Size()
	}
	for i := lowerBound; i < upperBound; i++ {
		if cont := fn(ms.points[i].P, ms.points[i].D); !cont {
			return
		}
	}
}

func (ms *matrixStorage) EditSupported() bool {
	return true
}

func (ms *matrixStorage) IsOrdered() bool {
	return true
}
