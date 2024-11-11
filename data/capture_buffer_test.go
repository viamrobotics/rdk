package data

import (
	"crypto/sha1"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "go.viam.com/api/app/datasync/v1"
	armPB "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
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

// TODO: rewrite tests.
func TestCaptureQueue(t *testing.T) {
	maxFileSize := 50
	tests := []struct {
		name               string
		dataType           v1.DataType
		pushCount          int
		expCompleteFiles   int
		expInProgressFiles int
	}{
		{
			name:               "Files that are still being written to should have the InProgressFileExt extension.",
			dataType:           v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			pushCount:          1,
			expCompleteFiles:   0,
			expInProgressFiles: 1,
		},
		{
			name:               "Pushing N binary data should write N files.",
			dataType:           v1.DataType_DATA_TYPE_BINARY_SENSOR,
			pushCount:          2,
			expCompleteFiles:   2,
			expInProgressFiles: 0,
		},
		{
			name:     "Pushing > MaxFileSize + 1 worth of struct data should write two files.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			// MaxFileSize / size(structSensorData) = ceil(50 / 19) = 3 readings per file => 2 files, one in progress
			pushCount:          4,
			expCompleteFiles:   1,
			expInProgressFiles: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			md := &v1.DataCaptureMetadata{Type: tc.dataType}
			sut := NewCaptureBuffer(tmpDir, md, int64(maxFileSize))
			var pushValue *v1.SensorData
			if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
				pushValue = binarySensorData
			} else {
				pushValue = structSensorData
			}

			for i := 0; i < tc.pushCount; i++ {
				err := sut.Write(pushValue)
				test.That(t, err, test.ShouldBeNil)
			}

			dcFiles, inProgressFiles := getCaptureFiles(tmpDir)
			test.That(t, len(dcFiles), test.ShouldEqual, tc.expCompleteFiles)
			test.That(t, len(inProgressFiles), test.ShouldEqual, tc.expInProgressFiles)

			// Test that sync is respected: after closing, all files should no longer be in progress.
			err := sut.Flush()
			test.That(t, err, test.ShouldBeNil)
			completeFiles, remainingProgFiles := getCaptureFiles(tmpDir)
			test.That(t, len(remainingProgFiles), test.ShouldEqual, 0)

			// Validate correct values were written.
			var actCaptures []*v1.SensorData
			for i := 0; i < len(completeFiles); i++ {
				c, err := SensorDataFromCaptureFilePath(completeFiles[i])
				test.That(t, err, test.ShouldBeNil)
				actCaptures = append(actCaptures, c...)
			}
			test.That(t, len(actCaptures), test.ShouldEqual, tc.pushCount)
			for _, capture := range actCaptures {
				if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
					test.That(t, capture.GetBinary(), test.ShouldNotBeNil)
					test.That(t, capture.GetBinary(), test.ShouldResemble, binarySensorData.GetBinary())
				}
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					test.That(t, capture.GetStruct(), test.ShouldNotBeNil)
					test.That(t, capture.GetStruct(), test.ShouldResemble, structSensorData.GetStruct())
				}
			}
		})
	}
}

// Tests reading the data written by a CaptureBuffer.
func TestCaptureBufferReader(t *testing.T) {
	t.Run("tabular data", func(t *testing.T) {
		type testCase struct {
			name             string
			resourceName     resource.Name
			additionalParams map[string]string
			tags             []string
			methodName       string
			readings         []*structpb.Struct
		}

		aStruct, err := structpb.NewStruct(map[string]interface{}{"im": "a struct"})
		test.That(t, err, test.ShouldBeNil)
		aList, err := structpb.NewList([]interface{}{"I'm", "a", "list"})
		test.That(t, err, test.ShouldBeNil)

		armJointPositionsReading1, err := protoutils.StructToStructPbIgnoreOmitEmpty(armPB.GetJointPositionsResponse{
			Positions: &armPB.JointPositions{Values: []float64{1.0}},
		})
		test.That(t, err, test.ShouldBeNil)
		armJointPositionsReading2, err := protoutils.StructToStructPbIgnoreOmitEmpty(armPB.GetJointPositionsResponse{
			Positions: &armPB.JointPositions{Values: []float64{2.0}},
		})
		test.That(t, err, test.ShouldBeNil)
		armJointPositionsReading3, err := protoutils.StructToStructPbIgnoreOmitEmpty(armPB.GetJointPositionsResponse{
			Positions: &armPB.JointPositions{Values: []float64{3.0}},
		})
		test.That(t, err, test.ShouldBeNil)
		// TODO: Add joint position
		testCases := []testCase{
			{
				name:             "sensor.Readings",
				resourceName:     resource.NewName(resource.APINamespaceRDK.WithComponentType("sensor"), "my-sensor"),
				additionalParams: map[string]string{"some": "params"},
				tags:             []string{"my", "tags"},
				readings: []*structpb.Struct{
					{
						Fields: map[string]*structpb.Value{
							"readings": structpb.NewStructValue(
								&structpb.Struct{Fields: map[string]*structpb.Value{
									"speed":   structpb.NewNumberValue(5),
									"temp":    structpb.NewNumberValue(30),
									"engaged": structpb.NewBoolValue(true),
									"name":    structpb.NewStringValue("my cool sensor"),
									"struct":  structpb.NewStructValue(aStruct),
									"list":    structpb.NewListValue(aList),
								}},
							),
						},
					},
					{
						Fields: map[string]*structpb.Value{
							"readings": structpb.NewStructValue(
								&structpb.Struct{Fields: map[string]*structpb.Value{
									"speed": structpb.NewNumberValue(6),
								}},
							),
						},
					},
					{
						Fields: map[string]*structpb.Value{
							"readings": structpb.NewStructValue(
								&structpb.Struct{Fields: map[string]*structpb.Value{
									"speed": structpb.NewNumberValue(7),
								}},
							),
						},
					},
				},
				methodName: "Readings",
			},
			{
				name:             "arm.JointPositions",
				resourceName:     resource.NewName(resource.APINamespaceRDK.WithComponentType("arm"), "my-arm"),
				additionalParams: map[string]string{"some": "params"},
				tags:             []string{"my", "tags"},
				readings: []*structpb.Struct{
					armJointPositionsReading1,
					armJointPositionsReading2,
					armJointPositionsReading3,
				},
				methodName: "JointPositions",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				tmpDir := t.TempDir()
				methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(tc.additionalParams)
				test.That(t, err, test.ShouldBeNil)

				readImageCaptureMetadata := BuildCaptureMetadata(
					tc.resourceName.API,
					tc.resourceName.ShortName(),
					tc.methodName,
					tc.additionalParams,
					methodParams,
					tc.tags,
				)

				test.That(t, readImageCaptureMetadata, test.ShouldResemble, &v1.DataCaptureMetadata{
					ComponentName:    tc.resourceName.ShortName(),
					ComponentType:    tc.resourceName.API.String(),
					MethodName:       tc.methodName,
					MethodParameters: methodParams,
					Tags:             tc.tags,
					FileExtension:    ".dat",
					Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
				})

				b := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(256*1024))

				// Path() is the same as the first paramenter passed to NewCaptureBuffer
				test.That(t, b.Path(), test.ShouldResemble, tmpDir)

				now := time.Now()
				timeRequested := timestamppb.New(now.UTC())
				timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
				msg := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Struct{
						Struct: tc.readings[0],
					},
				}
				test.That(t, b.Write(msg), test.ShouldBeNil)
				test.That(t, b.Flush(), test.ShouldBeNil)
				dirEntries, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(dirEntries), test.ShouldEqual, 1)
				test.That(t, filepath.Ext(dirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
				f, err := os.Open(filepath.Join(b.Path(), dirEntries[0].Name()))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f.Close()) }()

				cf, err := NewCaptureFile(f)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				sd, err := cf.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd, test.ShouldResemble, msg)

				_, err = cf.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)

				now = time.Now()
				timeRequested = timestamppb.New(now.UTC())
				timeReceived = timestamppb.New(now.Add(time.Millisecond).UTC())
				msg2 := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Struct{
						Struct: tc.readings[1],
					},
				}
				test.That(t, b.Write(msg2), test.ShouldBeNil)

				now = time.Now()
				timeRequested = timestamppb.New(now.UTC())
				timeReceived = timestamppb.New(now.Add(time.Millisecond).UTC())
				msg3 := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Struct{
						Struct: tc.readings[2],
					},
				}
				test.That(t, b.Write(msg3), test.ShouldBeNil)

				dirEntries2, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				// msg 2 and msg 3 should be in the newly written capture file
				test.That(t, len(dirEntries2), test.ShouldEqual, 2)
				var hasProgFile bool
				for _, d := range dirEntries2 {
					if filepath.Ext(d.Name()) == InProgressCaptureFileExt {
						hasProgFile = true
					}
				}
				test.That(t, hasProgFile, test.ShouldBeTrue)

				test.That(t, b.Flush(), test.ShouldBeNil)

				dirEntries3, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(dirEntries3), test.ShouldEqual, 2)
				hasProgFile = false
				for _, d := range dirEntries3 {
					if filepath.Ext(d.Name()) == InProgressCaptureFileExt {
						hasProgFile = true
					}
				}
				test.That(t, hasProgFile, test.ShouldBeFalse)

				f2, err := os.Open(filepath.Join(b.Path(), dirEntries3[1].Name()))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f2.Close()) }()

				cf2, err := NewCaptureFile(f2)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				sd2, err := cf2.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd2, test.ShouldResemble, msg2)

				sd3, err := cf2.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd3, test.ShouldResemble, msg3)

				_, err = cf2.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)
			})
		}
	})

	t.Run("binary data", func(t *testing.T) {
		type testCase struct {
			name              string
			resourceName      resource.Name
			additionalParams  map[string]string
			tags              []string
			methodName        string
			expectedExtension string
		}
		testCases := []testCase{
			{
				name:             readImage,
				resourceName:     resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams: map[string]string{"some": "params"},
				tags:             []string{"my", "tags"},
				methodName:       readImage,
			},
			{
				name:              readImage + " with jpeg mime type in additional params",
				resourceName:      resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams:  map[string]string{"mime_type": rutils.MimeTypeJPEG},
				tags:              []string{"", "tags"},
				expectedExtension: ".jpeg",
				methodName:        readImage,
			},
			{
				name:              readImage + " with png mime type in additional params",
				resourceName:      resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams:  map[string]string{"mime_type": rutils.MimeTypePNG},
				tags:              []string{"", "tags"},
				expectedExtension: ".png",
				methodName:        readImage,
			},
			{
				name:              readImage + " with pcd mime type in additional params",
				resourceName:      resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams:  map[string]string{"mime_type": rutils.MimeTypePCD},
				tags:              []string{"", "tags"},
				expectedExtension: ".pcd",
				methodName:        readImage,
			},
			{
				name:              nextPointCloud,
				resourceName:      resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams:  map[string]string{"some": "params"},
				tags:              []string{"my", "tags"},
				methodName:        nextPointCloud,
				expectedExtension: ".pcd",
			},
			{
				name:             GetImages,
				resourceName:     resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam"),
				additionalParams: map[string]string{"some": "params"},
				tags:             []string{"my", "tags"},
				methodName:       GetImages,
			},
			{
				name:             pointCloudMap,
				resourceName:     resource.NewName(resource.APINamespaceRDK.WithServiceType("slam"), "my-slam"),
				additionalParams: map[string]string{"some": "params"},
				tags:             []string{"my", "tags"},
				// NOTE: The fact that this doesn't get a .pcd extension is inconsistent with
				// how camera.NextPointCloud is handled
				methodName: pointCloudMap,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				tmpDir := t.TempDir()
				methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(tc.additionalParams)
				test.That(t, err, test.ShouldBeNil)

				readImageCaptureMetadata := BuildCaptureMetadata(
					tc.resourceName.API,
					tc.resourceName.ShortName(),
					tc.methodName,
					tc.additionalParams,
					methodParams,
					tc.tags,
				)

				test.That(t, readImageCaptureMetadata, test.ShouldResemble, &v1.DataCaptureMetadata{
					ComponentName:    tc.resourceName.ShortName(),
					ComponentType:    tc.resourceName.API.String(),
					MethodName:       tc.methodName,
					MethodParameters: methodParams,
					Tags:             tc.tags,
					FileExtension:    tc.expectedExtension,
					Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
				})

				b := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(256*1024))

				// Path() is the same as the first paramenter passed to NewCaptureBuffer
				test.That(t, b.Path(), test.ShouldResemble, tmpDir)

				// flushing before Write() doesn't create any files
				test.That(t, b.Flush(), test.ShouldBeNil)
				firstDirEntries, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, firstDirEntries, test.ShouldBeEmpty)

				// writing empty sensor data returns an error
				test.That(t, b.Write(nil), test.ShouldBeError, errors.New("proto: Marshal called with nil"))

				// flushing after this error occures, behaves the same as if no write had occurred
				// current behavior is likely a bug
				test.That(t, b.Flush(), test.ShouldBeNil)
				firstDirEntries, err = os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(firstDirEntries), test.ShouldEqual, 1)
				test.That(t, filepath.Ext(firstDirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
				f, err := os.Open(filepath.Join(b.Path(), firstDirEntries[0].Name()))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f.Close()) }()

				cf, err := NewCaptureFile(f)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				sd, err := cf.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)
				test.That(t, sd, test.ShouldBeNil)

				now := time.Now()
				timeRequested := timestamppb.New(now.UTC())
				timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
				msg := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Binary{
						Binary: []byte("this is fake binary data"),
					},
				}
				test.That(t, b.Write(msg), test.ShouldBeNil)
				test.That(t, b.Flush(), test.ShouldBeNil)
				secondDirEntries, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(secondDirEntries), test.ShouldEqual, 2)
				var newFileName string
				for _, de := range secondDirEntries {
					if de.Name() != firstDirEntries[0].Name() {
						newFileName = de.Name()
						break
					}
				}
				test.That(t, newFileName, test.ShouldNotBeEmpty)
				test.That(t, filepath.Ext(newFileName), test.ShouldResemble, CompletedCaptureFileExt)
				f2, err := os.Open(filepath.Join(b.Path(), newFileName))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f2.Close()) }()

				cf2, err := NewCaptureFile(f2)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				sd2, err := cf2.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd2, test.ShouldResemble, msg)

				_, err = cf2.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)

				timeRequested = timestamppb.New(now.UTC())
				timeReceived = timestamppb.New(now.Add(time.Millisecond).UTC())
				msg3 := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Binary{
						Binary: []byte("msg2"),
					},
				}

				test.That(t, b.Write(msg3), test.ShouldBeNil)

				timeRequested = timestamppb.New(now.UTC())
				timeReceived = timestamppb.New(now.Add(time.Millisecond).UTC())
				msg4 := &v1.SensorData{
					Metadata: &v1.SensorMetadata{
						TimeRequested: timeRequested,
						TimeReceived:  timeReceived,
					},
					Data: &v1.SensorData_Binary{
						Binary: []byte("msg3"),
					},
				}
				// Every binary data written becomes a new data capture file
				test.That(t, b.Write(msg4), test.ShouldBeNil)
				test.That(t, b.Flush(), test.ShouldBeNil)
				thirdDirEntries, err := os.ReadDir(b.Path())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(thirdDirEntries), test.ShouldEqual, 4)

				var newFileNames []string
				for _, de := range thirdDirEntries {
					if de.Name() != firstDirEntries[0].Name() && de.Name() != newFileName {
						newFileNames = append(newFileNames, de.Name())
					}
				}
				test.That(t, len(newFileNames), test.ShouldEqual, 2)

				// 3rd file
				f3, err := os.Open(filepath.Join(b.Path(), newFileNames[0]))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f3.Close()) }()
				cf3, err := NewCaptureFile(f3)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf3.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)
				sd3, err := cf3.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd3, test.ShouldResemble, msg3)
				_, err = cf3.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)

				// 4th file
				f4, err := os.Open(filepath.Join(b.Path(), newFileNames[1]))
				test.That(t, err, test.ShouldBeNil)
				defer func() { utils.UncheckedError(f4.Close()) }()
				cf4, err := NewCaptureFile(f4)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cf4.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)
				sd4, err := cf4.ReadNext()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sd4, test.ShouldResemble, msg4)
				_, err = cf4.ReadNext()
				test.That(t, err, test.ShouldBeError, io.EOF)
			})
		}
	})

	t.Run("binary data with file extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		name := resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam")
		method := readImage
		additionalParams := map[string]string{"mime_type": rutils.MimeTypeJPEG}
		tags := []string{"my", "tags"}
		methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(additionalParams)
		test.That(t, err, test.ShouldBeNil)

		readImageCaptureMetadata := BuildCaptureMetadata(
			name.API,
			name.ShortName(),
			method,
			additionalParams,
			methodParams,
			tags,
		)

		test.That(t, readImageCaptureMetadata, test.ShouldResemble, &v1.DataCaptureMetadata{
			ComponentName:    "my-cam",
			ComponentType:    "rdk:component:camera",
			MethodName:       readImage,
			MethodParameters: methodParams,
			Tags:             tags,
			Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			FileExtension:    ".jpeg",
		})

		b := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(256*1024))

		// Path() is the same as the first paramenter passed to NewCaptureBuffer
		test.That(t, b.Path(), test.ShouldResemble, tmpDir)
		test.That(t, b.metaData, test.ShouldResemble, readImageCaptureMetadata)

		now := time.Now()
		timeRequested := timestamppb.New(now.UTC())
		timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
		msg := &v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Binary{
				Binary: []byte("this is a fake image"),
			},
		}
		test.That(t, b.Write(msg), test.ShouldBeNil)
		test.That(t, b.Flush(), test.ShouldBeNil)
		dirEntries, err := os.ReadDir(b.Path())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(dirEntries), test.ShouldEqual, 1)
		test.That(t, filepath.Ext(dirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
		f, err := os.Open(filepath.Join(b.Path(), dirEntries[0].Name()))
		test.That(t, err, test.ShouldBeNil)
		defer func() { utils.UncheckedError(f.Close()) }()

		cf2, err := NewCaptureFile(f)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

		sd2, err := cf2.ReadNext()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sd2, test.ShouldResemble, msg)

		_, err = cf2.ReadNext()
		test.That(t, err, test.ShouldBeError, io.EOF)
	})
}

func BenchmarkChunked(b *testing.B) {
	type testCase struct {
		name string
		data []byte
	}
	eightKBFilled := make([]byte, 1024*8)
	for i := range eightKBFilled {
		eightKBFilled[i] = uint8(i % 256)
	}

	oneMbFilled := make([]byte, 1024*1000)
	for i := range eightKBFilled {
		oneMbFilled[i] = uint8(i % 256)
	}

	eightMbFilled := make([]byte, 1024*1000*8)
	for i := range eightMbFilled {
		eightMbFilled[i] = uint8(i % 256)
	}

	tcs := []testCase{
		{"empty data", []byte{}},
		{"small data", []byte("this is a fake image")},
		{"8kb empty", make([]byte, 1024*8)},
		{"8kb filled", eightKBFilled},
		{"1mb empty", make([]byte, 1024*1000)},
		{"1mb filled", oneMbFilled},
		{"8mb empty", make([]byte, 1024*1000*8)},
		{"8mb filled", eightMbFilled},
	}

	for _, tc := range tcs {
		s := sha1.New()
		_, err := s.Write(tc.data)
		test.That(b, err, test.ShouldBeNil)
		expectedHash := s.Sum(nil)
		tmpDir := b.TempDir()
		name := resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam")
		additionalParams := map[string]string{"mime_type": rutils.MimeTypeJPEG, "test": "1"}
		methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(additionalParams)
		test.That(b, err, test.ShouldBeNil)

		readImageCaptureMetadata := BuildCaptureMetadata(
			name.API,
			name.ShortName(),
			readImage,
			additionalParams,
			methodParams,
			[]string{"my", "tags"},
		)

		now := time.Now()
		timeRequested := timestamppb.New(now.UTC())
		timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
		msg := &v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Binary{
				Binary: tc.data,
			},
		}

		buf := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(4*1024))

		// Path() is the same as the first paramenter passed to NewCaptureBuffer
		test.That(b, buf.Path(), test.ShouldResemble, tmpDir)
		test.That(b, buf.metaData, test.ShouldResemble, readImageCaptureMetadata)

		test.That(b, buf.Write(msg), test.ShouldBeNil)
		test.That(b, buf.Flush(), test.ShouldBeNil)
		dirEntries, err := os.ReadDir(buf.Path())
		test.That(b, err, test.ShouldBeNil)
		test.That(b, len(dirEntries), test.ShouldEqual, 1)
		test.That(b, filepath.Ext(dirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
		f, err := os.Open(filepath.Join(buf.Path(), dirEntries[0].Name()))
		test.That(b, err, test.ShouldBeNil)
		b.Cleanup(func() { test.That(b, f.Close(), test.ShouldBeNil) })

		b.ResetTimer()
		b.Run("chunked "+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ret, err := f.Seek(0, io.SeekStart)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, ret, test.ShouldEqual, 0)
				cf2, err := NewCaptureFile(f)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				var md v1.SensorMetadata
				r, err := cf2.BinaryReader(&md)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, r, test.ShouldNotBeNil)
				test.That(b, &md, test.ShouldResemble, msg.GetMetadata())
				data := make([]byte, 4064)
				h := sha1.New()
				for {
					n, err := r.Read(data)
					if errors.Is(err, io.EOF) {
						break
					}
					test.That(b, err, test.ShouldBeNil)
					_, err = h.Write(data[:n])
					test.That(b, err, test.ShouldBeNil)
				}
				actualHash := h.Sum(nil)
				test.That(b, actualHash, test.ShouldResemble, expectedHash)
			}
		})
	}
}

func BenchmarkNonChunked(b *testing.B) {
	type testCase struct {
		name string
		data []byte
	}
	eightKBFilled := make([]byte, 1024*8)
	for i := range eightKBFilled {
		eightKBFilled[i] = uint8(i % 256)
	}

	oneMbFilled := make([]byte, 1024*1000)
	for i := range eightKBFilled {
		oneMbFilled[i] = uint8(i % 256)
	}

	eightMbFilled := make([]byte, 1024*1000*8)
	for i := range eightMbFilled {
		eightMbFilled[i] = uint8(i % 256)
	}

	tcs := []testCase{
		{"empty data", []byte{}},
		{"small data", []byte("this is a fake image")},
		{"8kb empty", make([]byte, 1024*8)},
		{"8kb filled", eightKBFilled},
		{"1mb empty", make([]byte, 1024*1000)},
		{"1mb filled", oneMbFilled},
		{"8mb empty", make([]byte, 1024*1000*8)},
		{"8mb filled", eightMbFilled},
	}

	for _, tc := range tcs {
		s := sha1.New()
		_, err := s.Write(tc.data)
		test.That(b, err, test.ShouldBeNil)
		expectedHash := s.Sum(nil)
		tmpDir := b.TempDir()
		name := resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam")
		additionalParams := map[string]string{"mime_type": rutils.MimeTypeJPEG, "test": "1"}
		methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(additionalParams)
		test.That(b, err, test.ShouldBeNil)

		readImageCaptureMetadata := BuildCaptureMetadata(
			name.API,
			name.ShortName(),
			readImage,
			additionalParams,
			methodParams,
			[]string{"my", "tags"},
		)

		now := time.Now()
		timeRequested := timestamppb.New(now.UTC())
		timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
		msg := &v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Binary{
				Binary: tc.data,
			},
		}

		buf := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(4*1024))

		test.That(b, buf.Path(), test.ShouldResemble, tmpDir)
		test.That(b, buf.metaData, test.ShouldResemble, readImageCaptureMetadata)

		test.That(b, buf.Write(msg), test.ShouldBeNil)
		test.That(b, buf.Flush(), test.ShouldBeNil)
		dirEntries, err := os.ReadDir(buf.Path())
		test.That(b, err, test.ShouldBeNil)
		test.That(b, len(dirEntries), test.ShouldEqual, 1)
		test.That(b, filepath.Ext(dirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
		f, err := os.Open(filepath.Join(buf.Path(), dirEntries[0].Name()))
		test.That(b, err, test.ShouldBeNil)
		b.Cleanup(func() { test.That(b, f.Close(), test.ShouldBeNil) })
		b.ResetTimer()
		b.Run("non chunked "+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ret, err := f.Seek(0, io.SeekStart)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, ret, test.ShouldEqual, 0)
				cf2, err := NewCaptureFile(f)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

				next, err := cf2.ReadNext()
				test.That(b, err, test.ShouldBeNil)
				test.That(b, next.GetMetadata(), test.ShouldResemble, msg.GetMetadata())
				h := sha1.New()
				_, err = h.Write(next.GetBinary())
				test.That(b, err, test.ShouldBeNil)
				actualHash := h.Sum(nil)
				test.That(b, actualHash, test.ShouldResemble, expectedHash)
			}
		})
	}
}

func FuzzBinaryReader(f *testing.F) {
	eightKBFilled := make([]byte, 1024*8)
	for i := range eightKBFilled {
		eightKBFilled[i] = uint8(i % 256)
	}

	eightMbFilled := make([]byte, 1024*1000*8)
	for i := range eightMbFilled {
		eightMbFilled[i] = uint8(i % 256)
	}

	tcs := [][]byte{
		{},
		[]byte("this is a fake image"),
		make([]byte, 1024*8),
		eightKBFilled,
		make([]byte, 1024*1000*8),
	}

	for _, tc := range tcs {
		f.Add(tc)
	}
	f.Fuzz(func(t *testing.T, binary []byte) {
		tmpDir := t.TempDir()
		name := resource.NewName(resource.APINamespaceRDK.WithComponentType("camera"), "my-cam")
		additionalParams := map[string]string{"mime_type": rutils.MimeTypeJPEG, "test": "1"}
		methodParams, err := rprotoutils.ConvertStringMapToAnyPBMap(additionalParams)
		test.That(t, err, test.ShouldBeNil)

		readImageCaptureMetadata := BuildCaptureMetadata(
			name.API,
			name.ShortName(),
			readImage,
			additionalParams,
			methodParams,
			[]string{"my", "tags"},
		)

		now := time.Now()
		timeRequested := timestamppb.New(now.UTC())
		timeReceived := timestamppb.New(now.Add(time.Millisecond).UTC())
		msg := &v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Binary{
				Binary: binary,
			},
		}

		b := NewCaptureBuffer(tmpDir, readImageCaptureMetadata, int64(4*1024))

		// Path() is the same as the first paramenter passed to NewCaptureBuffer
		test.That(t, b.Path(), test.ShouldResemble, tmpDir)
		test.That(t, b.metaData, test.ShouldResemble, readImageCaptureMetadata)

		test.That(t, b.Write(msg), test.ShouldBeNil)
		test.That(t, b.Flush(), test.ShouldBeNil)
		dirEntries, err := os.ReadDir(b.Path())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(dirEntries), test.ShouldEqual, 1)
		test.That(t, filepath.Ext(dirEntries[0].Name()), test.ShouldResemble, CompletedCaptureFileExt)
		f, err := os.Open(filepath.Join(b.Path(), dirEntries[0].Name()))
		test.That(t, err, test.ShouldBeNil)
		defer func() { test.That(t, f.Close(), test.ShouldBeNil) }()

		cf2, err := NewCaptureFile(f)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cf2.ReadMetadata(), test.ShouldResemble, readImageCaptureMetadata)

		var md v1.SensorMetadata
		r, err := cf2.BinaryReader(&md)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, r, test.ShouldNotBeNil)
		test.That(t, &md, test.ShouldResemble, msg.GetMetadata())
		data, err := io.ReadAll(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, msg.GetBinary())
	})
}

//nolint
func getCaptureFiles(dir string) (dcFiles, progFiles []string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == CompletedCaptureFileExt {
			dcFiles = append(dcFiles, path)
		}
		if filepath.Ext(path) == InProgressCaptureFileExt {
			progFiles = append(progFiles, path)
		}
		return nil
	})
	return dcFiles, progFiles
}
