package fake

import (
	"bytes"
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/pointcloud"
	pc "go.viam.com/rdk/pointcloud"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	pointcloudUUID = "pointcloud-001"
)

type PointCloudWorld struct {
	noise           *Perlin
	spacing         float64
	worldStateStore *WorldStateStore
}

func (w *PointCloudWorld) StartWorld() {
	w.worldStateStore.mu.Lock()
	defer w.worldStateStore.mu.Unlock()

	pointCloud, err := w.generatePointCloud(100, 100)
	if err != nil {
		w.worldStateStore.logger.Errorf("failed to generate point cloud: %v", err)
		return
	}

	var pcdBytes bytes.Buffer
	err = pc.ToPCD(pointCloud, &pcdBytes, pointcloud.PCDBinary)
	if err != nil {
		w.worldStateStore.logger.Errorf("failed to convert point cloud to pcd: %v", err)
		return
	}
	w.worldStateStore.logger.Infof("generating point cloud %v", pcdBytes.Len())

	w.worldStateStore.transforms[pointcloudUUID] = &commonpb.Transform{
		ReferenceFrame: "static-pointcloud",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 0, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Pointcloud{
				Pointcloud: &commonpb.PointCloud{
					PointCloud: pcdBytes.Bytes(),
				},
			},
		},
		Uuid:     []byte(pointcloudUUID),
		Metadata: &structpb.Struct{},
	}
}

func (generator *PointCloudWorld) generateTerrain(x, y int, timeOffset float64) float64 {
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

func (generator *PointCloudWorld) generatePointCloud(width, height int) (pointcloud.PointCloud, error) {
	pointCloud := pointcloud.NewBasicPointCloudWithMetaData(width*height, pointcloud.MetaData{HasColor: true})
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			xPos := float64(x)
			yPos := float64(y)
			heightMM := generator.generateTerrain(x, y, 0)
			point := r3.Vector{
				X: xPos * generator.spacing,
				Y: yPos * generator.spacing,
				Z: heightMM * generator.spacing * 10, // make the height more dramatic
			}

			heightColor := heightToColor(heightMM)
			colorData := pointcloud.NewColoredData(heightColor)

			if err := pointCloud.Set(point, colorData); err != nil {
				return nil, err
			}
		}
	}

	return pointCloud, nil
}

func (generator *PointCloudWorld) updatePointHeight(x, y int, currentHeight float64, timeOffset float64) float64 {
	animHeightMM := generator.generateTerrain(x, y, timeOffset)
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
