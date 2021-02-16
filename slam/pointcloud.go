package slam

import (
	"image/color"
)

type Vec3 struct {
	X, Y, Z int
}

type key Vec3

type PointCloud struct {
	points map[key]Point
}

type Point interface {
	Position() Vec3
	Color() *color.RGBA
}

type basicPoint Vec3

func (bp basicPoint) Position() Vec3 {
	return Vec3(bp)
}

func (bp basicPoint) Color() *color.RGBA {
	return nil
}

func NewPoint(x, y, z int) Point {
	return basicPoint{x, y, z}
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
