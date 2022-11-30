package datacapture

import (
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
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

// TODO: rewrite tests
func TestCaptureQueue(t *testing.T) {
	//MaxFileSize = 50
	//tests := []struct {
	//	name               string
	//	dataType           v1.DataType
	//	pushCount          int
	//	expCompleteFiles   int
	//	expInProgressFiles int
	//}{
	//	{
	//		name:      "Files that are still being written to should have the InProgressFileExt extension.",
	//		dataType:  v1.DataType_DATA_TYPE_TABULAR_SENSOR,
	//		pushCount: 1,
	//	},
	//	{
	//		name:             "Pushing N binary data should write N files.",
	//		dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
	//		pushCount:        2,
	//		expCompleteFiles: 2,
	//	},
	//	{
	//		name:     "Pushing > MaxFileSize + 1 worth of struct data should write two files.",
	//		dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
	//		// MaxFileSize / size(structSensorData) = ceil(50 / 19) = 3 per file => 2 pops for 4 pushes
	//		pushCount:          4,
	//		expCompleteFiles:   1,
	//		expInProgressFiles: 1,
	//	},
	//}
	//
	//for _, tc := range tests {
	//	t.Run(tc.name, func(t *testing.T) {
	//		tmpDir, err := os.MkdirTemp("", "")
	//		defer os.RemoveAll(tmpDir)
	//		test.That(t, err, test.ShouldBeNil)
	//		md := &v1.DataCaptureMetadata{Type: tc.dataType}
	//		sut := NewQueue(tmpDir, md)
	//		var pushValue *v1.SensorData
	//		if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
	//			pushValue = binarySensorData
	//		} else {
	//			pushValue = structSensorData
	//		}
	//
	//		for i := 0; i < tc.pushCount; i++ {
	//			err := sut.Push(pushValue)
	//			test.That(t, err, test.ShouldBeNil)
	//		}
	//
	//		var totalReadings1 int
	//		for i := 0; i < tc.expCompleteFiles; i++ {
	//			popped, err := sut.Pop()
	//			test.That(t, err, test.ShouldBeNil)
	//			test.That(t, popped, test.ShouldNotBeNil)
	//			test.That(t, popped, test.ShouldNotBeNil)
	//			for {
	//				next, err := popped.ReadNext()
	//				if errors.Is(err, io.EOF) {
	//					break
	//				}
	//				test.That(t, err, test.ShouldBeNil)
	//				if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
	//					test.That(t, next.GetBinary(), test.ShouldResemble, pushValue.GetBinary())
	//				} else {
	//					test.That(t, next.GetStruct(), test.ShouldResemble, pushValue.GetStruct())
	//				}
	//				totalReadings1++
	//			}
	//		}
	//		test.That(t, totalReadings1, test.ShouldEqual, tc.pushCount)
	//
	//		for i := 0; i < tc.pushCount; i++ {
	//			err := sut.Push(pushValue)
	//			test.That(t, err, test.ShouldBeNil)
	//		}
	//
	//		var totalReadings2 int
	//		for i := 0; i < tc.expCompleteFiles; i++ {
	//			popped, err := sut.Pop()
	//			test.That(t, err, test.ShouldBeNil)
	//			test.That(t, popped, test.ShouldNotBeNil)
	//			test.That(t, popped, test.ShouldNotBeNil)
	//			for {
	//				next, err := popped.ReadNext()
	//				if errors.Is(err, io.EOF) {
	//					break
	//				}
	//				test.That(t, err, test.ShouldBeNil)
	//				if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
	//					test.That(t, next.GetBinary(), test.ShouldResemble, pushValue.GetBinary())
	//				} else {
	//					test.That(t, next.GetStruct(), test.ShouldResemble, pushValue.GetStruct())
	//				}
	//				totalReadings2++
	//			}
	//		}
	//		test.That(t, totalReadings2, test.ShouldEqual, tc.pushCount)
	//
	//		next, err := sut.Pop()
	//		test.That(t, err, test.ShouldBeNil)
	//		test.That(t, next, test.ShouldBeNil)
	//
	//		// Test that close is respected: after closing, all files should no longer be in progress..
	//		err = sut.Close()
	//		test.That(t, err, test.ShouldBeNil)
	//		test.That(t, sut.IsClosed(), test.ShouldBeTrue)
	//	})
	//}
}
