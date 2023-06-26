package pointcloud

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/golang/geo/r3"
)

// visualizeOctree is a debugging tool that creates an image of size resolution x resolution for a given octree.
//
//nolint:unused
func visualizeOctree(octree *BasicOctree, resolution int) *image.RGBA {
	pixelScalar := float64(resolution) / octree.sideLength
	pixelOffset := r3.Vector{
		X: octree.center.X - octree.sideLength/2.,
		Y: octree.center.Y - octree.sideLength/2.,
	}

	// Create base image with a white background
	img := image.NewRGBA(image.Rect(0, 0, resolution, resolution))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	// Recursively visualize nodes
	helperVisualizeOctree(octree, img, pixelOffset, pixelScalar)

	return img
}

// Recursively iterates through octree, outlining internal nodes and coloring filled nodes based on probability value.
//
//nolint:unused
func helperVisualizeOctree(octree *BasicOctree, img *image.RGBA, pixelOffset r3.Vector, pixelScalar float64) {
	switch octree.node.nodeType {
	case internalNode:
		for _, childNode := range octree.node.children {
			pixelX := int((childNode.center.X - childNode.sideLength/2. - pixelOffset.X) * pixelScalar)
			pixelY := int((childNode.center.Y - childNode.sideLength/2. - pixelOffset.Y) * pixelScalar)
			pixelSide := int(childNode.sideLength * pixelScalar)
			drawSquareOutline(img, pixelX, pixelY, pixelSide, color.RGBA{0, 0, 0, 255})

			helperVisualizeOctree(childNode, img, pixelOffset, pixelScalar)
		}

	case leafNodeFilled:
		pixelX := int((octree.center.X - pixelOffset.X - octree.sideLength/2.) * pixelScalar)
		pixelY := int((octree.center.Y - pixelOffset.Y - octree.sideLength/2.) * pixelScalar)
		pixelSide := int(octree.sideLength * pixelScalar)

		fillSquare(img, pixelX+1, pixelY+1, pixelSide-1, color.RGBA{0, uint8(255.0 * octree.node.maxVal / 100), 0, 255})

		// Visualize points
		pointColor := color.RGBA{255, 0, 0, 255}
		img.Set(int((octree.node.point.P.X-pixelOffset.X)*pixelScalar), int((octree.node.point.P.Y-pixelOffset.Y)*pixelScalar), pointColor)

	case leafNodeEmpty:
	}
}

// Draw a outline of a square defined by a center and side length using the given color.
//
//nolint:unused
func drawSquareOutline(img *image.RGBA, x, y, side int, c color.RGBA) {
	for i := 0; i <= side; i++ {
		img.Set(x, y+i, c)
		img.Set(x+i, y, c)
		img.Set(x+side, y+i, c)
		img.Set(x+i, y+side, c)
	}
}

// Fill a square defined by a center and side length using the given color.
//
//nolint:unused
func fillSquare(img *image.RGBA, x, y, side int, c color.RGBA) {
	for i := 0; i <= side; i++ {
		for j := 0; j <= side; j++ {
			img.Set(x+i, y+j, c)
		}
	}
}
