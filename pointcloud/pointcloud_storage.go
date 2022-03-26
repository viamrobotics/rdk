package pointcloud

type storage interface {
	Size() int
	Set(p Vec3, d Data) error
	Unset(x, y, z float64)
	At(x, y, z float64) (Data, bool)
	Iterate(numBatches, myBatch int, fn func(p Vec3, d Data) bool)
	EditSupported() bool
}

// key is the map key used within the point cloud implementation. That is
// we index points by their positions. This is problematic for points that
// can mutate their own position outside the ownership of the cloud.
type key Vec3

type mapStorage struct {
	points map[key]Data
}

func (ms *mapStorage) Size() int {
	return len(ms.points)
}

// With 64-bit floating point numbers, you get about 16 decimal digits of precision.
// To guarantee at least 6 decimal places of precision past 0, Abs(x) cannot be greater than 2^33 - 1.
const (
	maxPreciseFloat64 = float64(8589934591)
	minPreciseFloat64 = float64(-8589934591)
)

// newOutOfRangeErr returns an error informing that a value is numerically out of range to
// be stored precisely.
func newOutOfRangeErr(dim string, val float64) error {
	return errors.Errorf("%s component (%v) is out of range [%v,%v]", dim, val, minPreciseFloat64, maxPreciseFloat64)
}


func (ms *mapStorage) Set(p Vec3, d Data) error {
	v := p.Position()
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	ms.points[p] = d
}

func (ms *mapStorage) Unset(x, y, z float64) {
	delete(ms.points, key{x, y, z})
}

func (ms *mapStorage) At(x, y, z float64) (Data, bool) {
	return ms.points[key{x, y, z}]
}

func (ms *mapStorage) Iterate(numBatches, myBatch int, fn func(p Vec3, d Data) bool) {
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

