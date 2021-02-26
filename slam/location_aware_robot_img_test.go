package slam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/test"
)

func TestNext(t *testing.T) {
	// empty means no detected points
	t.Run("initially image should be empty", func(t *testing.T) {
		harness := newTestHarness(t)
		larBot := harness.bot
		img, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		utils.IterateImage(img, func(x, y int, c color.Color) bool {
			r, g, b, a := c.RGBA()
			cC := color.RGBA{uint8(r / 256), uint8(g / 256), uint8(b / 256), uint8(a / 256)}
			test.That(t, cC, test.ShouldNotResemble, areaPointColor)
			return true
		})
	})

	t.Run("with area set to a few points", func(t *testing.T) {
		harness := newTestHarness(t)
		larBot := harness.bot
		harness.area.Mutate(func(area MutableArea) {
			area.Set(1, 1, 1)
			area.Set(5, 20, 1)
			area.Set(80, 4, 1)
		})

		img, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		points := utils.NewStringSet("1,1", "5,20", "80,4")
		utils.IterateImage(img, func(x, y int, c color.Color) bool {
			cC := utils.ConvertToNRGBA(c)
			if cC == areaPointColor {
				delete(points, fmt.Sprintf("%d,%d", x, y))
			}
			return true
		})
		test.That(t, points, test.ShouldBeEmpty)

		t.Run("live should be based on lidar, not area", func(t *testing.T) {
			larBot.clientLidarViewMode = clientLidarViewModeLive

			err1 := errors.New("oof")
			harness.lidarDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
				return nil, err1
			}

			_, err := larBot.Next(context.Background())
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

			img, err := larBot.Next(context.Background())
			test.That(t, err, test.ShouldBeNil)
			count := 0
			utils.IterateImage(img, func(x, y int, c color.Color) bool {
				cC := utils.ConvertToNRGBA(c)
				if cC == areaPointColor {
					count++
				}
				return true
			})
			test.That(t, count, test.ShouldEqual, 4)
		})
	})

	t.Run("unknown view mode", func(t *testing.T) {
		harness := newTestHarness(t)
		larBot := harness.bot
		larBot.clientLidarViewMode = "idk"
		_, err := larBot.Next(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown view mode")
	})

	t.Run("precomputed", func(t *testing.T) {
		getDataFileName := func(num int) string {
			return testutils.ResolveFile(fmt.Sprintf("slam/data/%d.png", num))
		}
		getNewDataFileName := func(num int) string {
			return testutils.ResolveFile(fmt.Sprintf("slam/data/%d_new.png", num))
		}
		getDiffDataFileName := func(num int) string {
			return testutils.ResolveFile(fmt.Sprintf("slam/data/%d_diff.png", num))
		}

		for i, tc := range []struct {
			Seed        int64
			BasePosX    int
			BasePosY    int
			Zoom        float64
			Orientation float64
			Diff        int
		}{
			{0, 0, 0, 1, 0, 1924},
			{0, 0, 0, 2, 0, 1925},
			{0, 0, 0, 2, 90, 1928},
		} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				fakeLidar := fake.NewLidar()
				fakeLidar.SetSeed(tc.Seed)
				test.That(t, fakeLidar.Start(context.Background()), test.ShouldBeNil)
				harness := newTestHarnessWithLidar(t, fakeLidar)
				larBot := harness.bot
				larBot.clientLidarViewMode = clientLidarViewModeLive
				larBot.basePosX = tc.BasePosX
				larBot.basePosY = tc.BasePosY
				test.That(t, tc.Zoom, test.ShouldBeGreaterThanOrEqualTo, 1)
				larBot.clientZoom = tc.Zoom
				larBot.setOrientation(tc.Orientation)

				img, err := larBot.Next(context.Background())
				test.That(t, err, test.ShouldBeNil)

				fn := getDataFileName(i)
				expectedFile, err := os.Open(fn)
				if os.IsNotExist(err) {
					newFileName := getNewDataFileName(i)
					t.Logf("no file for case %d, will output new image to %s", i, newFileName)
					t.Log("if it looks good, remove _new")
					test.That(t, utils.WriteImageToFile(newFileName, img), test.ShouldBeNil)
				}
				test.That(t, err, test.ShouldBeNil)

				expectedImg, _, err := image.Decode(expectedFile)
				test.That(t, err, test.ShouldBeNil)
				cmp, cmpImg, err := utils.CompareImages(img, expectedImg)
				test.That(t, err, test.ShouldBeNil)
				if cmp > tc.Diff {
					newFileName := getNewDataFileName(i)
					t.Logf("image for case %d does not match, will output new image to %s", i, newFileName)
					t.Log("if it looks good, replace old file")
					test.That(t, utils.WriteImageToFile(newFileName, img), test.ShouldBeNil)
					diffFileName := getDiffDataFileName(i)
					thinkFileName := testutils.ResolveFile(fmt.Sprintf("slam/data/%d_think.png", i))
					test.That(t, utils.WriteImageToFile(diffFileName, cmpImg), test.ShouldBeNil)
					test.That(t, utils.WriteImageToFile(thinkFileName, expectedImg), test.ShouldBeNil)
				}
				test.That(t, cmp, test.ShouldBeLessThanOrEqualTo, tc.Diff)
			})
		}
	})
}
