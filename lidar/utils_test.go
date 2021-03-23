package lidar_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/test"
)

func TestBestAngularResolution(t *testing.T) {
	lidar1 := &inject.LidarDevice{}
	lidar1.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}
	lidar2 := &inject.LidarDevice{}
	lidar2.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}

	_, _, _, err := lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	lidar1.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}
	_, _, _, err = lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	lidar2.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return .25, nil
	}
	best, bestDevice, bestDeviceNum, err := lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, best, test.ShouldEqual, .25)
	test.That(t, bestDevice, test.ShouldEqual, lidar2)
	test.That(t, bestDeviceNum, test.ShouldEqual, 1)
}

func TestDeviceDescriptionFlag(t *testing.T) {
	type MyStruct struct {
		Desc  lidar.DeviceDescription `flag:"desc"`
		Desc2 lidar.DeviceDescription `flag:"0"`
	}
	var myStruct MyStruct
	err := utils.ParseFlags([]string{"main", "--desc=foo"}, &myStruct)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	err = utils.ParseFlags([]string{"main", "--desc=foo,bar"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Desc.Type, test.ShouldEqual, lidar.DeviceType("foo"))
	test.That(t, myStruct.Desc.Path, test.ShouldEqual, "bar")

	err = utils.ParseFlags([]string{"main", "foo,bar"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Desc2.Type, test.ShouldEqual, lidar.DeviceType("foo"))
	test.That(t, myStruct.Desc2.Path, test.ShouldEqual, "bar")
}

func TestParseDeviceFlag(t *testing.T) {
	_, err := lidar.ParseDeviceFlag("woo", "foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "--woo")
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	desc, err := lidar.ParseDeviceFlag("woo", "foo,bar")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, desc, test.ShouldResemble, lidar.DeviceDescription{Type: lidar.DeviceType("foo"), Path: "bar"})
}

func TestParseDeviceFlags(t *testing.T) {
	_, err := lidar.ParseDeviceFlags("woo", []string{"foo", "foo,bar", "baz,baf"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "--woo")
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	descs, err := lidar.ParseDeviceFlags("woo", []string{"foo,bar", "baz,baf"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, descs, test.ShouldResemble, []lidar.DeviceDescription{
		{Type: lidar.DeviceType("foo"), Path: "bar"},
		{Type: lidar.DeviceType("baz"), Path: "baf"},
	})
}
