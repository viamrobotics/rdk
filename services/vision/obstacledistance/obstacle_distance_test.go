package obstacledistance

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
)

func TestObstacleDistDetector(t *testing.T) {
	inp := ObstacleDistanceDetectorConfig{
		NumQueries: 10,
	}
	ctx := context.Background()
	r := &inject.Robot{}
	name := vision.Named("test_odd") // what should this line be
	srv, err := registerObstacleDistanceDetector(ctx, name, &inp, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv.Name(), test.ShouldResemble, name)
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// Does not implement Detections
	_, err = srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// Does not implement Classifications
	_, err = srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// fakeCam := inject.NewCamera("myCam") // needs some work

	// visObj, err := srv.GetObjectPointClouds(ctx, "usSensor", nil)

	// with error - bad parameters
	inp.NumQueries = 0 // value out of range
	_, err = registerObstacleDistanceDetector(ctx, name, &inp, r)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid number of queries")

	// with error - nil parameters
	_, err = registerObstacleDistanceDetector(ctx, name, nil, r)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
}
