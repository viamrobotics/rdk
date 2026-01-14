package fakePointCloud

import (
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/pointcloud"
)

var (
	XYZRGB_FIELDS = []string{"x", "y", "z", "rgb"}
	XYZRGB_SIZES  = []uint32{4, 4, 4, 4}
	XYZRGB_TYPES  = []commonpb.PointCloudDataType{
		commonpb.PointCloudDataType_POINT_CLOUD_DATA_TYPE_FLOAT,
		commonpb.PointCloudDataType_POINT_CLOUD_DATA_TYPE_FLOAT,
		commonpb.PointCloudDataType_POINT_CLOUD_DATA_TYPE_FLOAT,
		commonpb.PointCloudDataType_POINT_CLOUD_DATA_TYPE_FLOAT,
	}
	XYZRGB_COUNTS = []uint32{1, 1, 1, 1}
)

type PointCloudGenerator struct {
	noise   *Perlin
	spacing float64
}

func (generator *PointCloudGenerator) GenerateTerrain(x, y int, timeOffset float64) float64 {
	var total float64
	var maxAmplitude float64

	frequency := 0.005
	amplitude := 1.0
	persistence := 0.5
	octaves := 4

	for i := 0; i < octaves; i++ {
		noiseValue := generator.noise.Noise3D(
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

func (generator *PointCloudGenerator) GeneratePointCloud(width, height int) (pointcloud.PointCloud, error) {
	pointCloud := pointcloud.NewBasicPointCloud(width * height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			xPos := float64(x)
			yPos := float64(y)
			heightMM := generator.GenerateTerrain(x, y, 0)
			point := r3.Vector{
				X: xPos * generator.spacing,
				Y: yPos * generator.spacing,
				Z: heightMM * generator.spacing * 10, // make the height more dramatic
			}

			heightColor := heightToColor(heightMM)
			packedColor := packColor(heightColor)
			colorData := pointcloud.NewValueData(int(math.Float32bits(packedColor)))

			if err := pointCloud.Set(point, colorData); err != nil {
				return nil, err
			}
		}
	}

	return pointCloud, nil
}

func (generator *PointCloudGenerator) updatePointHeight(x, y int, currentHeight float64, timeOffset float64) float64 {
	animHeightMM := generator.GenerateTerrain(x, y, timeOffset)
	targetHeightMM := currentHeight*0.8 + animHeightMM*0.2
	newHeightMM := math.Max(-1, math.Min(1, targetHeightMM))
	return newHeightMM
}

func packColor(colorValue color.NRGBA) float32 {
	rgb := (uint32(colorValue.R) << 16) | (uint32(colorValue.G) << 8) | uint32(colorValue.B)
	return math.Float32frombits(rgb)
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

func getStride() int {
	stride := 0
	for i := range XYZRGB_SIZES {
		stride += int(XYZRGB_SIZES[i] * XYZRGB_COUNTS[i])
	}
	return stride
}

func buildUpdateHeader(start uint32) *commonpb.PointCloudHeader {
	header := &commonpb.PointCloudHeader{
		Fields: XYZRGB_FIELDS,
		Size:   XYZRGB_SIZES,
		Type:   XYZRGB_TYPES,
		Count:  XYZRGB_COUNTS,
		Start:  start, // Buffer offset for partial update
	}

	return header
}
