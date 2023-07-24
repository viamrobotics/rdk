package videosource

import (
	"context"
	"path/filepath"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
)

func TestPCD(t *testing.T) {
	pcdPath := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
	cfg := &fileSourceConfig{PointCloud: pcdPath}
	ctx := context.Background()
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	pc, err := cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	pc, err = cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg.Color = colorImgPath
	cam, err = newCamera(ctx, resource.Name{API: camera.API}, cfg)
	test.That(t, err, test.ShouldBeNil)

	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	// inputFile, _ := os.Open(colorImgPath)
	// test.That(t, err, test.ShouldBeNil)
	// defer inputFile.Close()
	// readInImage, _, err := image.Decode(inputFile)
	// test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	// TODO: add to this test with an image comparison
	// test.That(t, strmImg, test.ShouldResemble, readInImage)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestColor(t *testing.T) {
	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg := &fileSourceConfig{Color: colorImgPath}
	ctx := context.Background()
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
