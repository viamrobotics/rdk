package camera_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"io"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	v1 "go.viam.com/api/app/data/v1"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

//nolint:lll
var viamLogoJpegB64 = []byte("/9j/4QD4RXhpZgAATU0AKgAAAAgABwESAAMAAAABAAEAAAEaAAUAAAABAAAAYgEbAAUAAAABAAAAagEoAAMAAAABAAIAAAExAAIAAAAhAAAAcgITAAMAAAABAAEAAIdpAAQAAAABAAAAlAAAAAAAAABIAAAAAQAAAEgAAAABQWRvYmUgUGhvdG9zaG9wIDIzLjQgKE1hY2ludG9zaCkAAAAHkAAABwAAAAQwMjIxkQEABwAAAAQBAgMAoAAABwAAAAQwMTAwoAEAAwAAAAEAAQAAoAIABAAAAAEAAAAgoAMABAAAAAEAAAAgpAYAAwAAAAEAAAAAAAAAAAAA/9sAhAAcHBwcHBwwHBwwRDAwMERcRERERFx0XFxcXFx0jHR0dHR0dIyMjIyMjIyMqKioqKioxMTExMTc3Nzc3Nzc3NzcASIkJDg0OGA0NGDmnICc5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ubm5ub/3QAEAAL/wAARCAAgACADASIAAhEBAxEB/8QBogAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoLEAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+foBAAMBAQEBAQEBAQEAAAAAAAABAgMEBQYHCAkKCxEAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDm6K0dNu1tZsSgGNuDx0961NX09WT7ZbgcD5gPT1oA5qiul0fT1VPtlwByPlB7D1rL1K7W5mxEAI04GBjPvQB//9Dm66TRr/I+xTf8A/wrm6ASpBXgjpQB0ms34UfYof8AgWP5VzdBJY5PJNFAH//Z")

var (
	doCommandMap = map[string]any{"readings": "random-test"}
	annotations1 = data.Annotations{Classifications: []data.Classification{{Label: "add_annotations"}}}
	annotations2 = data.Annotations{Classifications: []data.Classification{{Label: "add_more_annotations"}}}
)

const (
	serviceName     = "camera"
	captureInterval = time.Millisecond
)

func TestCollectors(t *testing.T) {
	viamLogoJpeg, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(viamLogoJpegB64)))
	test.That(t, err, test.ShouldBeNil)

	img := rimage.NewLazyEncodedImage(viamLogoJpeg, utils.MimeTypeJPEG)
	// 32 x 32 image
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 32)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 32)

	pcd, err := pointcloud.NewFromFile(artifact.MustPath("pointcloud/test.las"), "")
	test.That(t, err, test.ShouldBeNil)

	var pcdBuf bytes.Buffer
	test.That(t, pointcloud.ToPCD(pcd, &pcdBuf, pointcloud.PCDBinary), test.ShouldBeNil)

	cam := newCamera(img, img, pcd)

	tests := []struct {
		name         string
		collector    data.CollectorConstructor
		expected     []*datasyncpb.SensorData
		camera       camera.Camera
		methodParams map[string]interface{}
	}{
		{
			name:      "ReadImage returns a non nil binary response",
			collector: camera.NewReadImageCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
					Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "add_annotations"}}},
				},
				Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
			}},
			camera:       cam,
			methodParams: map[string]interface{}{"camera_name": "camera-1", "mime_type": "image/jpeg"},
		},
		{
			name:      "NextPointCloud returns a non nil binary response",
			collector: camera.NewNextPointCloudCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{
					MimeType: datasyncpb.MimeType_MIME_TYPE_APPLICATION_PCD,
				},
				Data: &datasyncpb.SensorData_Binary{Binary: pcdBuf.Bytes()},
			}},
			camera: cam,
		},
		{
			name:      "GetImages returns a non nil binary response",
			collector: camera.NewGetImagesCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{
						MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
						Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "add_annotations"}, {Label: "left"}}},
					},
					Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
				},
				{
					Metadata: &datasyncpb.SensorMetadata{
						MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
						Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "add_more_annotations"}, {Label: "right"}}},
					},
					Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
				},
			},
			camera:       cam,
			methodParams: map[string]interface{}{"camera_name": "camera-1"},
		},
		{
			name:      "GetImages with filterSourceNames returns only filtered images",
			collector: camera.NewGetImagesCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{
						MimeType:    datasyncpb.MimeType_MIME_TYPE_IMAGE_JPEG,
						Annotations: &v1.Annotations{Classifications: []*v1.Classification{{Label: "add_annotations"}, {Label: "left"}}},
					},
					Data: &datasyncpb.SensorData_Binary{Binary: viamLogoJpeg},
				},
			},
			camera:       cam,
			methodParams: map[string]interface{}{"camera_name": "camera-1", "filter_source_names": []interface{}{"left"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			mParams, err := protoutils.ConvertMapToProtoAny(tc.methodParams)
			test.That(t, err, test.ShouldBeNil)
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeBinary,
				ComponentName: serviceName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
				MethodParams:  mParams,
			}

			col, err := tc.collector(tc.camera, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
			buf.Close()
		})
	}
}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   serviceName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       camera.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newCamera(nil, nil, nil) },
	})
}

func newCamera(
	left, right image.Image,
	pcd pointcloud.PointCloud,
) camera.Camera {
	v := &inject.Camera{}

	v.NextPointCloudFunc = func(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
		return pcd, nil
	}

	v.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		leftImg, err := camera.NamedImageFromImage(left, "left", utils.MimeTypeJPEG, annotations1)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		rightImg, err := camera.NamedImageFromImage(right, "right", utils.MimeTypeJPEG, annotations2)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}

		allImgs := []camera.NamedImage{leftImg, rightImg}
		if len(filterSourceNames) == 0 {
			return allImgs, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}

		var filteredImgs []camera.NamedImage
		for _, img := range allImgs {
			for _, filter := range filterSourceNames {
				if img.SourceName == filter {
					filteredImgs = append(filteredImgs, img)
				}
			}
		}

		return filteredImgs,
			resource.ResponseMetadata{CapturedAt: time.Now()},
			nil
	}

	v.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}

	return v
}
