package data

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestBuildCaptureMetadata(t *testing.T) {
	tests := []struct {
		name             string
		componentType    string
		componentName    string
		method           string
		additionalParams map[string]interface{}
		dataType         v1.DataType
		fileExtension    string
		tags             []string
	}{
		{
			name:             "Metadata for arm positions stored in a length delimited proto file",
			componentType:    "arm",
			componentName:    "arm1",
			method:           "EndPosition",
			additionalParams: make(map[string]interface{}),
			dataType:         v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			fileExtension:    ".dat",
			tags:             []string{"tagA", "tagB"},
		},
		{
			name:             "Metadata for a camera Next() image stored as a binary .jpeg file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".jpeg",
			tags:             []string{},
		},
		{
			name:             "Metadata for a camera Next() image stored as a binary .png file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypePNG},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".png",
			tags:             []string{},
		},
		{
			name:             "Metadata for a LiDAR Next() point cloud stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypePCD},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
			tags:             []string{},
		},
		{
			name:             "Metadata for a camera Next() image which has no mime_type in the data capture config",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]interface{}{},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			tags:             []string{},
		},
		{
			name:             "Metadata for a camera Next() image stored as a an unknown file format",
			componentType:    "camera",
			componentName:    "cam1",
			method:           readImage,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeQOI},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			tags:             []string{},
		},
		{
			name:             "Metadata for a GetImages() response",
			componentType:    "camera",
			componentName:    "cam1",
			method:           GetImages,
			additionalParams: make(map[string]interface{}),
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			tags:             []string{},
		},
		{
			name:             "Metadata for a GetImages() response even if you add meme_type tags",
			componentType:    "camera",
			componentName:    "cam1",
			method:           GetImages,
			additionalParams: map[string]interface{}{"mime_type": utils.MimeTypeJPEG},
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			tags:             []string{},
		},
		{
			name:             "Metadata for a LiDAR NextPointCloud() stored as a binary .pcd file",
			componentType:    "camera",
			componentName:    "cam1",
			method:           nextPointCloud,
			additionalParams: make(map[string]interface{}),
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			fileExtension:    ".pcd",
			tags:             []string{},
		},
		{
			name:             "Metadata for a LiDAR NextPointCloud() stored as a binary .pcd file",
			componentType:    "slam",
			componentName:    "slam1",
			method:           pointCloudMap,
			additionalParams: make(map[string]interface{}),
			dataType:         v1.DataType_DATA_TYPE_BINARY_SENSOR,
			tags:             []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			methodParams, err := protoutils.ConvertMapToProtoAny(tc.additionalParams)
			test.That(t, err, test.ShouldEqual, nil)

			actualMetadata, _ := BuildCaptureMetadata(
				resource.APINamespaceRDK.WithComponentType(tc.componentType),
				tc.componentName,
				tc.method,
				tc.additionalParams,
				methodParams,
				tc.tags,
			)

			expectedMetadata := v1.DataCaptureMetadata{
				ComponentType:    resource.APINamespaceRDK.WithComponentType(tc.componentType).String(),
				ComponentName:    tc.componentName,
				MethodName:       tc.method,
				Type:             tc.dataType,
				MethodParameters: methodParams,
				Tags:             tc.tags,
				FileExtension:    tc.fileExtension,
			}
			test.That(t, actualMetadata.String(), test.ShouldEqual, expectedMetadata.String())
		})
	}
}

func TestBinaryPayloadReader(t *testing.T) {
	// captureFileFromSensorData writes msgs to a new capture file, closes it (which
	// renames it from .prog to .capture), then reopens it for reading.
	captureFileFromSensorData := func(t *testing.T, msgs ...*v1.SensorData) *CaptureFile {
		t.Helper()
		dir := t.TempDir()
		cf, err := NewCaptureFile(dir, &v1.DataCaptureMetadata{Type: v1.DataType_DATA_TYPE_BINARY_SENSOR})
		test.That(t, err, test.ShouldBeNil)
		for _, msg := range msgs {
			test.That(t, cf.WriteNext(msg), test.ShouldBeNil)
		}
		test.That(t, cf.Close(), test.ShouldBeNil)
		entries, err := os.ReadDir(dir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(entries), test.ShouldEqual, 1)
		f, err := os.Open(filepath.Join(dir, entries[0].Name()))
		test.That(t, err, test.ShouldBeNil)
		readCF, err := ReadCaptureFile(f)
		test.That(t, err, test.ShouldBeNil)
		return readCF
	}

	// captureFileFromRawBytes builds a capture file with a manually constructed SensorData
	// body, allowing tests to inject arbitrary wire-format bytes.
	captureFileFromRawBytes := func(t *testing.T, body []byte) *CaptureFile {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "*"+CompletedCaptureFileExt)
		test.That(t, err, test.ShouldBeNil)
		_, err = pbutil.WriteDelimited(f, &v1.DataCaptureMetadata{Type: v1.DataType_DATA_TYPE_BINARY_SENSOR})
		test.That(t, err, test.ShouldBeNil)
		var lenBuf [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(lenBuf[:], uint64(len(body)))
		_, err = f.Write(lenBuf[:n])
		test.That(t, err, test.ShouldBeNil)
		_, err = f.Write(body)
		test.That(t, err, test.ShouldBeNil)
		_, err = f.Seek(0, io.SeekStart)
		test.That(t, err, test.ShouldBeNil)
		cf, err := ReadCaptureFile(f)
		test.That(t, err, test.ShouldBeNil)
		return cf
	}

	tcs := []struct {
		name         string
		setup        func(t *testing.T) *CaptureFile
		wantPayloads [][]byte
		wantErr      error
	}{
		{
			name: "single message",
			setup: func(t *testing.T) *CaptureFile {
				return captureFileFromSensorData(t, &v1.SensorData{
					Metadata: &v1.SensorMetadata{},
					Data:     &v1.SensorData_Binary{Binary: []byte("single binary payload")},
				})
			},
			wantPayloads: [][]byte{[]byte("single binary payload")},
		},
		{
			name: "multiple messages",
			setup: func(t *testing.T) *CaptureFile {
				msgs := make([]*v1.SensorData, 5)
				for i := range msgs {
					msgs[i] = &v1.SensorData{
						Metadata: &v1.SensorMetadata{},
						Data:     &v1.SensorData_Binary{Binary: []byte(fmt.Sprintf("payload-%d", i))},
					}
				}
				return captureFileFromSensorData(t, msgs...)
			},
			wantPayloads: [][]byte{
				[]byte("payload-0"),
				[]byte("payload-1"),
				[]byte("payload-2"),
				[]byte("payload-3"),
				[]byte("payload-4"),
			},
		},
		{
			name: "unknown non-bytes wire type field is skipped",
			setup: func(t *testing.T) *CaptureFile {
				metaBytes, err := proto.Marshal(&v1.SensorMetadata{})
				test.That(t, err, test.ShouldBeNil)
				var body []byte
				body = protowire.AppendTag(body, 1, protowire.BytesType)
				body = protowire.AppendBytes(body, metaBytes)
				// Unknown field 99 with varint wire type — must be skipped gracefully.
				body = protowire.AppendTag(body, 99, protowire.VarintType)
				body = protowire.AppendVarint(body, 42)
				body = protowire.AppendTag(body, 3, protowire.BytesType)
				body = protowire.AppendBytes(body, []byte("payload after unknown varint field"))
				return captureFileFromRawBytes(t, body)
			},
			wantPayloads: [][]byte{[]byte("payload after unknown varint field")},
		},
		{
			name: "no binary field returns error",
			setup: func(t *testing.T) *CaptureFile {
				return captureFileFromSensorData(t, &v1.SensorData{
					Metadata: &v1.SensorMetadata{},
					Data:     &v1.SensorData_Struct{Struct: &structpb.Struct{}},
				})
			},
			wantErr: ErrNoBinaryField,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cf := tc.setup(t)
			cf.Reset()

			if tc.wantErr != nil {
				_, _, _, err := cf.BinaryPayloadReader()
				test.That(t, err, test.ShouldBeError, tc.wantErr)
				return
			}

			for _, want := range tc.wantPayloads {
				meta, payloadLen, r, err := cf.BinaryPayloadReader()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, meta, test.ShouldNotBeNil)
				test.That(t, payloadLen, test.ShouldEqual, int64(len(want)))
				got, err := io.ReadAll(r)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, got, test.ShouldResemble, want)
			}
			_, _, _, err := cf.BinaryPayloadReader()
			test.That(t, err, test.ShouldEqual, io.EOF)
		})
	}
}

// TestReadCorruptedFile ensures that if a file ends with invalid data (which can occur if a robot is killed uncleanly
// during a write, e.g. if the power is cut), the file is still successfully read up until that point.
func TestReadCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	md := &v1.DataCaptureMetadata{
		Type: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
	}
	f, err := NewCaptureFile(dir, md)
	test.That(t, err, test.ShouldBeNil)
	numReadings := 100
	for i := 0; i < numReadings; i++ {
		err := f.WriteNext(&v1.SensorData{
			Metadata: &v1.SensorMetadata{},
			Data:     &v1.SensorData_Struct{Struct: &structpb.Struct{}},
		})
		test.That(t, err, test.ShouldBeNil)
	}
	_, err = f.writer.Write([]byte("invalid data"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.writer.Flush(), test.ShouldBeNil)

	// Should still be able to successfully read all the successfully written data.
	sd, err := SensorDataFromCaptureFilePath(f.GetPath())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(sd), test.ShouldEqual, numReadings)
}
