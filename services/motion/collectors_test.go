package motion_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	motion "go.viam.com/rdk/services/motion"
	tu "go.viam.com/rdk/testutils"
	inject "go.viam.com/rdk/testutils/inject/motion"
)

const (
	componentName   = "motion"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tests := []struct {
		name         string
		collector    data.CollectorConstructor
		methodParams map[string]*anypb.Any
		expectError  bool
	}{
		{
			name:      "DoCommand collector should write a list of values",
			collector: motion.NewDoCommandCollector,
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
			collector: motion.NewDoCommandCollector,
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
			collector: motion.NewDoCommandCollector,
			methodParams: map[string]*anypb.Any{
				"docommand_input": {},
			},
		},
		{
			name:         "DoCommand collector should error on missing payload",
			collector:    motion.NewDoCommandCollector,
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
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				MethodParams:  tc.methodParams,
			}

			sw := newMotion()
			col, err := tc.collector(sw, params)
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
						Struct: tu.ToStructPBStruct(t, map[string]any{
							"docommand_output": doCommandMap,
						}),
					},
				}})
			}
			buf.Close()
		})
	}
}

func newMotion() motion.Service {
	m := &inject.MotionService{}
	m.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return m
}
