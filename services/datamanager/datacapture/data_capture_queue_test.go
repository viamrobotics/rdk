package datacapture

import (
	"fmt"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
	"io"
	"os"
	"testing"
)

/**
Things to test!

- Can push then pop binary data, and pushes:pops are 1:1
- Can push then pop tabular data, and MAX_SIZE is respected. Should push enough to have 2 pops.
- Can intermix push and pop. E.g. for above two, should do push->pop->push->pop
- That close is respected: flushes everything, and then trying to pop again errors
*/

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
			name:     "Pushing > maxSize + 1 worth of struct data should allow 2 files to be popped.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// maxSize / size(structSensorData) = 4096 / VALUE = 2
			firstPushCount: 400,
			firstPopCount:  2,
		},
		{
			name:     "Intermixing pushes/pops of binary data should not cause data races.",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
			// maxSize / size(structSensorData) = 4096 / VALUE = 2
			firstPushCount:  2,
			firstPopCount:   2,
			secondPushCount: 2,
			secondPopCount:  2,
		},
		{
			name:     "Intermixing pushes/pops of tabular data should not cause data races.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// maxSize / size(structSensorData) = 4096 / VALUE = 2
			firstPushCount:  400,
			firstPopCount:   2,
			secondPushCount: 400,
			secondPopCount:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Println("starting test")
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

			fmt.Println("starting push")
			for i := 0; i < tc.firstPushCount; i++ {
				err := sut.Push(pushValue)
				fmt.Println("pushed")
				test.That(t, err, test.ShouldBeNil)
			}
			fmt.Println("done pushing")

			var totalReadings1 int
			for i := 0; i < tc.firstPopCount; i++ {
				fmt.Println(i)
				popped, err := sut.Pop()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				for {
					next, err := popped.ReadNext()
					if errors.Is(err, io.EOF) {
						fmt.Println("got EOF")
						break
					}
					fmt.Println("didn't get EOF")
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
				fmt.Println("pushed")
				test.That(t, err, test.ShouldBeNil)
			}
			fmt.Println("done pushing")

			var totalReadings2 int
			for i := 0; i < tc.firstPopCount; i++ {
				fmt.Println(i)
				popped, err := sut.Pop()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				test.That(t, popped, test.ShouldNotBeNil)
				for {
					next, err := popped.ReadNext()
					if errors.Is(err, io.EOF) {
						fmt.Println("got EOF")
						break
					}
					fmt.Println("didn't get EOF")
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
			next, err = sut.Pop()
			test.That(t, errors.Is(err, ErrQueueClosed), test.ShouldBeTrue)
			test.That(t, next, test.ShouldBeNil)
		})
	}
}
