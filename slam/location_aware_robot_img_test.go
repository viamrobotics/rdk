package slam

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/artifact"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/slam/v1"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils"
	"go.viam.com/core/utils"

	"go.viam.com/test"
)

func TestRobotNext(t *testing.T) {
	// empty means no detected points
	t.Run("initially image should be empty", func(t *testing.T) {
		harness := newTestHarnessWithSize(t, 10, 10)
		larBot := harness.bot
		img, _, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		rimage.IterateImage(img, func(x, y int, c color.Color) bool {
			r, g, b, a := c.RGBA()
			cC := color.RGBA{uint8(r / 256), uint8(g / 256), uint8(b / 256), uint8(a / 256)}
			test.That(t, cC, test.ShouldNotResemble, areaPointColor)
			return true
		})
	})

	t.Run("with area set to a few points", func(t *testing.T) {
		harness := newTestHarnessWithSize(t, 10, 10)
		larBot := harness.bot
		harness.area.Mutate(func(area MutableArea) {
			test.That(t, area.Set(-10, 1, 1), test.ShouldBeNil)
			test.That(t, area.Set(5, -20, 1), test.ShouldBeNil)
			test.That(t, area.Set(40, 4, 1), test.ShouldBeNil)
		})

		img, _, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		points := utils.NewStringSet("40,49", "55,70", "90,46")
		rimage.IterateImage(img, func(x, y int, c color.Color) bool {
			cC := rimage.ConvertToNRGBA(c)
			if cC == areaPointColor {
				delete(points, fmt.Sprintf("%d,%d", x, y))
			}
			return true
		})
		test.That(t, points, test.ShouldBeEmpty)

		t.Run("live should be based on lidar, not area", func(t *testing.T) {
			larBot.clientLidarViewMode = pb.LidarViewMode_LIDAR_VIEW_MODE_LIVE

			err1 := errors.New("oof")
			harness.lidarDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
				return nil, err1
			}

			_, _, err := larBot.Next(context.Background())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, errors.Is(err, err1), test.ShouldBeTrue)

			harness.lidarDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
				return lidar.Measurements{
					lidar.NewMeasurement(0, 1),
					lidar.NewMeasurement(1, 2),
					lidar.NewMeasurement(3, 4),
					lidar.NewMeasurement(5, 3),
				}, nil
			}

			img, _, err := larBot.Next(context.Background())
			test.That(t, err, test.ShouldBeNil)
			count := 0
			rimage.IterateImage(img, func(x, y int, c color.Color) bool {
				cC := rimage.ConvertToNRGBA(c)
				if cC == areaPointColor {
					count++
				}
				return true
			})
			test.That(t, count, test.ShouldEqual, 4)
		})
	})

	t.Run("unknown view mode", func(t *testing.T) {
		harness := newTestHarnessWithSize(t, 10, 10)
		larBot := harness.bot
		larBot.clientLidarViewMode = pb.LidarViewMode_LIDAR_VIEW_MODE_UNSPECIFIED
		_, _, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown view mode")
	})

	t.Run("precomputed", func(t *testing.T) {
		getDataFileName := func(testName string) string {
			return artifact.MustPath(fmt.Sprintf("slam/%s.png", testName))
		}

		tempDir := testutils.TempDirT(t, "", "slam")
		getNewDataFileName := func(testName string) string {
			return fmt.Sprintf("%s/%s_new.png", tempDir, testName)
		}
		getDiffDataFileName := func(testName string) string {
			return fmt.Sprintf("%s/%s_diff.png", tempDir, testName)
		}

		for _, tc := range []struct {
			Seed        int64
			BasePosX    float64
			BasePosY    float64
			Zoom        int
			Orientation int
			Diff        int
		}{
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 0, Diff: 1452},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 15, Diff: 1419},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 30, Diff: 1388},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 45, Diff: 1387},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 60, Diff: 1336},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 75, Diff: 1342},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 90, Diff: 1381},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 120, Diff: 1345},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 135, Diff: 1350},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 150, Diff: 1346},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 165, Diff: 1325},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 180, Diff: 1396},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 195, Diff: 1347},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 210, Diff: 1270},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 225, Diff: 1392},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 240, Diff: 1374},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 255, Diff: 1419},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 270, Diff: 1441},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 285, Diff: 1474},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 315, Diff: 1442},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 330, Diff: 1424},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 1, Orientation: 345, Diff: 1441},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 2, Orientation: 0, Diff: 1442},
			{Seed: 0, BasePosX: 0, BasePosY: 0, Zoom: 2, Orientation: 90, Diff: 1370},
			{Seed: 5, BasePosX: 5, BasePosY: 0, Zoom: 2, Orientation: 90, Diff: 1369},
		} {
			testName := fmt.Sprintf("%d_%v_%v_%d_%d", tc.Seed, tc.BasePosX, tc.BasePosY, tc.Zoom, tc.Orientation)
			t.Run(testName, func(t *testing.T) {
				fakeLidar := fake.NewLidar("lidar1")
				fakeLidar.SetSeed(tc.Seed)
				test.That(t, fakeLidar.Start(context.Background()), test.ShouldBeNil)
				harness := newTestHarnessWithLidarAndSize(t, fakeLidar, 10, 10)
				larBot := harness.bot
				larBot.clientLidarViewMode = pb.LidarViewMode_LIDAR_VIEW_MODE_LIVE
				larBot.basePosX = tc.BasePosX
				larBot.basePosY = tc.BasePosY
				test.That(t, tc.Zoom, test.ShouldBeGreaterThanOrEqualTo, 1)
				larBot.clientZoom = float64(tc.Zoom)
				larBot.setOrientation(float64(tc.Orientation))

				img, _, err := larBot.Next(context.Background())
				test.That(t, err, test.ShouldBeNil)

				fn := getDataFileName(testName)
				expectedFile, err := os.Open(fn)
				if os.IsNotExist(err) {
					newFileName := getNewDataFileName(testName)
					t.Logf("no file for test %s, will output new image to %s", testName, newFileName)
					t.Log("if it looks good, remove _new")
					test.That(t, rimage.WriteImageToFile(newFileName, img), test.ShouldBeNil)
				}
				test.That(t, err, test.ShouldBeNil)

				expectedImg, _, err := image.Decode(expectedFile)
				test.That(t, err, test.ShouldBeNil)
				cmp, cmpImg, err := rimage.CompareImages(img, expectedImg)
				test.That(t, err, test.ShouldBeNil)
				if cmp > tc.Diff {
					newFileName := getNewDataFileName(testName)
					t.Logf("image for test %s does not match, will output new image to %s", testName, newFileName)
					t.Log("if it looks good, replace old file")
					test.That(t, rimage.WriteImageToFile(newFileName, img), test.ShouldBeNil)
					diffFileName := getDiffDataFileName(testName)
					test.That(t, rimage.WriteImageToFile(diffFileName, cmpImg), test.ShouldBeNil)
				}
				tcCopy := tc
				tcCopy.Diff = cmp
				t.Logf("possibly new case %#v\n", tcCopy)
				test.That(t, cmp, test.ShouldAlmostEqual, tc.Diff, 10)
			})
		}
	})
}
