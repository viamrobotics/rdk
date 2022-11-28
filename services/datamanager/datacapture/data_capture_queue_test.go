package datacapture

import (
	"io"
	"os"
	"testing"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
)

type structReading struct {
	Field1 bool
}

func (r structReading) toProto() *structpb.Struct {
	msg, err := protoutils.StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

var (
	structSensorData = &v1.SensorData{
		Metadata: &v1.SensorMetadata{},
		Data:     &v1.SensorData_Struct{Struct: structReading{}.toProto()},
	}
	binarySensorData = &v1.SensorData{
		Metadata: &v1.SensorMetadata{},
		Data: &v1.SensorData_Binary{
			Binary: []byte("this sure is binary data, yup it is"),
		},
	}
)

func TestCaptureQueueSimple(t *testing.T) {
	MaxFileSize = 50
	tests := []struct {
		name            string
		dataType        v1.DataType
		firstPushCount  int
		firstPopCount   int
		secondPushCount int
		secondPopCount  int
	}{
		{
			name:           "Pushing N binary data should allow N files to be popped.",
			dataType:       v1.DataType_DATA_TYPE_BINARY_SENSOR,
			firstPushCount: 2,
			firstPopCount:  2,
		},
		{
			name:     "Pushing > MaxFileSize + 1 worth of struct data should allow 3 files to be popped.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// MaxFileSize / size(structSensorData) = ceil(50 / 19) = 3 per file => 2 pops for 4 pushes
			firstPushCount: 4,
			firstPopCount:  2,
		},
		{
			name:            "Intermixing pushes/pops of binary data should not cause data races.",
			dataType:        v1.DataType_DATA_TYPE_BINARY_SENSOR,
			firstPushCount:  2,
			firstPopCount:   2,
			secondPushCount: 2,
			secondPopCount:  2,
		},
		{
			name:     "Intermixing pushes/pops of tabular data should not cause data races.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// MaxFileSize / size(structSensorData) = ceil(50 / 19) = 3 per file => 2 pops for 4 pushes
			firstPushCount:  4,
			firstPopCount:   2,
			secondPushCount: 4,
			secondPopCount:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "")
			defer os.RemoveAll(tmpDir)
			test.That(t, err, test.ShouldBeNil)
			md := &v1.DataCaptureMetadata{Type: tc.dataType}
			sut := NewQueue(tmpDir, md)
			var pushValue *v1.SensorData
			if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
				pushValue = binarySensorData
			} else {
				pushValue = structSensorData
			}

			for i := 0; i < tc.firstPushCount; i++ {
				err := sut.Push(pushValue)
				test.That(t, err, test.ShouldBeNil)
			}

			var totalReadings1 int
			for i := 0; i < tc.firstPopCount; i++ {
				popped, err := sut.Pop()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				for {
					next, err := popped.ReadNext()
					if errors.Is(err, io.EOF) {
						break
					}
					test.That(t, err, test.ShouldBeNil)
					if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
						test.That(t, next.GetBinary(), test.ShouldResemble, pushValue.GetBinary())
					} else {
						test.That(t, next.GetStruct(), test.ShouldResemble, pushValue.GetStruct())
					}
					totalReadings1++
				}
			}
			test.That(t, totalReadings1, test.ShouldEqual, tc.firstPushCount)

			for i := 0; i < tc.firstPushCount; i++ {
				err := sut.Push(pushValue)
				test.That(t, err, test.ShouldBeNil)
			}

			var totalReadings2 int
			for i := 0; i < tc.firstPopCount; i++ {
				popped, err := sut.Pop()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				for {
					next, err := popped.ReadNext()
					if errors.Is(err, io.EOF) {
						break
					}
					test.That(t, err, test.ShouldBeNil)
					if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
						test.That(t, next.GetBinary(), test.ShouldResemble, pushValue.GetBinary())
					} else {
						test.That(t, next.GetStruct(), test.ShouldResemble, pushValue.GetStruct())
					}
					totalReadings2++
				}
			}
			test.That(t, totalReadings2, test.ShouldEqual, tc.firstPushCount)

			next, err := sut.Pop()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, next, test.ShouldBeNil)

			// Test that close is respected.
			err = sut.Close()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, sut.IsClosed(), test.ShouldBeTrue)
		})
	}
}
