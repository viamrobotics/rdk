package transformpipeline

import (
	"context"
	"testing"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
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
	r := &inject.Robot{}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	inImg, _, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_color_original.png", inImg)

	color, err := newTransformPipeline(context.Background(), cam, transformConf, r)
	test.That(t, err, test.ShouldBeNil)

	outImg, _, err := color.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_color_transform_pipeline.png", outImg)
	test.That(t, err, test.ShouldBeNil)
	_, err = color.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	_, err = color.GetProperties(context.Background())
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)

}

func TestTransformPipelineDepth(t *testing.T) {
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     900.538000,
		Fy:     900.818000,
		Ppx:    648.934000,
		Ppy:    367.736000,
	}

	transformConf := &transformConfig{
		AttrConfig: &camera.AttrConfig{
			Stream:           "depth",
			CameraParameters: intrinsics,
		},
		Source: "source",
		Pipeline: []Transformation{
			{Type: "rotate", Attributes: config.AttributeMap{}},
			{Type: "resize", Attributes: config.AttributeMap{"height": 200, "width": 100}},
		},
	}
	r := &inject.Robot{}

	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: dm}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	inImg, _, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_depth_original.png", inImg)

	depth, err := newTransformPipeline(context.Background(), cam, transformConf, r)
	test.That(t, err, test.ShouldBeNil)

	outImg, _, err := depth.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/test_depth_transform_pipeline.png", outImg)
	test.That(t, err, test.ShouldBeNil)
	prop, err := depth.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, prop, test.ShouldResemble, intrinsics)
	outPc, err := depth.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outPc, test.ShouldNotBeNil)
}
