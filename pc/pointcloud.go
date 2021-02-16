package pc

type Vec3 struct {
	X, Y, Z int
}

type key Vec3

type PointCloud struct {
	points   map[key]Point
	hasColor bool
}

func NewPointCloud() *PointCloud {
	return &PointCloud{points: map[key]Point{}}
}

func (pc *PointCloud) Size() int {
	return len(pc.points)
}

func (pc *PointCloud) At(x, y, z int) Point {
	return pc.points[key{x, y, z}]
}

func (pc *PointCloud) Set(p Point) {
	pc.points[key(p.Position())] = p
	if _, ok := p.(ColoredPoint); ok {
		pc.hasColor = true
	}
}

func (pc *PointCloud) Unset(x, y, z int) {
	delete(pc.points, key{x, y, z})
}

func (pc *PointCloud) Iterate(fn func(p Point) bool) {
	for _, p := range pc.points {
		if cont := fn(p); !cont {
			return
		}
	}
}
