package vision_test

import (
	"context"
	"image"
	"strconv"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	v1 "go.viam.com/api/common/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	visionservice "go.viam.com/rdk/services/vision"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	vision "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/viscapture"
)

type fakeDetection struct {
	boundingBox *image.Rectangle
	score       float64
	label       string
}

type fakeClassification struct {
	score float64
	label string
}

type extraFields struct {
	Height   int
	Width    int
	MimeType string
}

const (
	serviceName     = "vision"
	captureInterval = time.Second
	numRetries      = 5
	testName1       = "CaptureAllFromCameraCollector returns non-empty CaptureAllFromCameraResp"
	testName2       = "CaptureAllFromCameraCollector w/ Classifications & Detections < 0.5 returns empty CaptureAllFromCameraResp"
)

var fakeDetections = []objectdetection.Detection{
	&fakeDetection{
		boundingBox: &image.Rectangle{
			Min: image.Point{X: 10, Y: 20},
			Max: image.Point{X: 110, Y: 120},
		},
		score: 0.95,
		label: "cat",
	},
}

var fakeDetections2 = []objectdetection.Detection{
	&fakeDetection{
		boundingBox: &image.Rectangle{
			Min: image.Point{X: 10, Y: 20},
			Max: image.Point{X: 110, Y: 120},
		},
		score: 0.3,
		label: "cat",
	},
}

var fakeClassifications = []classification.Classification{
	&fakeClassification{
		score: 0.95,
		label: "cat",
	},
}

var fakeClassifications2 = []classification.Classification{
	&fakeClassification{
		score: 0.49,
		label: "cat",
	},
}

var fakeObjects = []*vision.Object{}

var extra = extraFields{}

var fakeExtraFields, _ = protoutils.StructToStructPb(extra)

func (fc *fakeClassification) Score() float64 {
	return fc.score
}

func (fc *fakeClassification) Label() string {
	return fc.label
}

func (fd *fakeDetection) BoundingBox() *image.Rectangle {
	return fd.boundingBox
}

func (fd *fakeDetection) Score() float64 {
	return fd.score
}

func (fd *fakeDetection) Label() string {
	return fd.label
}

func clasToProto(classifications classification.Classifications) []*pb.Classification {
	protoCs := make([]*pb.Classification, 0, len(classifications))
	for _, c := range classifications {
		cc := &pb.Classification{
			ClassName:  c.Label(),
			Confidence: c.Score(),
		}
		protoCs = append(protoCs, cc)
	}
	return protoCs
}

func detsToProto(detections []objectdetection.Detection) []*pb.Detection {
	protoDets := make([]*pb.Detection, 0, len(detections))
	for _, det := range detections {
		box := det.BoundingBox()
		if box == nil {
			return nil
		}
		xMin := int64(box.Min.X)
		yMin := int64(box.Min.Y)
		xMax := int64(box.Max.X)
		yMax := int64(box.Max.Y)
		d := &pb.Detection{
			XMin:       &xMin,
			YMin:       &yMin,
			XMax:       &xMax,
			YMax:       &yMax,
			Confidence: det.Score(),
			ClassName:  det.Label(),
		}
		protoDets = append(protoDets, d)
	}
	return protoDets
}

func convertStringMapToAnyPBMap(params map[string]string) (map[string]*anypb.Any, error) {
	methodParams := map[string]*anypb.Any{}
	for key, paramVal := range params {
		anyVal, err := convertStringToAnyPB(paramVal)
		if err != nil {
			return nil, err
		}
		methodParams[key] = anyVal
	}
	return methodParams, nil
}

func convertStringToAnyPB(str string) (*anypb.Any, error) {
	var wrappedVal protoreflect.ProtoMessage
	if boolVal, err := strconv.ParseBool(str); err == nil {
		wrappedVal = wrapperspb.Bool(boolVal)
	} else if int64Val, err := strconv.ParseInt(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.Int64(int64Val)
	} else if uint64Val, err := strconv.ParseUint(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.UInt64(uint64Val)
	} else if float64Val, err := strconv.ParseFloat(str, 64); err == nil {
		wrappedVal = wrapperspb.Double(float64Val)
	} else {
		wrappedVal = wrapperspb.String(str)
	}
	anyVal, err := anypb.New(wrappedVal)
	if err != nil {
		return nil, err
	}
	return anyVal, nil
}

var methodParams, _ = convertStringMapToAnyPBMap(map[string]string{"camera_name": "camera-1", "mime_type": "image/jpeg"})

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      testName1,
			collector: visionservice.NewCaptureAllFromCameraCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.CaptureAllFromCameraResponse{
				Image:           &camerapb.Image{},
				Classifications: clasToProto(fakeClassifications),
				Detections:      detsToProto(fakeDetections),
				Objects:         []*v1.PointCloudObject{},
				Extra:           fakeExtraFields,
			}),
		},
		{
			name:      testName2,
			collector: visionservice.NewCaptureAllFromCameraCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.CaptureAllFromCameraResponse{
				Image:           &camerapb.Image{},
				Classifications: clasToProto([]classification.Classification{}),
				Detections:      detsToProto([]objectdetection.Detection{}),
				Objects:         []*v1.PointCloudObject{},
				Extra:           fakeExtraFields,
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			params := data.CollectorParams{
				ComponentName: serviceName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         mockClock,
				Target:        &buf,
				MethodParams:  methodParams,
			}

			var vision visionservice.Service
			if tc.name == testName1 {
				vision = newVisionService()
			} else if tc.name == testName2 {
				vision = newVisionService2()
			}

			col, err := tc.collector(vision, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			tu.Retry(func() bool {
				return buf.Length() != 0
			}, numRetries)
			test.That(t, buf.Length(), test.ShouldBeGreaterThan, 0)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

func newVisionService() visionservice.Service {
	v := &inject.VisionService{}
	v.CaptureAllFromCameraFunc = func(ctx context.Context, cameraName string, opts viscapture.CaptureOptions,
		extra map[string]interface{},
	) (viscapture.VisCapture, error) {
		return viscapture.VisCapture{
			Image:           nil,
			Detections:      fakeDetections,
			Classifications: fakeClassifications,
			Objects:         fakeObjects,
		}, nil
	}

	return v
}

func newVisionService2() visionservice.Service {
	v := &inject.VisionService{}
	v.CaptureAllFromCameraFunc = func(ctx context.Context, cameraName string, opts viscapture.CaptureOptions,
		extra map[string]interface{},
	) (viscapture.VisCapture, error) {
		return viscapture.VisCapture{
			Image:           nil,
			Detections:      fakeDetections2,
			Classifications: fakeClassifications2,
			Objects:         fakeObjects,
		}, nil
	}

	return v
}
