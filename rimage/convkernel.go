package rimage

import (
	"errors"
	"image"
	"math"
)

// Matrix interface for the Kernel.
type Matrix interface {
	At(x, y int) float64
}

// Kernel is a 2 dimensional matrix used mainly for convolution.
type Kernel struct {
	Content [][]float64
	Width   int
	Height  int
}

// NewKernel creates a new Kernel with the given width and height. The value for every position of the kernel is 0.
func NewKernel(width, height int) (*Kernel, error) {
	if width < 0 || height < 0 {
		return nil, errors.New("negative kernel size")
	}
	m := make([][]float64, height)
	for i := range m {
		m[i] = make([]float64, width)
	}
	return &Kernel{Content: m, Width: width, Height: height}, nil
}

// At returns a value from the position of {x, y} of a kernel.
func (k *Kernel) At(x, y int) float64 {
	return k.Content[x][y]
}

// Set sets a value at a given {x, y} position.
func (k *Kernel) Set(x, y int, value float64) {
	k.Content[x][y] = value
}

// Size returns the size of the kernel. The size is a type of image.Point containing the width and height of the kernel.
func (k *Kernel) Size() image.Point {
	return image.Point{X: k.Width, Y: k.Height}
}

// AbSum returns the sum of every absolute value from a kernel.
func (k *Kernel) AbSum() float64 {
	var sum float64
	for x := 0; x < k.Height; x++ {
		for y := 0; y < k.Width; y++ {
			sum += math.Abs(k.At(x, y))
		}
	}
	return sum
}

// Normalize returns a normalized kernel where each value is divided by the absolute sum of the kernel.
func (k *Kernel) Normalize() *Kernel {
	m := make([][]float64, k.Height)
	for i := range m {
		m[i] = make([]float64, k.Width)
	}
	normalized := &Kernel{m, k.Width, k.Height}
	sum := k.AbSum()
	if sum == 0 {
		sum = 1
	}
	for x := 0; x < k.Height; x++ {
		for y := 0; y < k.Width; y++ {
			normalized.Set(x, y, k.At(x, y)/sum)
		}
	}
	return normalized
}
