package pointcloud

type storage interface {
	Size() int
	Set(p Point)
	Unset(x, y, z float64)
	At(x, y, z float64) Point
	Iterate(fn func(p Point) bool)
	Points() []Point
	EditSupported() bool
}

// key is the map key used within the point cloud implementation. That is
// we index points by their positions. This is problematic for points that
// can mutate their own position outside the ownership of the cloud.
type key Vec3

type mapStorage struct {
	points map[key]Point
}

func (ms *mapStorage) Size() int {
	return len(ms.points)
}

func (ms *mapStorage) Set(p Point) {
	ms.points[key(p.Position())] = p
}

func (ms *mapStorage) Unset(x, y, z float64) {
	delete(ms.points, key{x, y, z})
}

func (ms *mapStorage) At(x, y, z float64) Point {
	return ms.points[key{x, y, z}]
}

func (ms *mapStorage) Iterate(fn func(p Point) bool) {
	for _, p := range ms.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

func (ms *mapStorage) Points() []Point {
	pts := make([]Point, 0, ms.Size())
	for _, v := range ms.points {
		pts = append(pts, v)
	}
	return pts
}

func (ms *mapStorage) EditSupported() bool {
	return true
}

// ----

type arrayStorage struct {
	points []Point
}

func (as *arrayStorage) Size() int {
	return len(as.points)
}

func (as *arrayStorage) Set(p Point) {
	as.points = append(as.points, p)
}

func (as *arrayStorage) Unset(x, y, z float64) {
	panic("Unset not supported in arrayStorage")
}

func (as *arrayStorage) At(x, y, z float64) Point {
	panic("At not supported in arrayStorage")
}

func (as *arrayStorage) Iterate(fn func(p Point) bool) {
	for _, p := range as.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

func (as *arrayStorage) Points() []Point {
	return as.points
}

func (ms *arrayStorage) EditSupported() bool {
	return false
}

func convertToMapStorage(s storage) *mapStorage {
	ms, ok := s.(*mapStorage)
	if ok {
		return ms
	}

	ms = &mapStorage{make(map[key]Point, s.Size())}

	s.Iterate(func(p Point) bool {
		ms.Set(p)
		return true
	})

	return ms
}
