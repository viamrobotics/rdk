package data

import (
	"errors"
	"testing"
	"time"

	v1 "go.viam.com/api/app/data/v1"
	datasyncPB "go.viam.com/api/app/datasync/v1"
	commonPB "go.viam.com/api/common/v1"
	armPB "go.viam.com/api/component/arm/v1"
	cameraPB "go.viam.com/api/component/camera/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	tu "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

func TestNewBinaryCaptureResult(t *testing.T) {
	timeRequested := time.Now()
	timeReceived := time.Now()
	ts := Timestamps{TimeRequested: timeRequested, TimeReceived: timeReceived}
	type testCase struct {
		input       CaptureResult
		output      CaptureResult
		validateErr error
	}
	confidence := 0.1
	emptyBinaries := []Binary{}
	singleSimpleBinaries := []Binary{{Payload: []byte("hi there")}}
	singleSimpleBinariesWithMimeType := []Binary{
		{
			Payload:  []byte("hi there"),
			MimeType: MimeTypeImageJpeg,
		},
	}
	singleComplexBinaries := []Binary{
		{
			Payload:  []byte("hi there"),
			MimeType: MimeTypeImageJpeg,
			Annotations: Annotations{
				Classifications: []Classification{
					{Label: "no-confidence"},
					{Label: "confidence", Confidence: &confidence},
				},
				BoundingBoxes: []BoundingBox{
					{
						Label:          "no-confidence",
						XMinNormalized: 1,
						XMaxNormalized: 2,
						YMinNormalized: 3,
						YMaxNormalized: 4,
					},
					{
						Label:          "confidence",
						Confidence:     &confidence,
						XMinNormalized: 5,
						XMaxNormalized: 6,
						YMinNormalized: 7,
						YMaxNormalized: 8,
					},
				},
			},
		},
		{
			Payload:  []byte("hi too am here here"),
			MimeType: MimeTypeImageJpeg,
			Annotations: Annotations{
				Classifications: []Classification{
					{Label: "something completely different"},
				},
			},
		},
	}

	multipleComplexBinaries := []Binary{
		{
			Payload:  []byte("hi there"),
			MimeType: MimeTypeImageJpeg,
			Annotations: Annotations{
				Classifications: []Classification{
					{Label: "no-confidence"},
					{Label: "confidence", Confidence: &confidence},
				},
				BoundingBoxes: []BoundingBox{
					{
						Label:          "no-confidence",
						XMinNormalized: 1,
						XMaxNormalized: 2,
						YMinNormalized: 3,
						YMaxNormalized: 4,
					},
					{
						Label:          "confidence",
						Confidence:     &confidence,
						XMinNormalized: 5,
						XMaxNormalized: 6,
						YMinNormalized: 7,
						YMaxNormalized: 8,
					},
				},
			},
		},
		{
			Payload:  []byte("hi too am here here"),
			MimeType: MimeTypeImageJpeg,
			Annotations: Annotations{
				Classifications: []Classification{
					{Label: "something completely different"},
				},
			},
		},
	}
	tcs := []testCase{
		{
			input:       NewBinaryCaptureResult(ts, nil),
			output:      CaptureResult{Type: CaptureTypeBinary, Timestamps: ts},
			validateErr: errors.New("binary result must have non empty binary data"),
		},
		{
			input: NewBinaryCaptureResult(ts, emptyBinaries),
			output: CaptureResult{
				Type:       CaptureTypeBinary,
				Timestamps: ts,
				Binaries:   emptyBinaries,
			},
			validateErr: errors.New("binary result must have non empty binary data"),
		},
		{
			input: NewBinaryCaptureResult(ts, singleSimpleBinaries),
			output: CaptureResult{
				Type:       CaptureTypeBinary,
				Timestamps: ts,
				Binaries:   singleSimpleBinaries,
			},
		},
		{
			input: NewBinaryCaptureResult(ts, singleSimpleBinariesWithMimeType),
			output: CaptureResult{
				Type:       CaptureTypeBinary,
				Timestamps: ts,
				Binaries:   singleSimpleBinariesWithMimeType,
			},
		},
		{
			input: NewBinaryCaptureResult(ts, singleComplexBinaries),
			output: CaptureResult{
				Type:       CaptureTypeBinary,
				Timestamps: ts,
				Binaries:   singleComplexBinaries,
			},
		},
		{
			input: NewBinaryCaptureResult(ts, multipleComplexBinaries),
			output: CaptureResult{
				Type:       CaptureTypeBinary,
				Timestamps: ts,
				Binaries:   multipleComplexBinaries,
			},
		},
	}
	for i, tc := range tcs {
		t.Logf("index: %d", i)

		// confirm input resembles output
		test.That(t, tc.input, test.ShouldResemble, tc.output)

		// confirm input conforms to validation expectations
		if tc.validateErr != nil {
			test.That(t, tc.input.Validate(), test.ShouldBeError, tc.validateErr)
			continue
		}
		test.That(t, tc.input.Validate(), test.ShouldBeNil)

		// confirm input conforms to ToProto expectations
		proto := tc.input.ToProto()
		test.That(t, len(proto), test.ShouldEqual, len(tc.input.Binaries))
		for j := range tc.input.Binaries {
			test.That(t, proto[j].Metadata, test.ShouldResemble, &datasyncPB.SensorMetadata{
				TimeRequested: timestamppb.New(timeRequested.UTC()),
				TimeReceived:  timestamppb.New(timeReceived.UTC()),
				MimeType:      tc.input.Binaries[j].MimeType.ToProto(),
				Annotations:   tc.input.Binaries[j].Annotations.ToProto(),
			})

			test.That(t, proto[j].Data, test.ShouldResemble, &datasyncPB.SensorData_Binary{
				Binary: tc.input.Binaries[j].Payload,
			})
		}
	}
}

func TestNewTabularCaptureResultReadings(t *testing.T) {
	now := time.Now()
	type testCase struct {
		input  map[string]interface{}
		output *structpb.Struct
		err    error
	}
	firstReading := map[string]any{
		"hi":    1,
		"there": 1.2,
		"friend": []any{
			map[string]any{
				"weird": "stuff",
				"even":  "stranger",
			},
			1,
			true,
			"20 mickey mouse",
			[]any{3.3, 9.9},
			[]byte{1, 2, 3},
		},
	}
	tcs := []testCase{
		{
			input:  nil,
			output: tu.ToStructPBStruct(t, map[string]any{"readings": map[string]any{}}),
		},
		{
			input:  firstReading,
			output: tu.ToStructPBStruct(t, map[string]any{"readings": firstReading}),
		},
		{
			input: map[string]any{"invalid_type": []float64{3.3, 9.9}},
			err:   errors.New("proto: invalid type: []float64"),
		},
	}

	for i, tc := range tcs {
		t.Logf("index: %d", i)
		res, err := NewTabularCaptureResultReadings(now, tc.input)
		if tc.err != nil {
			test.That(t, err, test.ShouldBeError, tc.err)
			continue
		}

		test.That(t, err, test.ShouldBeNil)
		verifyStruct(t, res, now, tc.output)
	}
}

func TestNewTabularCaptureResult(t *testing.T) {
	now := time.Now()
	type testCase struct {
		input  any
		output *structpb.Struct
		err    error
	}
	tcs := []testCase{
		{
			input: nil,
			err:   errors.New("unable to convert interface <nil> to a form acceptable to structpb.NewStruct: no data passed in"),
		},
		{
			input: armPB.GetEndPositionResponse{Pose: &commonPB.Pose{X: 1, Y: 2, Z: 3, OX: 4, OY: 5, OZ: 6, Theta: 7}},
			output: tu.ToStructPBStruct(t, map[string]any{"pose": map[string]any{
				"x":     1,
				"y":     2,
				"z":     3,
				"o_x":   4,
				"o_y":   5,
				"o_z":   6,
				"theta": 7,
			}}),
		},
		{
			input: &armPB.GetEndPositionResponse{Pose: &commonPB.Pose{X: 1, Y: 2, Z: 3, OX: 4, OY: 5, OZ: 6, Theta: 7}},
			output: tu.ToStructPBStruct(t, map[string]any{"pose": map[string]any{
				"x":     1,
				"y":     2,
				"z":     3,
				"o_x":   4,
				"o_y":   5,
				"o_z":   6,
				"theta": 7,
			}}),
		},
	}

	for i, tc := range tcs {
		t.Logf("index: %d", i)
		res, err := NewTabularCaptureResult(now, tc.input)
		if tc.err != nil {
			test.That(t, err, test.ShouldBeError, tc.err)
			continue
		}
		test.That(t, err, test.ShouldBeNil)
		verifyStruct(t, res, now, tc.output)
	}
}

func verifyStruct(t *testing.T, res CaptureResult, now time.Time, output *structpb.Struct) {
	t.Helper()
	test.That(t, res, test.ShouldNotBeNil)

	test.That(t, res.Type, test.ShouldEqual, CaptureTypeTabular)
	test.That(t, res.TimeRequested, test.ShouldEqual, now)
	test.That(t, res.TimeReceived, test.ShouldHappenAfter, now)
	test.That(t, res.TimeReceived, test.ShouldHappenBefore, time.Now())
	test.That(t, res.Binaries, test.ShouldBeNil)
	test.That(t, res.TabularData.Payload, test.ShouldNotBeNil)
	test.That(t, res.TabularData.Payload, test.ShouldResemble, output)

	test.That(t, res.Validate(), test.ShouldBeNil)

	// confirm input conforms to ToProto expectations
	for _, proto := range res.ToProto() {
		test.That(t, proto.Metadata, test.ShouldResemble, &datasyncPB.SensorMetadata{
			TimeRequested: timestamppb.New(res.TimeRequested.UTC()),
			TimeReceived:  timestamppb.New(res.TimeReceived.UTC()),
		})

		test.That(t, proto.Data, test.ShouldResemble, &datasyncPB.SensorData_Struct{
			Struct: output,
		})
	}
}

func TestCaptureTypeToProto(t *testing.T) {
	test.That(t, CaptureTypeBinary.ToProto(), test.ShouldEqual, datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR)
	test.That(t, CaptureTypeTabular.ToProto(), test.ShouldEqual, datasyncPB.DataType_DATA_TYPE_TABULAR_SENSOR)
	test.That(t, CaptureTypeUnspecified.ToProto(), test.ShouldEqual, datasyncPB.DataType_DATA_TYPE_UNSPECIFIED)
	invalidCaptureType := CaptureType(20)
	test.That(t, invalidCaptureType.ToProto(), test.ShouldEqual, datasyncPB.DataType_DATA_TYPE_UNSPECIFIED)
}

func TestMimeTypeToProto(t *testing.T) {
	test.That(t, MimeTypeImageJpeg.ToProto(), test.ShouldEqual, datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG)
	test.That(t, MimeTypeImagePng.ToProto(), test.ShouldEqual, datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG)
	test.That(t, MimeTypeApplicationPcd.ToProto(), test.ShouldEqual, datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD)
	test.That(t, MimeTypeUnspecified.ToProto(), test.ShouldEqual, datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED)
}

func TestGetDataType(t *testing.T) {
	test.That(t, GetDataType(nextPointCloud), test.ShouldEqual, CaptureTypeBinary)
	test.That(t, GetDataType(readImage), test.ShouldEqual, CaptureTypeBinary)
	test.That(t, GetDataType(pointCloudMap), test.ShouldEqual, CaptureTypeBinary)
	test.That(t, GetDataType(GetImages), test.ShouldEqual, CaptureTypeBinary)
	test.That(t, GetDataType("anything else"), test.ShouldEqual, CaptureTypeTabular)
}

func TestMimeTypeFromProto(t *testing.T) {
	test.That(t, MimeTypeFromProto(datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG), test.ShouldEqual, MimeTypeImageJpeg)
	test.That(t, MimeTypeFromProto(datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG), test.ShouldEqual, MimeTypeImagePng)
	test.That(t, MimeTypeFromProto(datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD), test.ShouldEqual, MimeTypeApplicationPcd)
	test.That(t, MimeTypeFromProto(datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED), test.ShouldEqual, MimeTypeUnspecified)
	test.That(t, MimeTypeFromProto(datasyncPB.MimeType(20)), test.ShouldEqual, MimeTypeUnspecified)
}

func TestCameraFormatToMimeType(t *testing.T) {
	test.That(t, CameraFormatToMimeType(cameraPB.Format_FORMAT_JPEG), test.ShouldEqual, MimeTypeImageJpeg)
	test.That(t, CameraFormatToMimeType(cameraPB.Format_FORMAT_PNG), test.ShouldEqual, MimeTypeImagePng)
	test.That(t, CameraFormatToMimeType(cameraPB.Format_FORMAT_RAW_RGBA), test.ShouldEqual, MimeTypeUnspecified)
	test.That(t, CameraFormatToMimeType(cameraPB.Format_FORMAT_RAW_DEPTH), test.ShouldEqual, MimeTypeUnspecified)
	test.That(t, CameraFormatToMimeType(cameraPB.Format_FORMAT_UNSPECIFIED), test.ShouldEqual, MimeTypeUnspecified)
}

func TestAnnotationsToProto(t *testing.T) {
	conf := 0.2

	empty := Annotations{}
	test.That(t, empty.ToProto() == nil, test.ShouldBeTrue)

	onlyBBoxes := Annotations{
		BoundingBoxes: []BoundingBox{
			{Label: "a", Confidence: &conf, XMinNormalized: 1, XMaxNormalized: 2, YMinNormalized: 3, YMaxNormalized: 4},
			{Label: "b", XMinNormalized: 5, XMaxNormalized: 6, YMinNormalized: 7, YMaxNormalized: 8},
		},
	}
	test.That(t, onlyBBoxes.ToProto(), test.ShouldResemble, &v1.Annotations{
		Bboxes: []*v1.BoundingBox{
			{Label: "a", Confidence: &conf, XMinNormalized: 1, XMaxNormalized: 2, YMinNormalized: 3, YMaxNormalized: 4},
			{Label: "b", XMinNormalized: 5, XMaxNormalized: 6, YMinNormalized: 7, YMaxNormalized: 8},
		},
	})

	onlyClassifications := Annotations{
		Classifications: []Classification{
			{Label: "c"},
			{Label: "d", Confidence: &conf},
		},
	}
	test.That(t, onlyClassifications.ToProto(), test.ShouldResemble, &v1.Annotations{
		Classifications: []*v1.Classification{
			{Label: "c"},
			{Label: "d", Confidence: &conf},
		},
	})

	both := Annotations{
		BoundingBoxes: []BoundingBox{
			{Label: "a", Confidence: &conf, XMinNormalized: 1, XMaxNormalized: 2, YMinNormalized: 3, YMaxNormalized: 4},
			{Label: "b", XMinNormalized: 5, XMaxNormalized: 6, YMinNormalized: 7, YMaxNormalized: 8},
		},
		Classifications: []Classification{
			{Label: "c"},
			{Label: "d", Confidence: &conf},
		},
	}
	test.That(t, both.ToProto(), test.ShouldResemble, &v1.Annotations{
		Bboxes: []*v1.BoundingBox{
			{Label: "a", Confidence: &conf, XMinNormalized: 1, XMaxNormalized: 2, YMinNormalized: 3, YMaxNormalized: 4},
			{Label: "b", XMinNormalized: 5, XMaxNormalized: 6, YMinNormalized: 7, YMaxNormalized: 8},
		},
		Classifications: []*v1.Classification{
			{Label: "c"},
			{Label: "d", Confidence: &conf},
		},
	})
}

func TestGetFileExt(t *testing.T) {
	test.That(t, getFileExt(CaptureTypeTabular, "anything", nil), test.ShouldResemble, ".dat")
	test.That(t, getFileExt(CaptureTypeUnspecified, "anything", nil), test.ShouldResemble, "")
	test.That(t, getFileExt(CaptureType(20), "anything", nil), test.ShouldResemble, "")
	test.That(t, getFileExt(CaptureTypeBinary, "anything", nil), test.ShouldResemble, "")
	test.That(t, getFileExt(CaptureTypeBinary, "NextPointCloud", nil), test.ShouldResemble, ".pcd")
	test.That(t, getFileExt(CaptureTypeBinary, "ReadImage", nil), test.ShouldResemble, "")
	test.That(t, getFileExt(CaptureTypeBinary, "ReadImage", map[string]string{"mime_type": rutils.MimeTypeJPEG}), test.ShouldResemble, ".jpeg")
	test.That(t, getFileExt(CaptureTypeBinary, "ReadImage", map[string]string{"mime_type": rutils.MimeTypePNG}), test.ShouldResemble, ".png")
	test.That(t, getFileExt(CaptureTypeBinary, "ReadImage", map[string]string{"mime_type": rutils.MimeTypePCD}), test.ShouldResemble, ".pcd")
}
