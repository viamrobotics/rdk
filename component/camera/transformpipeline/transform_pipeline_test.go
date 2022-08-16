package transformpipeline

import (
	"context"
	"testing"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestTransformPipelineColor(t *testing.T) {
	transformConf := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "color",
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	inImg, _, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_color_original.png", inImg)

	color, err := newTransformPipeline(context.Background(), cam, transformConf)
	test.That(t, err, test.ShouldBeNil)

	outImg, _, err := color.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_color_transform_pipeline.png", outImg)
	test.That(t, err, test.ShouldBeNil)
}

func TestTransformPipelineDepth(t *testing.T) {
	transformConf := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}
}
