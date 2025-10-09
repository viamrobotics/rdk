package fake

import (
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/pointcloud"
)

func (worldState *WorldStateStore) generateTerrain(x, y int, timeOffset float64) float64 {
	var total float64
	var maxAmplitude float64

	frequency := 0.005
	amplitude := 1.0
	persistence := 0.5
	octaves := 4

	for i := 0; i < octaves; i++ {
		noiseValue := worldState.noise.Noise3D(
			(float64(x)+0.5)*frequency,
			(float64(y)+0.5)*frequency,
			timeOffset*frequency,
		)

		total += noiseValue * amplitude
		maxAmplitude += amplitude

		frequency *= 2.0
		amplitude *= persistence
	}

	return total
}

func (worldState *WorldStateStore) generatePointCloud(width, height int) pointcloud.PointCloud {
	pointCloud := pointcloud.NewBasicPointCloud(width * height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			xPos := float64(x)
			yPos := float64(y)
			heightMM := worldState.generateTerrain(x, y, 0)
			point := r3.Vector{
				X: xPos * worldState.spacing,
				Y: yPos * worldState.spacing,
				Z: heightMM * worldState.spacing * 10, // make the height more dramatic
			}

			heightColor := heightToColor(heightMM)
			packedColor := packColor(heightColor)
			colorData := pointcloud.NewValueData(int(math.Float32bits(packedColor)))

			if err := pointCloud.Set(point, colorData); err != nil {
				worldState.logger.Warnf("failed to set point", "error", err)
				continue
			}
		}
	}

	return pointCloud
}

func (worldState *WorldStateStore) updatePointHeight(x, y int, currentHeight float64, timeOffset float64) float64 {
	animHeightMM := worldState.generateTerrain(x, y, timeOffset)
	targetHeightMM := currentHeight*0.8 + animHeightMM*0.2
	newHeightMM := math.Max(-1, math.Min(1, targetHeightMM))
	return newHeightMM
}

func heightToColor(heightMM float64) color.NRGBA {
	var r, g, b uint8

	if heightMM < -0.5 {
		// Deep Blue
		r = 29
		g = 130
		b = 201
	} else if heightMM < 0 {
		// Blue
		r = 33
		g = 136
		b = 217
	} else if heightMM < 0.05 {
		// Beige
		r = 221
		g = 186
		b = 152
	} else if heightMM < 0.15 {
		// Green
		r = 76
		g = 186
		b = 80
	} else if heightMM < 0.4 {
		// Dark Green
		r = 70
		g = 178
		b = 74
	} else if heightMM < 0.65 {
		// Brown
		r = 111
		g = 105
		b = 96
	} else if heightMM < 0.75 {
		// Dark Brown
		r = 100
		g = 90
		b = 84
	} else if heightMM < 9 {
		// Grey
		r = 220
		g = 220
		b = 220
	} else {
		// White
		r = 255
		g = 255
		b = 255
	}

	return color.NRGBA{R: r, G: g, B: b, A: 255}
}
