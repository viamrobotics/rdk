// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fake

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// WorldStateStore implements the worldstatestore.Service interface.
type WorldStateStore struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	mu sync.RWMutex

	transforms map[string]*commonpb.Transform
	fps        float64

	startTime               time.Time
	activeBackgroundWorkers sync.WaitGroup

	changeChan chan worldstatestore.TransformChange
	streamCtx  context.Context
	cancel     context.CancelFunc

	logger logging.Logger

	noiseTime float64
	minZ      float64
	maxZ      float64
}

const (
	// GPUBufferAlignment ensures optimal GPU memory transfer (aligned to 256 bytes)
	GPUBufferAlignment = 256

	// MaxFPS is the maximum supported frame rate (matches high-end monitors)
	MaxFPS = 144.0

	// MinFPS is the minimum non-zero frame rate
	MinFPS = 0.1

	// TargetBandwidthMBps is the target bandwidth in MB/sec for point cloud updates
	// This balances network efficiency with real-time performance
	TargetBandwidthMBps = 8.0

	// MinChunkSize is the minimum chunk size to avoid excessive overhead
	MinChunkSize = 1000

	// MaxChunkSize is the maximum chunk size to keep messages under ~1MB
	MaxChunkSize = 65000
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

var (
	ErrTransformNotFound = errors.New("transform not found")
	ErrInvalidFPS        = errors.New("fps must be greater than 0")
)

var (
	boxUUID        = "box-001"
	sphereUUID     = "sphere-001"
	capsuleUUID    = "capsule-001"
	pointcloudUUID = "pointcloud-001"

	boxMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.5,
				},
			},
		},
	}

	sphereMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.7,
				},
			},
		},
	}

	capsuleMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 1.0,
				},
			},
		},
	}

	dynamicBoxMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.3,
				},
			},
		},
	}

	permutations = []int{
		151, 160, 137, 91, 90, 15, 131, 13, 201, 95, 96, 53, 194, 233, 7, 225, 140,
		36, 103, 30, 69, 142, 8, 99, 37, 240, 21, 10, 23, 190, 6, 148, 247, 120,
		234, 75, 0, 26, 197, 62, 94, 252, 219, 203, 117, 35, 11, 32, 57, 177, 33,
		88, 237, 149, 56, 87, 174, 20, 125, 136, 171, 168, 68, 175, 74, 165, 71,
		134, 139, 48, 27, 166, 77, 146, 158, 231, 83, 111, 229, 122, 60, 211, 133,
		230, 220, 105, 92, 41, 55, 46, 245, 40, 244, 102, 143, 54, 65, 25, 63, 161,
		1, 216, 80, 73, 209, 76, 132, 187, 208, 89, 18, 169, 200, 196, 135, 130,
		116, 188, 159, 86, 164, 100, 109, 198, 173, 186, 3, 64, 52, 217, 226, 250,
		124, 123, 5, 202, 38, 147, 118, 126, 255, 82, 85, 212, 207, 206, 59, 227, 47,
		16, 58, 17, 182, 189, 28, 42, 223, 183, 170, 213, 119, 248, 152, 2, 44, 154,
		163, 70, 221, 153, 101, 155, 167, 43, 172, 9, 129, 22, 39, 253, 19, 98, 108,
		110, 79, 113, 224, 232, 178, 185, 112, 104, 218, 246, 97, 228, 251, 34, 242,
		193, 238, 210, 144, 12, 191, 179, 162, 241, 81, 51, 145, 235, 249, 14, 239,
		107, 49, 192, 214, 31, 181, 199, 106, 157, 184, 84, 204, 176, 115, 121, 50,
		45, 127, 4, 150, 254, 138, 236, 205, 93, 222, 114, 67, 29, 24, 72, 243, 141,
		128, 195, 78, 66, 215, 61, 156, 180,
	}

	permutationCache [512]int
)

func init() {
	copy(permutationCache[:256], permutations)
	copy(permutationCache[256:], permutations)

	resource.RegisterService(
		worldstatestore.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[worldstatestore.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (worldstatestore.Service, error) {
			return newFakeWorldStateStore(conf.ResourceName(), logger), nil
		}})
}

// ListUUIDs returns all transform UUIDs currently in the store.
func (worldState *WorldStateStore) ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error) {
	worldState.mu.RLock()
	defer worldState.mu.RUnlock()

	uuids := make([][]byte, 0, len(worldState.transforms))
	for _, transform := range worldState.transforms {
		uuids = append(uuids, transform.Uuid)
	}

	return uuids, nil
}

// GetTransform returns the transform for the given UUID.
func (worldState *WorldStateStore) GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
	worldState.mu.RLock()
	defer worldState.mu.RUnlock()

	uuidStr := string(uuid)
	transform, exists := worldState.transforms[uuidStr]
	if !exists {
		return nil, ErrTransformNotFound
	}

	return transform, nil
}

// StreamTransformChanges returns a channel of transform changes.
func (worldState *WorldStateStore) StreamTransformChanges(
	ctx context.Context,
	extra map[string]any,
) (*worldstatestore.TransformChangeStream, error) {
	return worldstatestore.NewTransformChangeStreamFromChannel(ctx, worldState.changeChan), nil
}

// DoCommand handles arbitrary commands. Accepts:
//
// - "fps": float64 to set the animation rate (0 to pause, max 144)
func (worldState *WorldStateStore) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if fps, ok := cmd["fps"].(float64); ok {
		originalFPS := fps

		if fps < 0 {
			worldState.logger.Warnf("FPS value %.2f is below 0, clamping to 0 (paused)", fps)
			fps = 0
		} else if fps > 0 && fps < MinFPS {
			worldState.logger.Warnf("FPS value %.2f is below minimum %.2f, clamping to %.2f", fps, MinFPS, MinFPS)
			fps = MinFPS
		} else if fps > MaxFPS {
			worldState.logger.Warnf("FPS value %.2f exceeds maximum %.2f, clamping to %.2f", fps, MaxFPS, MaxFPS)
			fps = MaxFPS
		}

		worldState.mu.Lock()
		worldState.fps = fps
		worldState.mu.Unlock()

		statusMsg := fmt.Sprintf("fps set to %.2f", fps)
		if fps != originalFPS {
			statusMsg += fmt.Sprintf(" (clamped from %.2f)", originalFPS)
		}

		return map[string]any{
			"status": statusMsg,
		}, nil
	}

	return map[string]any{
		"status": "command not implemented",
	}, nil
}

// Close stops the fake service and cleans up resources.
func (worldState *WorldStateStore) Close(ctx context.Context) error {
	worldState.cancel()

	done := make(chan struct{})
	go func() {
		worldState.activeBackgroundWorkers.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// proceed even if workers did not exit in time
	}

	close(worldState.changeChan)
	return nil
}

func newFakeWorldStateStore(name resource.Name, logger logging.Logger) worldstatestore.Service {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &WorldStateStore{
		Named:                   name.AsNamed(),
		TriviallyReconfigurable: resource.TriviallyReconfigurable{},
		TriviallyCloseable:      resource.TriviallyCloseable{},
		transforms:              make(map[string]*commonpb.Transform),
		fps:                     10,
		startTime:               time.Now(),
		changeChan:              make(chan worldstatestore.TransformChange, 100),
		streamCtx:               ctx,
		cancel:                  cancel,
		logger:                  logger,
	}

	fake.initializeTransforms()
	fake.activeBackgroundWorkers.Add(1)
	go func() {
		defer fake.activeBackgroundWorkers.Done()
		fake.animate()
	}()
	fake.activeBackgroundWorkers.Add(1)
	go func() {
		defer fake.activeBackgroundWorkers.Done()
		fake.boxSequence()
	}()

	return fake
}

func getStride() int {
	stride := 0
	for i := range XYZRGB_SIZES {
		stride += int(XYZRGB_SIZES[i] * XYZRGB_COUNTS[i])
	}
	return stride
}

func heightToColor(normalizedHeight float64) color.NRGBA {
	height := math.Max(0, math.Min(1, normalizedHeight))

	var r, g, b uint8
	if height < 0.25 {
		// Blue to Cyan
		transition := height / 0.25
		r = 0
		g = uint8(transition * 255)
		b = 255
	} else if height < 0.5 {
		// Cyan to Green
		transition := (height - 0.25) / 0.25
		r = 0
		g = 255
		b = uint8((1 - transition) * 255)
	} else if height < 0.75 {
		// Green to Yellow
		transition := (height - 0.5) / 0.25
		r = uint8(transition * 255)
		g = 255
		b = 0
	} else {
		// Yellow to Red
		transition := (height - 0.75) / 0.25
		r = 255
		g = uint8((1 - transition) * 255)
		b = 0
	}

	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

func packColor(colorValue color.NRGBA) float32 {
	rgb := (uint32(colorValue.R) << 16) | (uint32(colorValue.G) << 8) | uint32(colorValue.B)
	return math.Float32frombits(rgb)
}

func fade(interpolation float64) float64 {
	return interpolation * interpolation * interpolation * (interpolation*(interpolation*6-15) + 10)
}

func lerp(interpolation, start, end float64) float64 {
	return start + interpolation*(end-start)
}

func gradient(hash int, x, y, z float64) float64 {
	hashValue := hash & 15
	gradientU := x
	if hashValue < 8 {
		gradientU = y
	}

	gradientV := y
	if hashValue < 4 {
		gradientV = x
	} else if hashValue == 12 || hashValue == 14 {
		gradientV = x
	} else {
		gradientV = z
	}

	if (hashValue & 1) == 0 {
		gradientU = -gradientU
	}

	if (hashValue & 2) == 0 {
		gradientV = -gradientV
	}

	return gradientU + gradientV
}

func fastFloor(x float64) int {
	if x >= 0 {
		return int(x)
	}
	return int(x) - 1
}

func noise(x, y, z float64) float64 {
	X := fastFloor(x) & 255
	Y := fastFloor(y) & 255
	Z := fastFloor(z) & 255

	x -= math.Floor(x)
	y -= math.Floor(y)
	z -= math.Floor(z)

	fadeX := fade(x)
	fadeY := fade(y)
	fadeZ := fade(z)

	A := permutationCache[X] + Y
	AA := permutationCache[A&255] + Z
	AB := permutationCache[(A+1)&255] + Z
	B := permutationCache[(X+1)&255] + Y
	BA := permutationCache[B&255] + Z
	BB := permutationCache[(B+1)&255] + Z

	return lerp(
		fadeZ,
		lerp(
			fadeY,
			lerp(
				fadeX,
				gradient(permutationCache[AA&255], x, y, z),
				gradient(permutationCache[BA&255], x-1, y, z),
			),
			lerp(
				fadeX,
				gradient(permutationCache[AB&255], x, y-1, z),
				gradient(permutationCache[BB&255], x-1, y-1, z),
			),
		),
		lerp(
			fadeY,
			lerp(
				fadeX,
				gradient(permutationCache[(AA+1)&255], x, y, z-1),
				gradient(permutationCache[(BA+1)&255], x-1, y, z-1),
			),
			lerp(
				fadeX,
				gradient(permutationCache[(AB+1)&255], x, y-1, z-1),
				gradient(permutationCache[(BB+1)&255], x-1, y-1, z-1),
			),
		),
	)
}

func sampleBounds(values []float64, lowPerc, highPerc float64) (float64, float64) {
	count := len(values)
	if count == 0 {
		return 0, 0
	}

	sampleSize := 1000
	if count < sampleSize {
		sampleSize = count
	}

	step := count / sampleSize
	sample := make([]float64, 0, sampleSize)
	for i := 0; i < count; i += step {
		sample = append(sample, values[i])
		if len(sample) >= sampleSize {
			break
		}
	}

	sort.Float64s(sample)
	sampleCount := len(sample)
	lowIdx := int(float64(sampleCount) * lowPerc)
	highIdx := int(float64(sampleCount) * highPerc)
	if highIdx >= sampleCount {
		highIdx = sampleCount - 1
	}

	return sample[lowIdx], sample[highIdx]
}

func buildUpdateHeader(start, count uint32) *commonpb.PointCloudHeader {
	startPtr := &start
	header := &commonpb.PointCloudHeader{
		Fields: append([]string{}, XYZRGB_FIELDS...),
		Size:   append([]uint32{}, XYZRGB_SIZES...),
		Type:   append([]commonpb.PointCloudDataType{}, XYZRGB_TYPES...),
		Count:  append([]uint32{}, XYZRGB_COUNTS...),
		Width:  count, // Number of points in this update
		Height: 1,
		Start:  startPtr, // Buffer offset for partial update
	}

	return header
}

func validateStride(header *commonpb.PointCloudHeader) int {
	if len(header.Size) != len(header.Count) {
		return 0
	}

	stride := 0
	for i := range header.Size {
		stride += int(header.Size[i] * header.Count[i])
	}
	return stride
}

func validatePointCloud(data []byte, header *commonpb.PointCloudHeader) error {
	if header == nil {
		return errors.New("header cannot be nil")
	}

	stride := validateStride(header)
	if stride <= 0 {
		return errors.New("invalid header: stride must be positive")
	}

	expectedSize := stride * int(header.Width)
	if len(data) != expectedSize {
		return fmt.Errorf("binary data size mismatch: expected %d bytes, got %d bytes", expectedSize, len(data))
	}

	return nil
}

func createBuffer(sizeBytes int) []byte {
	alignedSize := ((sizeBytes + GPUBufferAlignment - 1) / GPUBufferAlignment) * GPUBufferAlignment
	return make([]byte, sizeBytes, alignedSize)
}

func (worldState *WorldStateStore) processPointCloud(pc pointcloud.PointCloud) pointcloud.PointCloud {
	size := pc.Size()
	if size == 0 {
		return pc
	}

	zValues := make([]float64, 0, size)

	var sumX, sumY float64
	pc.Iterate(0, 0, func(point r3.Vector, data pointcloud.Data) bool {
		sumX += point.X
		sumY += point.Y
		zValues = append(zValues, point.Z)
		return true
	})

	count := len(zValues)
	if count == 0 {
		return pc
	}

	minZ, maxZ := sampleBounds(zValues, 0.01, 0.99)
	center := r3.Vector{
		X: sumX / float64(count),
		Y: sumY / float64(count),
		Z: minZ,
	}

	zRange := maxZ - minZ
	if zRange == 0 {
		zRange = 1
	}

	worldState.minZ = minZ
	worldState.maxZ = maxZ

	withColors := pointcloud.NewBasicPointCloud(size)
	pc.Iterate(0, 0, func(point r3.Vector, data pointcloud.Data) bool {
		translatedPoint := r3.Vector{X: point.X - center.X, Y: point.Y - center.Y, Z: point.Z - center.Z}
		normalizedHeight := (point.Z - minZ) / zRange
		heightColor := heightToColor(normalizedHeight)
		packedColor := packColor(heightColor)
		colorData := pointcloud.NewValueData(int(math.Float32bits(packedColor)))
		err := withColors.Set(translatedPoint, colorData)
		if err != nil {
			worldState.logger.Debug("failed to set color for point", "error", err)
		}
		return true
	})

	return withColors
}

func writePointCloud(cloud pointcloud.PointCloud, out io.Writer) error {
	size := cloud.Size()
	if size == 0 {
		return nil
	}

	const bufSize = 256 * 1024
	buf := make([]byte, bufSize)
	pos := 0

	var iterErr error
	cloud.Iterate(0, 0, func(position r3.Vector, data pointcloud.Data) bool {
		x := float32(position.X / 1000.)
		y := float32(position.Y / 1000.)
		z := float32(position.Z / 1000.)
		rgb := math.Float32frombits(uint32(data.Value()))
		if pos+16 > len(buf) {
			if _, err := out.Write(buf[:pos]); err != nil {
				iterErr = err
				return false
			}
			pos = 0
		}

		binary.LittleEndian.PutUint32(buf[pos:], math.Float32bits(x))
		binary.LittleEndian.PutUint32(buf[pos+4:], math.Float32bits(y))
		binary.LittleEndian.PutUint32(buf[pos+8:], math.Float32bits(z))
		binary.LittleEndian.PutUint32(buf[pos+12:], math.Float32bits(rgb))
		pos += 16

		return true
	})

	if pos > 0 && iterErr == nil {
		if _, err := out.Write(buf[:pos]); err != nil {
			iterErr = err
		}
	}

	return iterErr
}

func (worldState *WorldStateStore) loadPointCloud() ([]byte, *commonpb.PointCloudHeader, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		worldState.logger.Errorw("failed to get current file path")
		return nil, nil, errors.New("failed to get current file path")
	}

	// From: https://github.com/PointCloudLibrary/data
	pcdPath := filepath.Join(filepath.Dir(filename), "FSite5_orig-utm.pcd")
	file, err := os.Open(pcdPath)
	if err != nil {
		worldState.logger.Errorw("failed to open PCD file", "error", err, "path", pcdPath)
		return nil, nil, err
	}
	defer file.Close()

	pc, err := pointcloud.ReadPCD(file, pointcloud.BasicOctreeType)
	if err != nil {
		worldState.logger.Errorw("failed to read PCD file", "error", err)
		return nil, nil, err
	}

	worldState.logger.Infow("Loaded point cloud", "size", pc.Size())
	withColors := worldState.processPointCloud(pc)
	header := buildUpdateHeader(0, uint32(withColors.Size()))
	buf := &bytes.Buffer{}
	stride := getStride()
	buf.Grow(withColors.Size() * stride)
	if err := writePointCloud(withColors, buf); err != nil {
		worldState.logger.Errorw("failed to write interleaved pointcloud data", "error", err)
		return nil, nil, err
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	if err := validatePointCloud(result, header); err != nil {
		worldState.logger.Errorw("generated point cloud data validation failed", "error", err)
		return nil, nil, err
	}

	return result, header, nil
}

func (worldState *WorldStateStore) initializeTransforms() {
	worldState.mu.Lock()
	defer worldState.mu.Unlock()

	worldState.transforms[boxUUID] = &commonpb.Transform{
		ReferenceFrame: "static-box",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: -5000, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{
						X: 1000,
						Y: 1000,
						Z: 1000,
					},
				},
			},
		},
		Uuid:     []byte(boxUUID),
		Metadata: boxMetadata,
	}

	worldState.transforms[sphereUUID] = &commonpb.Transform{
		ReferenceFrame: "static-sphere",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 0, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{
					RadiusMm: 500,
				},
			},
		},
		Uuid:     []byte(sphereUUID),
		Metadata: sphereMetadata,
	}

	worldState.transforms[capsuleUUID] = &commonpb.Transform{
		ReferenceFrame: "static-capsule",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 5000, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Capsule{
				Capsule: &commonpb.Capsule{
					RadiusMm: 125,
					LengthMm: 1000,
				},
			},
		},
		Uuid:     []byte(capsuleUUID),
		Metadata: capsuleMetadata,
	}

	pointcloudBytes, pointcloudHeader, err := worldState.loadPointCloud()
	if err != nil {
		worldState.logger.Errorw("failed to load point cloud", "error", err)
	} else if pointcloudBytes != nil {
		worldState.transforms[pointcloudUUID] = &commonpb.Transform{
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
						PointCloud: pointcloudBytes,
						Header:     pointcloudHeader,
					},
				},
			},
			Uuid:     []byte(pointcloudUUID),
			Metadata: &structpb.Struct{},
		}
	}
}

func (worldState *WorldStateStore) updateBox(elapsed time.Duration) {
	rotationSpeed := 2 * math.Pi / 5.0
	angle := rotationSpeed * elapsed.Seconds()

	worldState.mu.Lock()
	if transform, exists := worldState.transforms["box-001"]; exists {
		theta := angle * 180 / math.Pi
		transform.PoseInObserverFrame.Pose.Theta = theta
		worldState.mu.Unlock()
		worldState.emitTransformChange(
			&commonpb.Transform{
				Uuid: transform.Uuid,
				PoseInObserverFrame: &commonpb.PoseInFrame{
					Pose: &commonpb.Pose{
						Theta: theta,
					},
				},
			},
			pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			[]string{"poseInObserverFrame.pose.theta"},
		)
		return
	}
	worldState.mu.Unlock()
}

func (worldState *WorldStateStore) updateSphere(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0
	height := math.Sin(frequency*elapsed.Seconds()) * 2000.0 // ±2 units

	worldState.mu.Lock()
	if transform, exists := worldState.transforms["sphere-001"]; exists {
		transform.PoseInObserverFrame.Pose.Z = height
		worldState.mu.Unlock()
		worldState.emitTransformChange(
			&commonpb.Transform{
				Uuid: transform.Uuid,
				PoseInObserverFrame: &commonpb.PoseInFrame{
					Pose: &commonpb.Pose{
						Z: height,
					},
				},
			},
			pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			[]string{"poseInObserverFrame.pose.z"},
		)
		return
	}
	worldState.mu.Unlock()
}

func (worldState *WorldStateStore) updateCapsule(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0
	scale := 1.0 + 0.5*math.Sin(frequency*elapsed.Seconds()) // 0.5x to 1.5x
	radius := 125 * scale
	length := 1000 * scale

	worldState.mu.Lock()
	if transform, exists := worldState.transforms["capsule-001"]; exists {
		transform.PhysicalObject.GetCapsule().RadiusMm = radius
		transform.PhysicalObject.GetCapsule().LengthMm = length
		worldState.mu.Unlock()
		worldState.emitTransformChange(
			&commonpb.Transform{
				Uuid: transform.Uuid,
				PhysicalObject: &commonpb.Geometry{
					GeometryType: &commonpb.Geometry_Capsule{
						Capsule: &commonpb.Capsule{
							RadiusMm: radius,
							LengthMm: length,
						},
					},
				},
			},
			pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			[]string{"physicalObject.geometryType.value.radiusMm", "physicalObject.geometryType.value.lengthMm"},
		)
		return
	}
	worldState.mu.Unlock()
}

// calculateChunkSize returns an FPS-adaptive chunk size that balances bandwidth and latency.
//
// Examples at 16 bytes per point:
//
//	| FPS | Chunk Size | Message Size | Bandwidth |
//	|-----|------------|--------------|-----------|
//	| 10  | 50,000     | 800 KB       | 8 MB/sec  |
//	| 30  | 16,666     | 266 KB       | 8 MB/sec  |
//	| 60  | 8,333      | 133 KB       | 8 MB/sec  |
//	| 144 | 3,472      | 55 KB        | 8 MB/sec  |
func calculateChunkSize(fps float64) int {
	if fps <= 0 {
		return MaxChunkSize
	}

	stride := getStride()
	targetBytesPerSecond := TargetBandwidthMBps * 1024 * 1024
	pointsPerSecond := targetBytesPerSecond / float64(stride)
	chunkSize := int(pointsPerSecond / fps)
	if chunkSize < MinChunkSize {
		return MinChunkSize
	}
	if chunkSize > MaxChunkSize {
		return MaxChunkSize
	}

	return chunkSize
}

func (worldState *WorldStateStore) updatePointCloud(elapsed time.Duration) {
	worldState.mu.Lock()
	transform, exists := worldState.transforms[pointcloudUUID]
	if !exists {
		worldState.mu.Unlock()
		return
	}

	originalPC := transform.PhysicalObject.GetPointcloud()
	if originalPC == nil || originalPC.Header == nil {
		worldState.mu.Unlock()
		return
	}

	totalPoints := int(originalPC.Header.Width)
	if totalPoints == 0 {
		worldState.mu.Unlock()
		return
	}

	originalData := originalPC.PointCloud
	worldState.mu.Unlock()
	worldState.mu.RLock()
	fps := worldState.fps
	worldState.mu.RUnlock()
	if fps < 0 {
		fps = 0
	}

	chunkSize := calculateChunkSize(fps)
	worldState.noiseTime = elapsed.Seconds()
	frameNumber := int(elapsed.Seconds() * fps)
	totalChunks := (totalPoints + chunkSize - 1) / chunkSize
	chunkIdx := frameNumber % totalChunks
	startIdx := chunkIdx * chunkSize
	count := chunkSize
	if startIdx+count > totalPoints {
		count = totalPoints - startIdx
	}

	stride := getStride()
	chunkData := createBuffer(count * stride)
	for i := 0; i < count; i++ {
		pointIdx := startIdx + i
		originalOffset := pointIdx * stride
		if originalOffset+stride > len(originalData) {
			worldState.logger.Errorw("point index out of bounds", "pointIdx", pointIdx)
			continue
		}

		origX := math.Float32frombits(binary.LittleEndian.Uint32(originalData[originalOffset : originalOffset+4]))
		origY := math.Float32frombits(binary.LittleEndian.Uint32(originalData[originalOffset+4 : originalOffset+8]))
		origZ := math.Float32frombits(binary.LittleEndian.Uint32(originalData[originalOffset+8 : originalOffset+12]))
		posX := float64(origX * 1000.0)
		posY := float64(origY * 1000.0)
		noiseScale := 0.005
		noiseValue := noise(
			posX*noiseScale,
			posY*noiseScale,
			worldState.noiseTime*0.8,
		)

		waveAmplitude := 5000.0
		heightOffset := waveAmplitude * noiseValue
		animatedZ := origZ + float32(heightOffset/1000.0)
		animatedZMm := float64(animatedZ * 1000.0)
		zRange := worldState.maxZ - worldState.minZ
		if zRange == 0 {
			zRange = 1
		}
		normalizedHeight := animatedZMm / zRange
		heightColor := heightToColor(normalizedHeight)
		animatedRGB := packColor(heightColor)

		offset := i * stride
		binary.LittleEndian.PutUint32(chunkData[offset:], math.Float32bits(origX))
		binary.LittleEndian.PutUint32(chunkData[offset+4:], math.Float32bits(origY))
		binary.LittleEndian.PutUint32(chunkData[offset+8:], math.Float32bits(animatedZ))
		binary.LittleEndian.PutUint32(chunkData[offset+12:], math.Float32bits(animatedRGB))
	}

	header := buildUpdateHeader(uint32(startIdx), uint32(count))
	if err := validatePointCloud(chunkData, header); err != nil {
		worldState.logger.Errorw("chunk data validation failed", "error", err)
		return
	}

	worldState.mu.Lock()
	if transform, exists := worldState.transforms[pointcloudUUID]; exists {
		pc := transform.PhysicalObject.GetPointcloud()
		if pc != nil && pc.PointCloud != nil {
			for i := 0; i < count; i++ {
				pointIdx := startIdx + i
				srcOffset := i * stride
				dstOffset := pointIdx * stride
				if dstOffset+stride <= len(pc.PointCloud) && srcOffset+stride <= len(chunkData) {
					copy(pc.PointCloud[dstOffset:dstOffset+stride], chunkData[srcOffset:srcOffset+stride])
				}
			}
		}
	}
	worldState.mu.Unlock()

	updatedFields := []string{
		"physicalObject.geometryType.value.pointCloud.pointCloud",
		"physicalObject.geometryType.value.pointCloud.header",
	}

	deltaTransform := &commonpb.Transform{
		Uuid: transform.Uuid,
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Pointcloud{
				Pointcloud: &commonpb.PointCloud{
					PointCloud: chunkData,
					Header:     header,
				},
			},
		},
	}

	worldState.emitTransformChange(deltaTransform, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED, updatedFields)
}

func (worldState *WorldStateStore) emitTransformChange(transform *commonpb.Transform, changeType pb.TransformChangeType, updatedFields []string) {
	if transform == nil || len(transform.GetUuid()) == 0 {
		return
	}

	change := worldstatestore.TransformChange{
		ChangeType:    changeType,
		Transform:     transform,
		UpdatedFields: updatedFields,
	}

	select {
	case worldState.changeChan <- change:
		// Successfully sent
	case <-worldState.streamCtx.Done():
		return
	default:
		// Channel is full - implement backpressure strategy with brief timeout
		select {
		case worldState.changeChan <- change:
		case <-time.After(time.Millisecond):
			// Drop update after timeout to prevent blocking
		case <-worldState.streamCtx.Done():
			return
		}
	}
}

func (worldState *WorldStateStore) animate() {
	var (
		curFPS          = 10.0
		interval        = time.Duration(float64(time.Second) / curFPS)
		ticker          = time.NewTicker(interval)
		lastElapsed     time.Duration
		fpsCheckCounter int64
	)
	defer ticker.Stop()

	for {
		select {
		case <-worldState.streamCtx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(worldState.startTime)
			if elapsed-lastElapsed < time.Millisecond {
				continue
			}

			lastElapsed = elapsed
			worldState.update()

			fpsCheckCounter++
			if fpsCheckCounter%100 == 0 {
				worldState.mu.RLock()
				newFPS := worldState.fps
				worldState.mu.RUnlock()

				if newFPS != curFPS && newFPS > 0 {
					ticker.Stop()
					curFPS = newFPS
					interval = time.Duration(float64(time.Second) / curFPS)
					ticker = time.NewTicker(interval)
				}
			}
		}
	}
}

func (worldState *WorldStateStore) boxSequence() {
	delay := 3 * time.Second
	sequence := []struct {
		action string
		name   string
	}{
		{"add", "box-front-box"},
		{"remove", "box-front-box"},
		{"add", "box-front-sphere"},
		{"remove", "box-front-sphere"},
		{"add", "box-front-capsule"},
		{"remove", "box-front-capsule"},
	}

	for {
		for _, step := range sequence {
			select {
			case <-worldState.streamCtx.Done():
				return
			default:
			}

			switch step.action {
			case "add":
				worldState.addBox(step.name)
			case "remove":
				worldState.removeBox(step.name)
			}

			select {
			case <-worldState.streamCtx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

func (worldState *WorldStateStore) addBox(name string) {
	var xOffset float64

	switch name {
	case "box-front-box":
		xOffset = -5000 // In front of the main box
	case "box-front-sphere":
		xOffset = 0 // In front of the sphere
	case "box-front-capsule":
		xOffset = 5000 // In front of the capsule
	}

	uuid := name + "-" + time.Now().Format("20060102150405")
	transform := &commonpb.Transform{
		ReferenceFrame: "dynamic-box",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: xOffset, Y: -2000, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{
						X: 500,
						Y: 500,
						Z: 500,
					},
				},
			},
		},
		Uuid:     []byte(uuid),
		Metadata: dynamicBoxMetadata,
	}

	worldState.mu.Lock()
	worldState.transforms[uuid] = transform
	worldState.mu.Unlock()

	worldState.emitTransformChange(transform, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, nil)
}

func (worldState *WorldStateStore) removeBox(name string) {
	worldState.mu.Lock()

	var uuidToRemove string
	for uuid := range worldState.transforms {
		if strings.HasPrefix(uuid, name) {
			uuidToRemove = uuid
			break
		}
	}

	if uuidToRemove == "" {
		worldState.mu.Unlock()
		return
	}

	transform := worldState.transforms[uuidToRemove]
	delete(worldState.transforms, uuidToRemove)
	worldState.mu.Unlock()

	change := worldstatestore.TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
		Transform: &commonpb.Transform{
			Uuid: transform.Uuid,
		},
	}

	select {
	case worldState.changeChan <- change:
	case <-worldState.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}

func (worldState *WorldStateStore) update() {
	elapsed := time.Since(worldState.startTime)

	worldState.updateBox(elapsed)
	worldState.updateSphere(elapsed)
	worldState.updateCapsule(elapsed)
	worldState.updatePointCloud(elapsed)
}
