package testutils

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
)

// DoCommandTestConfig holds configuration for DoCommand tests
type DoCommandTestConfig struct {
	ComponentName   string
	CaptureInterval time.Duration
	DoCommandMap    map[string]interface{}
	Collector       data.CollectorConstructor
	ResourceFactory func() interface{} // Factory function to create the component-specific resource
}

// TestDoCommandCollector runs a comprehensive test suite for DoCommand collectors
func TestDoCommandCollector(t *testing.T, config DoCommandTestConfig) {
	tests := []struct {
		name         string
		methodParams map[string]*anypb.Any
		expectError  bool
	}{
		{
			name: "DoCommand collector should write a list of values",
			methodParams: map[string]*anypb.Any{
				"docommand_input": func() *anypb.Any {
					structVal := ToStructPBStruct(t, map[string]any{
						"command": "random",
					})
					anyVal, _ := anypb.New(structVal)
					return anyVal
				}(),
			},
		},
		{
			name: "DoCommand collector should handle empty struct payload",
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
			name: "DoCommand collector should handle empty payload",
			methodParams: map[string]*anypb.Any{
				"docommand_input": {},
			},
		},
		{
			name:         "DoCommand collector should error on missing payload",
			methodParams: map[string]*anypb.Any{},
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: config.ComponentName,
				Interval:      config.CaptureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				MethodParams:  tc.methodParams,
			}

			// Use the component-specific resource factory
			resource := config.ResourceFactory()

			col, err := config.Collector(resource, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.expectError {
				test.That(t, len(buf.Writes), test.ShouldEqual, 0)
			} else {
				CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{
						Struct: ToStructPBStruct(t, map[string]any{
							"docommand_output": config.DoCommandMap,
						}),
					},
				}})
			}
			buf.Close()
		})
	}
}
