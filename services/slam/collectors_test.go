package slam_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	serviceName     = "slam"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestCollectors(t *testing.T) {
	pcdPath := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
	pcd, err := os.ReadFile(pcdPath)
	test.That(t, err, test.ShouldBeNil)
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
		datatype  data.CaptureType
		slam      slam.Service
	}{
		{
			name:      "PositionCollector returns non-empty position responses",
			collector: slam.NewPositionCollector,
			datatype:  data.CaptureTypeTabular,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"pose": map[string]any{
						"o_x":   0,
						"o_y":   0,
						"o_z":   1,
						"theta": 0,
						"x":     1,
						"y":     2,
						"z":     3,
					},
				})},
			}},
			slam: newSlamService(pcdPath),
		},
		{
			name:      "PointCloudMapCollector returns non-empty pointcloud responses",
			collector: slam.NewPointCloudMapCollector,
			datatype:  data.CaptureTypeBinary,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_APPLICATION_PCD,
				},
				Data: &datasyncpb.SensorData_Binary{Binary: pcd},
			}},
			slam: newSlamService(pcdPath),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      tc.datatype,
				ComponentName: serviceName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
			}

			col, err := tc.collector(tc.slam, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
			buf.Close()
		})
	}
}

func TestDoCommandCollector(t *testing.T) {
	tests := []struct {
		name         string
		collector    data.CollectorConstructor
		methodParams map[string]*anypb.Any
		expectError  bool
	}{
		{
			name:      "DoCommand collector should write a list of values",
			collector: slam.NewDoCommandCollector,
			methodParams: map[string]*anypb.Any{
				"docommand_input": func() *anypb.Any {
					structVal := tu.ToStructPBStruct(t, map[string]any{
						"command": "random",
					})
					anyVal, _ := anypb.New(structVal)
					return anyVal
				}(),
			},
		},
		{
			name:      "DoCommand collector should handle empty struct payload",
			collector: slam.NewDoCommandCollector,
			methodParams: map[string]*anypb.Any{
				"docommand_input": func() *anypb.Any {
					emptyStruct := &structpb.Struct{
						Fields: make(map[string]*structpb.Value),
					}
					anyVal, _ := anypb.New(emptyStruct)
					return anyVal
				}(),
			},
		},
		{
			name:      "DoCommand collector should handle empty payload",
			collector: slam.NewDoCommandCollector,
			methodParams: map[string]*anypb.Any{
				"docommand_input": {},
			},
		},
		{
			name:         "DoCommand collector should error on missing payload",
			collector:    slam.NewDoCommandCollector,
			methodParams: map[string]*anypb.Any{},
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: serviceName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				MethodParams:  tc.methodParams,
			}

			pcdPath := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
			slam := newSlamService(pcdPath)
			col, err := tc.collector(slam, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.expectError {
				test.That(t, len(buf.Writes), test.ShouldEqual, 0)
			} else {
				tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{
						Struct: tu.ToStructPBStruct(t, doCommandMap),
					},
				}})
			}
			buf.Close()
		})
	}
}

func getPointCloudMap(path string) (func() ([]byte, error), error) {
	const chunkSizeBytes = 1 * 1024 * 1024
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	chunk := make([]byte, chunkSizeBytes)
	f := func() ([]byte, error) {
		bytesRead, err := file.Read(chunk)
		if err != nil {
			defer utils.UncheckedErrorFunc(file.Close)
			return nil, err
		}
		return chunk[:bytesRead], err
	}
	return f, nil
}

func newSlamService(pcdPath string) slam.Service {
	s := &inject.SLAMService{}
	s.PositionFunc = func(ctx context.Context) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	}

	s.PointCloudMapFunc = func(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {
		return getPointCloudMap(pcdPath)
	}

	s.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}

	return s
}
