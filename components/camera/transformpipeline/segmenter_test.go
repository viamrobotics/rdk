//go:build !no_media

package transformpipeline

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/edaniels/golog"
	"github.com/viamrobotics/gostream"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	vizservices "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	vision "go.viam.com/rdk/vision"
	segment "go.viam.com/rdk/vision/segmentation"
)

func TestTransformSegmenterProps(t *testing.T) {
	r := &inject.Robot{}
	cam := &inject.Camera{}
	vizServ := &inject.VisionService{}
	logger := golog.NewTestLogger(t)

	cam.StreamFunc = func(ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.MediaStream[image.Image], error) {
		return &streamTest{}, nil
	}
	cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		case "fakeVizService":
			return vizServ, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}

	transformConf := &transformConfig{
		Source: "fakeCamera",
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{
					"segmenter_name": "fakeVizService",
				},
			},
		},
	}

	am := transformConf.Pipeline[0].Attributes
	conf, err := resource.TransformAttributeMap[*segmenterConfig](am)
	test.That(t, err, test.ShouldBeNil)
	_, err = conf.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	_, err = newTransformPipeline(context.Background(), cam, transformConf, r, logger)
	test.That(t, err, test.ShouldBeNil)

	transformConf = &transformConfig{
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{},
			},
		},
	}

	am = transformConf.Pipeline[0].Attributes
	conf, err = resource.TransformAttributeMap[*segmenterConfig](am)
	test.That(t, err, test.ShouldBeNil)
	_, err = conf.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestTransformSegmenterFunctionality(t *testing.T) {
	// TODO(RSDK-1200): remove skip when complete
	t.Skip("remove skip once RSDK-1200 improvement is complete")

	r := &inject.Robot{}
	cam := &inject.Camera{}
	vizServ := &inject.VisionService{}
	logger := golog.NewTestLogger(t)

	cam.StreamFunc = func(ctx context.Context,
		errHandlers ...gostream.ErrorHandler,
	) (gostream.MediaStream[image.Image], error) {
		return &streamTest{}, nil
	}
	cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	vizServ.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string,
		extra map[string]interface{},
	) ([]*vision.Object, error) {
		segments := make([]pc.PointCloud, 3)
		segments[0] = pc.New()
		err := segments[0].Set(pc.NewVector(0, 0, 1), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}
		segments[1] = pc.New()
		err = segments[1].Set(pc.NewVector(0, 1, 0), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}
		segments[2] = pc.New()
		err = segments[2].Set(pc.NewVector(1, 0, 0), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		if err != nil {
			return nil, err
		}

		objects, err := segment.NewSegmentsFromSlice(segments, "fake")
		if err != nil {
			return nil, err
		}
		return objects.Objects, nil
	}

	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera"), vizservices.Named("fakeVizService")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		case "fakeVizService":
			return vizServ, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}

	transformConf := &transformConfig{
		Source: "fakeCamera",
		Pipeline: []Transformation{
			{
				Type: "segmentations", Attributes: utils.AttributeMap{
					"segmenter_name": "fakeVizService",
				},
			},
		},
	}

	pipeline, err := newTransformPipeline(context.Background(), cam, transformConf, r, logger)
	test.That(t, err, test.ShouldBeNil)

	pc, err := pipeline.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	_, isValid := pc.At(0, 0, 1)
	test.That(t, isValid, test.ShouldBeTrue)
	_, isValid = pc.At(1, 0, 0)
	test.That(t, isValid, test.ShouldBeTrue)
	_, isValid = pc.At(0, 1, 0)
	test.That(t, isValid, test.ShouldBeTrue)
}
