package data

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
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
	t.Run("single message round-trip", func(t *testing.T) {
		dir := t.TempDir()
		cf, err := NewCaptureFile(dir, &v1.DataCaptureMetadata{Type: v1.DataType_DATA_TYPE_BINARY_SENSOR})
		test.That(t, err, test.ShouldBeNil)

		payload := []byte("single binary payload")
		err = cf.WriteNext(&v1.SensorData{
			Metadata: &v1.SensorMetadata{},
			Data:     &v1.SensorData_Binary{Binary: payload},
		})
		test.That(t, err, test.ShouldBeNil)

		cf.Reset()
		meta, payloadLen, r, err := cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, meta, test.ShouldNotBeNil)
		test.That(t, payloadLen, test.ShouldEqual, int64(len(payload)))

		got, err := io.ReadAll(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, payload)

		_, _, _, err = cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldEqual, io.EOF)
	})

	t.Run("multiple messages", func(t *testing.T) {
		dir := t.TempDir()
		cf, err := NewCaptureFile(dir, &v1.DataCaptureMetadata{Type: v1.DataType_DATA_TYPE_BINARY_SENSOR})
		test.That(t, err, test.ShouldBeNil)

		const n = 5
		payloads := make([][]byte, n)
		for i := range payloads {
			payloads[i] = []byte(fmt.Sprintf("payload-%d", i))
			err = cf.WriteNext(&v1.SensorData{
				Metadata: &v1.SensorMetadata{},
				Data:     &v1.SensorData_Binary{Binary: payloads[i]},
			})
			test.That(t, err, test.ShouldBeNil)
		}

		cf.Reset()
		for i := range payloads {
			_, payloadLen, r, err := cf.BinaryPayloadReader()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, payloadLen, test.ShouldEqual, int64(len(payloads[i])))
			got, err := io.ReadAll(r)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, got, test.ShouldResemble, payloads[i])
		}
		_, _, _, err = cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldEqual, io.EOF)
	})

	// writeRawCaptureFile builds a capture file with a manually constructed SensorData
	// body, allowing tests to control field ordering and unknown wire types.
	writeRawCaptureFile := func(t *testing.T, body []byte) *CaptureFile {
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

	t.Run("field 3 before field 1 (reverse ordering)", func(t *testing.T) {
		payload := []byte("reverse field order payload")
		metaBytes, err := proto.Marshal(&v1.SensorMetadata{})
		test.That(t, err, test.ShouldBeNil)

		// field 3 (binary) first, then field 1 (SensorMetadata).
		var body []byte
		body = protowire.AppendTag(body, 3, protowire.BytesType)
		body = protowire.AppendBytes(body, payload)
		body = protowire.AppendTag(body, 1, protowire.BytesType)
		body = protowire.AppendBytes(body, metaBytes)

		cf := writeRawCaptureFile(t, body)
		meta, _, r, err := cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, meta, test.ShouldNotBeNil)
		got, err := io.ReadAll(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, payload)
	})

	t.Run("unknown non-bytes wire type field is skipped", func(t *testing.T) {
		// Simulate a file written by a future server that added a varint field.
		payload := []byte("payload after unknown varint field")
		metaBytes, err := proto.Marshal(&v1.SensorMetadata{})
		test.That(t, err, test.ShouldBeNil)

		var body []byte
		body = protowire.AppendTag(body, 1, protowire.BytesType)
		body = protowire.AppendBytes(body, metaBytes)
		// Unknown field 99 with varint wire type — must be skipped gracefully.
		body = protowire.AppendTag(body, 99, protowire.VarintType)
		body = protowire.AppendVarint(body, 42)
		body = protowire.AppendTag(body, 3, protowire.BytesType)
		body = protowire.AppendBytes(body, payload)

		cf := writeRawCaptureFile(t, body)
		_, _, r, err := cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldBeNil)
		got, err := io.ReadAll(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, payload)
	})

	t.Run("no binary field returns error", func(t *testing.T) {
		dir := t.TempDir()
		cf, err := NewCaptureFile(dir, &v1.DataCaptureMetadata{Type: v1.DataType_DATA_TYPE_TABULAR_SENSOR})
		test.That(t, err, test.ShouldBeNil)
		err = cf.WriteNext(&v1.SensorData{
			Metadata: &v1.SensorMetadata{},
			Data:     &v1.SensorData_Struct{Struct: &structpb.Struct{}},
		})
		test.That(t, err, test.ShouldBeNil)

		cf.Reset()
		_, _, _, err = cf.BinaryPayloadReader()
		test.That(t, err, test.ShouldNotBeNil)
	})
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
