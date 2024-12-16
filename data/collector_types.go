package data

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	dataPB "go.viam.com/api/app/data/v1"
	datasyncPB "go.viam.com/api/app/datasync/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	rprotoutils "go.viam.com/rdk/protoutils"
	rutils "go.viam.com/rdk/utils"
)

// CaptureFunc allows the creation of simple Capturers with anonymous functions.
type CaptureFunc func(ctx context.Context, params map[string]*anypb.Any) (CaptureResult, error)

// CaptureResult is the result of a capture function.
type CaptureResult struct {
	// Type represents the type of result (binary or tabular)
	Type CaptureType
	// Timestamps contain the time the data was requested and received
	Timestamps
	// TabularData contains the tabular data payload when Type == CaptureResultTypeTabular
	TabularData TabularData
	// Binaries contains binary data responses when Type == CaptureResultTypeBinary
	Binaries []Binary
}

// BEGIN CONSTRUCTORS

// NewBinaryCaptureResult returns a binary capture result.
func NewBinaryCaptureResult(ts Timestamps, binaries []Binary) CaptureResult {
	return CaptureResult{
		Timestamps: ts,
		Type:       CaptureTypeBinary,
		Binaries:   binaries,
	}
}

// NewTabularCaptureResultReadings returns a tabular readings result.
func NewTabularCaptureResultReadings(ts Timestamps, readings map[string]interface{}) (CaptureResult, error) {
	var res CaptureResult
	values, err := rprotoutils.ReadingGoToProto(readings)
	if err != nil {
		return res, err
	}

	return CaptureResult{
		Timestamps: ts,
		Type:       CaptureTypeTabular,
		TabularData: TabularData{
			Payload: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					// Top level key necessary for backwards compatibility with GetReadingsResponse.
					"readings": structpb.NewStructValue(&structpb.Struct{Fields: values}),
				},
			},
		},
	}, nil
}

// NewTabularCaptureResult returns a tabular result.
func NewTabularCaptureResult(ts Timestamps, i interface{}) (CaptureResult, error) {
	var res CaptureResult
	readings, err := protoutils.StructToStructPbIgnoreOmitEmpty(i)
	if err != nil {
		return res, err
	}

	return CaptureResult{
		Timestamps: ts,
		Type:       CaptureTypeTabular,
		TabularData: TabularData{
			Payload: readings,
		},
	}, nil
}

// END CONSTRUCTORS

// ToProto converts a CaptureResult into a []*datasyncPB.SensorData{}.
func (cr *CaptureResult) ToProto() []*datasyncPB.SensorData {
	ts := cr.Timestamps
	if cr.Type == CaptureTypeTabular {
		return []*datasyncPB.SensorData{{
			Metadata: &datasyncPB.SensorMetadata{
				TimeRequested: timestamppb.New(ts.TimeRequested.UTC()),
				TimeReceived:  timestamppb.New(ts.TimeReceived.UTC()),
			},
			Data: &datasyncPB.SensorData_Struct{
				Struct: cr.TabularData.Payload,
			},
		}}
	}

	if cr.Type == CaptureTypeBinary {
		var sd []*datasyncPB.SensorData
		for _, b := range cr.Binaries {
			sd = append(sd, &datasyncPB.SensorData{
				Metadata: &datasyncPB.SensorMetadata{
					TimeRequested: timestamppb.New(ts.TimeRequested.UTC()),
					TimeReceived:  timestamppb.New(ts.TimeReceived.UTC()),
					MimeType:      b.MimeType.ToProto(),
					Annotations:   b.Annotations.ToProto(),
				},
				Data: &datasyncPB.SensorData_Binary{
					Binary: b.Payload,
				},
			})
		}
		return sd
	}

	// This should never happen
	return nil
}

// Validate returns an error if the *CaptureResult is invalid.
func (cr *CaptureResult) Validate() error {
	if cr.Timestamps.TimeRequested.IsZero() {
		return errors.New("Timestamps.TimeRequested must be set")
	}

	if cr.Timestamps.TimeReceived.IsZero() {
		return errors.New("Timestamps.TimeRequested must be set")
	}

	switch cr.Type {
	case CaptureTypeTabular:
		if len(cr.Binaries) > 0 {
			return errors.New("tabular result can't contain binary data")
		}
		if cr.TabularData.Payload == nil {
			return errors.New("tabular result must have non empty tabular data")
		}
		return nil
	case CaptureTypeBinary:
		if cr.TabularData.Payload != nil {
			return errors.New("binary result can't contain tabular data")
		}
		if len(cr.Binaries) == 0 {
			return errors.New("binary result must have non empty binary data")
		}

		for _, b := range cr.Binaries {
			if len(b.Payload) == 0 {
				return errors.New("binary results can't have empty binary payload")
			}
		}
		return nil
	case CaptureTypeUnspecified:
		return fmt.Errorf("unknown CaptureResultType: %d", cr.Type)
	default:
		return fmt.Errorf("unknown CaptureResultType: %d", cr.Type)
	}
}

// CaptureType represents captured tabular or binary data.
type CaptureType int

const (
	// CaptureTypeUnspecified represents that the data type of the captured data was not specified.
	CaptureTypeUnspecified CaptureType = iota
	// CaptureTypeTabular represents that the data type of the captured data is tabular.
	CaptureTypeTabular
	// CaptureTypeBinary represents that the data type of the captured data is binary.
	CaptureTypeBinary
)

// ToProto converts a CaptureType into a v1.DataType.
func (dt CaptureType) ToProto() datasyncPB.DataType {
	switch dt {
	case CaptureTypeTabular:
		return datasyncPB.DataType_DATA_TYPE_TABULAR_SENSOR
	case CaptureTypeBinary:
		return datasyncPB.DataType_DATA_TYPE_BINARY_SENSOR
	case CaptureTypeUnspecified:
		return datasyncPB.DataType_DATA_TYPE_UNSPECIFIED
	default:
		return datasyncPB.DataType_DATA_TYPE_UNSPECIFIED
	}
}

// MethodToCaptureType returns the DataType of the method.
func MethodToCaptureType(methodName string) CaptureType {
	switch methodName {
	case nextPointCloud, readImage, pointCloudMap, GetImages, captureAllFromCamera:
		return CaptureTypeBinary
	default:
		return CaptureTypeTabular
	}
}

// Timestamps are the timestamps the data was captured.
type Timestamps struct {
	// TimeRequested represents the time the request for the data was started
	TimeRequested time.Time
	// TimeReceived represents the time the response for the request for the data
	// was received
	TimeReceived time.Time
}

// MimeType represents the mime type of the sensor data.
type MimeType int

// This follows the mime types supported in
// https://github.com/viamrobotics/api/blob/d7436a969cbc03bf7e84bb456b6dbfeb51f664d7/proto/viam/app/datasync/v1/data_sync.proto#L69
const (
	// MimeTypeUnspecified means that the mime type was not specified.
	MimeTypeUnspecified MimeType = iota
	// MimeTypeImageJpeg means that the mime type is jpeg.
	MimeTypeImageJpeg
	// MimeTypeImagePng means that the mime type is png.
	MimeTypeImagePng
	// MimeTypeApplicationPcd means that the mime type is pcd.
	MimeTypeApplicationPcd
)

// ToProto converts MimeType to datasyncPB.
func (mt MimeType) ToProto() datasyncPB.MimeType {
	switch mt {
	case MimeTypeUnspecified:
		return datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED
	case MimeTypeImageJpeg:
		return datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG
	case MimeTypeImagePng:
		return datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG
	case MimeTypeApplicationPcd:
		return datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD
	default:
		return datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED
	}
}

// MimeTypeFromProto converts a datasyncPB.MimeType to a data.MimeType.
func MimeTypeFromProto(mt datasyncPB.MimeType) MimeType {
	switch mt {
	case datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED:
		return MimeTypeUnspecified
	case datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG:
		return MimeTypeImageJpeg
	case datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG:
		return MimeTypeImagePng
	case datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD:
		return MimeTypeApplicationPcd
	default:
		return MimeTypeUnspecified
	}
}

// CameraFormatToMimeType converts a camera camerapb.Format into a MimeType.
func CameraFormatToMimeType(f camerapb.Format) MimeType {
	switch f {
	case camerapb.Format_FORMAT_UNSPECIFIED:
		return MimeTypeUnspecified
	case camerapb.Format_FORMAT_JPEG:
		return MimeTypeImageJpeg
	case camerapb.Format_FORMAT_PNG:
		return MimeTypeImagePng
	case camerapb.Format_FORMAT_RAW_RGBA:
		// TODO: https://viam.atlassian.net/browse/DATA-3497
		fallthrough
	case camerapb.Format_FORMAT_RAW_DEPTH:
		// TODO: https://viam.atlassian.net/browse/DATA-3497
		fallthrough
	default:
		return MimeTypeUnspecified
	}
}

// MimeTypeToCameraFormat converts a data.MimeType into a camerapb.Format.
func MimeTypeToCameraFormat(mt MimeType) camerapb.Format {
	if mt == MimeTypeImageJpeg {
		return camerapb.Format_FORMAT_JPEG
	}

	if mt == MimeTypeImagePng {
		return camerapb.Format_FORMAT_PNG
	}
	return camerapb.Format_FORMAT_UNSPECIFIED
}

// Binary represents an element of a binary capture result response.
type Binary struct {
	// Payload contains the binary payload
	Payload []byte
	// MimeType  descibes the payload's MimeType
	MimeType MimeType
	// Annotations provide metadata about the Payload
	Annotations Annotations
}

// TabularData contains a tabular data payload.
type TabularData struct {
	Payload *structpb.Struct
}

// BoundingBox represents a labeled bounding box
// with an optional confidence interval between 0 and 1.
type BoundingBox struct {
	Label          string
	Confidence     *float64
	XMinNormalized float64
	YMinNormalized float64
	XMaxNormalized float64
	YMaxNormalized float64
}

// Classification represents a labeled classification
// with an optional confidence interval between 0 and 1.
type Classification struct {
	Label      string
	Confidence *float64
}

// Annotations represents ML classifications.
type Annotations struct {
	BoundingBoxes   []BoundingBox
	Classifications []Classification
}

// Empty returns true when Annotations are empty.
func (mt Annotations) Empty() bool {
	return len(mt.BoundingBoxes) == 0 && len(mt.Classifications) == 0
}

// ToProto converts Annotations to *dataPB.Annotations.
func (mt Annotations) ToProto() *dataPB.Annotations {
	if mt.Empty() {
		return nil
	}

	var bboxes []*dataPB.BoundingBox
	for _, bb := range mt.BoundingBoxes {
		bboxes = append(bboxes, &dataPB.BoundingBox{
			Label:          bb.Label,
			Confidence:     bb.Confidence,
			XMinNormalized: bb.XMinNormalized,
			XMaxNormalized: bb.XMaxNormalized,
			YMinNormalized: bb.YMinNormalized,
			YMaxNormalized: bb.YMaxNormalized,
		})
	}

	var classifications []*dataPB.Classification
	for _, c := range mt.Classifications {
		classifications = append(classifications, &dataPB.Classification{
			Label:      c.Label,
			Confidence: c.Confidence,
		})
	}

	return &dataPB.Annotations{
		Bboxes:          bboxes,
		Classifications: classifications,
	}
}

const (
	// ExtDefault is the default file extension.
	ExtDefault = ""
	// ExtDat is the file extension for tabular data.
	ExtDat = ".dat"
	// ExtPcd is the file extension for pcd files.
	ExtPcd = ".pcd"
	// ExtJpeg is the file extension for jpeg files.
	ExtJpeg = ".jpeg"
	// ExtPng is the file extension for png files.
	ExtPng = ".png"
)

// getFileExt gets the file extension for a capture file.
func getFileExt(dataType CaptureType, methodName string, parameters map[string]string) string {
	switch dataType {
	case CaptureTypeTabular:
		return ExtDat
	case CaptureTypeBinary:
		if methodName == nextPointCloud {
			return ExtPcd
		}
		if methodName == readImage {
			// TODO: Add explicit file extensions for all mime types.
			switch parameters["mime_type"] {
			case rutils.MimeTypeJPEG:
				return ExtJpeg
			case rutils.MimeTypePNG:
				return ExtPng
			case rutils.MimeTypePCD:
				return ExtPcd
			default:
				return ExtDefault
			}
		}
	case CaptureTypeUnspecified:
		return ExtDefault
	default:
		return ExtDefault
	}
	return ExtDefault
}
