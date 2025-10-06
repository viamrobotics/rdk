// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fake

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/aquilax/go-perlin"
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

	noise     *perlin.Perlin
	noiseTime float64
	minZ      float64
	maxZ      float64
	width     int
	height    int
	spacing   float64
}

const (
	// GPUBufferAlignment ensures optimal GPU memory transfer (aligned to 256 bytes)
	GPUBufferAlignment = 256

	// MinFPS is the minimum non-zero frame rate
	MinFPS = 0

	// MaxFPS is the maximum supported frame rate (matches high-end monitors)
	MaxFPS = 144.0

	// MaxPointCloudSize is the maximum point cloud size
	MaxPointCloudSize = 1000000

	// MinPointCloudSpacing is the minimum point cloud spacing
	MinPointCloudSpacing = 1

	// TargetBandwidthMBps is the target bandwidth in MB/sec for point cloud updates
	TargetBandwidthMBps = 2.0

	// MinChunkSize is the minimum chunk size to avoid excessive overhead
	MinChunkSize = 1000

	// MaxChunkSize is the maximum chunk size - capped at 512KB worth of points
	MaxChunkSize = 32000
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
)

func init() {
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
// - "fps": float64 to set the animation rate (0 to pause)
//
// - "point_cloud_width": int to set the width of the point cloud
//
// - "point_cloud_height": int to set the height of the point cloud
//
// - "point_cloud_spacing": float64 to set the spacing of the point cloud
func (worldState *WorldStateStore) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	var statusMsgs []string
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

		msg := fmt.Sprintf("fps set to %.2f", fps)
		if fps != originalFPS {
			msg += fmt.Sprintf(" (clamped from %.2f)", originalFPS)
		}
		statusMsgs = append(statusMsgs, msg)
	}

	worldState.mu.Lock()
	currentWidth := worldState.width
	currentHeight := worldState.height
	worldState.mu.Unlock()

	if pointCloudWidth, ok := cmd["point_cloud_width"].(int); ok {
		originalWidth := pointCloudWidth
		if pointCloudWidth < 1 {
			worldState.logger.Warnf("Point cloud width %d is below 1, clamping to 1", pointCloudWidth)
			pointCloudWidth = 1
		}

		height := currentHeight
		if h, ok := cmd["point_cloud_height"].(int); ok {
			height = h
		}
		if pointCloudWidth*height > MaxPointCloudSize {
			pointCloudWidth = MaxPointCloudSize / height
			worldState.logger.Warnf("Point cloud dimensions exceed maximum points (%d), clamping width to %d", MaxPointCloudSize, pointCloudWidth)
		}

		worldState.mu.Lock()
		worldState.width = pointCloudWidth
		worldState.mu.Unlock()

		msg := fmt.Sprintf("point cloud width set to %d", pointCloudWidth)
		if pointCloudWidth != originalWidth {
			msg += fmt.Sprintf(" (clamped from %d)", originalWidth)
		}
		statusMsgs = append(statusMsgs, msg)
	}

	if pointCloudHeight, ok := cmd["point_cloud_height"].(int); ok {
		originalHeight := pointCloudHeight
		if pointCloudHeight < 1 {
			worldState.logger.Warnf("Point cloud height %d is below 1, clamping to 1", pointCloudHeight)
			pointCloudHeight = 1
		}

		width := currentWidth
		if w, ok := cmd["point_cloud_width"].(int); ok {
			width = w
		}
		if width*pointCloudHeight > MaxPointCloudSize {
			pointCloudHeight = MaxPointCloudSize / width
			worldState.logger.Warnf("Point cloud dimensions exceed maximum points (%d), clamping height to %d", MaxPointCloudSize, pointCloudHeight)
		}

		worldState.mu.Lock()
		worldState.height = pointCloudHeight
		worldState.mu.Unlock()

		msg := fmt.Sprintf("point cloud height set to %d", pointCloudHeight)
		if pointCloudHeight != originalHeight {
			msg += fmt.Sprintf(" (clamped from %d)", originalHeight)
		}
		statusMsgs = append(statusMsgs, msg)
	}

	if pointCloudSpacing, ok := cmd["point_cloud_spacing"].(float64); ok {
		originalSpacing := pointCloudSpacing
		if pointCloudSpacing < 1 {
			worldState.logger.Warnf("Point cloud spacing %.2f is below 1, clamping to 1", pointCloudSpacing)
			pointCloudSpacing = 1
		}

		worldState.mu.Lock()
		worldState.spacing = pointCloudSpacing
		worldState.mu.Unlock()

		msg := fmt.Sprintf("point cloud spacing set to %.2f", pointCloudSpacing)
		if pointCloudSpacing != originalSpacing {
			msg += fmt.Sprintf(" (clamped from %.2f)", originalSpacing)
		}
		statusMsgs = append(statusMsgs, msg)
	}

	if len(statusMsgs) == 0 {
		return map[string]any{
			"status": "no commands processed",
		}, nil
	}

	return map[string]any{
		"status": strings.Join(statusMsgs, "; "),
	}, nil
}

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
	noise := perlin.NewPerlin(2, 2, 5, 1337)

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
		noise:                   noise,
		width:                   750,
		height:                  750,
		spacing:                 100,
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

func pointCloudToRawBytes(pc pointcloud.PointCloud) []byte {
	stride := getStride()
	size := pc.Size()
	data := make([]byte, size*stride)

	idx := 0
	pc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		offset := idx * stride
		xMeters := float32(p.X / 1000.0)
		yMeters := float32(p.Y / 1000.0)
		zMeters := float32(p.Z / 1000.0)

		var rgb float32
		if d != nil && d.HasValue() {
			rgb = math.Float32frombits(uint32(d.Value()))
		}

		*(*float32)(unsafe.Pointer(&data[offset])) = xMeters
		*(*float32)(unsafe.Pointer(&data[offset+4])) = yMeters
		*(*float32)(unsafe.Pointer(&data[offset+8])) = zMeters
		*(*float32)(unsafe.Pointer(&data[offset+12])) = rgb

		idx++
		return true
	})

	return data
}

func packColor(colorValue color.NRGBA) float32 {
	rgb := (uint32(colorValue.R) << 16) | (uint32(colorValue.G) << 8) | uint32(colorValue.B)
	return math.Float32frombits(rgb)
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

	pointCloud := worldState.generatePointCloud(worldState.width, worldState.height)
	pointcloudBytes := pointCloudToRawBytes(pointCloud)
	pointcloudHeader := buildUpdateHeader(0, uint32(worldState.width*worldState.height))

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
		offset := i * stride

		x := pointIdx / worldState.width
		y := pointIdx % worldState.height

		originalOffset := pointIdx * stride
		currentHeightMeters := float32(0)
		if originalOffset+8 < len(originalData) {
			currentHeightMeters = math.Float32frombits(
				uint32(originalData[originalOffset+8]) |
					uint32(originalData[originalOffset+9])<<8 |
					uint32(originalData[originalOffset+10])<<16 |
					uint32(originalData[originalOffset+11])<<24,
			)
		}

		currentNoiseValue := float64(currentHeightMeters*1000.0) / (worldState.spacing * 10.0)
		newNoiseValue := worldState.updatePointHeight(x, y, currentNoiseValue, worldState.noiseTime)

		heightColor := heightToColor(newNoiseValue)
		animatedRGB := packColor(heightColor)

		xMeters := float32(x) * float32(worldState.spacing) / 1000.0
		yMeters := float32(y) * float32(worldState.spacing) / 1000.0
		zMeters := float32((newNoiseValue * worldState.spacing * 10.0) / 1000.0)

		*(*float32)(unsafe.Pointer(&chunkData[offset])) = xMeters
		*(*float32)(unsafe.Pointer(&chunkData[offset+4])) = yMeters
		*(*float32)(unsafe.Pointer(&chunkData[offset+8])) = zMeters
		*(*float32)(unsafe.Pointer(&chunkData[offset+12])) = animatedRGB
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
