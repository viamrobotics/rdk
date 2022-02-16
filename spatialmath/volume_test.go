package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestVolumeSerialization(t *testing.T) {
	translation := TranslationConfig{1, 1, 1}
	orientation := OrientationConfig{}
	testMap := loadOrientationTests(t)
	err := json.Unmarshal(testMap["euler"], &orientation)
	test.That(t, err, test.ShouldBeNil)

	testCases := []struct {
		name    string
		config  VolumeConfig
		success bool
	}{
		{"box", VolumeConfig{Type: "box", X: 1, Y: 1, Z: 1, TranslationOffset: translation, OrientationOffset: orientation}, true},
		{"box bad dims", VolumeConfig{Type: "box", X: 1, Y: 0, Z: 1}, false},
		{"infer box", VolumeConfig{X: 1, Y: 1, Z: 1}, true},
		{"sphere", VolumeConfig{Type: "sphere", R: 1, TranslationOffset: translation, OrientationOffset: orientation}, true},
		{"sphere bad dims", VolumeConfig{Type: "sphere", R: -1}, false},
		{"infer sphere", VolumeConfig{R: 1, OrientationOffset: orientation}, true},
		{"point", VolumeConfig{Type: "point", TranslationOffset: translation, OrientationOffset: orientation}, true},
		{"infer point", VolumeConfig{}, false},
		{"bad type", VolumeConfig{Type: "bad"}, false},
	}

	pose := NewPoseFromPoint(r3.Vector{X: 1, Y: 1, Z: 1})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			vc, err := testCase.config.ParseConfig()
			if testCase.success == false {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			data, err := vc.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)
			config := VolumeConfig{}
			err = json.Unmarshal(data, &config)
			test.That(t, err, test.ShouldBeNil)
			newVc, err := config.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, vc.NewVolume(pose).AlmostEqual(newVc.NewVolume(pose)), test.ShouldBeTrue)
		})
	}
}

type volumeComparisonTestCase struct {
	testname string
	volumes  [2]Volume
	expected float64
}

func testVolumeComparisons(t *testing.T, cases []volumeComparisonTestCase) {
	t.Helper()
	for _, c := range cases {
		for i := 0; i < 2; i++ {
			t.Run(fmt.Sprintf("%s %T %T collision", c.testname, c.volumes[i], c.volumes[(i+1)%2]), func(t *testing.T) {
				fn := test.ShouldBeFalse
				if c.expected <= 0.0 {
					fn = test.ShouldBeTrue
				}
				collides, err := c.volumes[i].CollidesWith(c.volumes[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, collides, fn)
			})
			t.Run(fmt.Sprintf("%s %T %T distance", c.testname, c.volumes[i], c.volumes[(i+1)%2]), func(t *testing.T) {
				distance, err := c.volumes[i].DistanceFrom(c.volumes[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, distance, test.ShouldAlmostEqual, c.expected, 1e-3)
			})
		}
	}
}

func TestBoxVsBox(t *testing.T) {
	deg45 := math.Pi / 4.
	cases := []volumeComparisonTestCase{
		{
			"inscribed",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}),
			},
			-1.5,
		},
		{
			"face to face contact",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2, 0, 0}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"face to face near contact",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2.01, 0, 0}, r3.Vector{2, 2, 2}),
			},
			0.01,
		},
		{
			"coincident edge contact",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2, 4, 0}, r3.Vector{2, 6, 2}),
			},
			0,
		},
		{
			"coincident edges near contact",
			[2]Volume{
				makeBox((NewZeroOrientation()), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2, 4.01, 0}, r3.Vector{2, 6, 2}),
			},
			0.01,
		},
		{
			"vertex to vertex contact",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"vertex to vertex near contact",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{2.01, 2, 2}, r3.Vector{2, 2, 2}),
			},
			0.01,
		},
		{
			"edge along face contact",
			[2]Volume{
				makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{0, 1 + math.Sqrt2, 0}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"edge along face near contact",
			[2]Volume{
				makeBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{0, 1.01 + math.Sqrt2, 0}, r3.Vector{2, 2, 2}),
			},
			0.01,
		},
		{
			"edge to edge contact",
			[2]Volume{
				makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt2, 0, 0}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"edge to edge near contact",
			[2]Volume{
				makeBox(&EulerAngles{0, 0, deg45}, r3.Vector{-.01, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt2, 0, 0}, r3.Vector{2, 2, 2}),
			},
			0.01,
		},
		{
			"vertex to face contact",
			[2]Volume{
				makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0.5, -.5, 0}, r3.Vector{2, 2, 2}),
				makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}),
			},
			-.005,
		},
		{
			"vertex to face near contact",
			[2]Volume{
				makeBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0, 0, -0.01}, r3.Vector{2, 2, 2}),
				makeBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}),
			},
			0.005,
		},
		{
			"separated axis aligned",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{5, 6, 0}, r3.Vector{2, 2, 2}),
			},
			4, // upper bound on separation distance
		},
		{
			"axis aligned overlap",
			[2]Volume{
				makeBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{20, 20, 20}),
				makeBox(NewZeroOrientation(), r3.Vector{20, 20, 20}, r3.Vector{24, 26, 28}),
			},
			-2,
		},
	}
	testVolumeComparisons(t, cases)
}

func TestSphereVsSphere(t *testing.T) {
	cases := []volumeComparisonTestCase{
		{
			"test inscribed spheres",
			[2]Volume{makeSphere(r3.Vector{}, 1), makeSphere(r3.Vector{}, 2)},
			-3,
		},
		{
			"test tangent spheres",
			[2]Volume{makeSphere(r3.Vector{}, 1), makeSphere(r3.Vector{0, 0, 2}, 1)},
			0,
		},
		{
			"separated spheres",
			[2]Volume{makeSphere(r3.Vector{}, 1), makeSphere(r3.Vector{0, 0, 2 + 1e-3}, 1)},
			1e-3,
		},
	}
	testVolumeComparisons(t, cases)
}

func TestPointVsPoint(t *testing.T) {
	cases := []volumeComparisonTestCase{
		{
			"coincident",
			[2]Volume{makePoint(r3.Vector{}), makePoint(r3.Vector{})},
			0,
		},
		{
			"separated",
			[2]Volume{makePoint(r3.Vector{}), makePoint(r3.Vector{1, 0, 0})},
			1,
		},
	}
	testVolumeComparisons(t, cases)
}

func TestSphereVsBox(t *testing.T) {
	cases := []volumeComparisonTestCase{
		{
			"separated face closest",
			[2]Volume{
				makeSphere(r3.Vector{0, 0, 2 + 1e-3}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			1e-3,
		},
		{
			"separated edge closest",
			[2]Volume{
				makeSphere(r3.Vector{0, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt2 - 1,
		},
		{
			"separated vertex closest",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt(3) - 1,
		},
		{
			"face tangent",
			[2]Volume{
				makeSphere(r3.Vector{0, 0, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"edge tangent",
			[2]Volume{
				makeSphere(r3.Vector{0, 2, 2}, math.Sqrt2),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"vertex tangent",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, math.Sqrt(3)),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			0,
		},
		{
			"center point inside",
			[2]Volume{
				makeSphere(r3.Vector{-.2, 0.1, .75}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			-1.25,
		},
		{
			"inscribed",
			[2]Volume{
				makeSphere(r3.Vector{2, 2, 2}, 1),
				makeBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}),
			},
			-2,
		},
	}
	testVolumeComparisons(t, cases)
}

func TestPointVsBox(t *testing.T) {
	cases := []volumeComparisonTestCase{
		{
			"separated face closest",
			[2]Volume{
				makePoint(r3.Vector{2, 0, 0}),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			1,
		},
		{
			"separated edge closest",
			[2]Volume{
				makePoint(r3.Vector{2, 2, 0}),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt2,
		},
		{
			"separated vertex closest",
			[2]Volume{
				makePoint(r3.Vector{2, 2, 2}),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			math.Sqrt(3),
		},
		{
			"inside",
			[2]Volume{
				makePoint(r3.Vector{0, 0.3, 0.5}),
				makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}),
			},
			-0.5,
		},
	}
	testVolumeComparisons(t, cases)
}

func TestPointVsSphere(t *testing.T) {
	cases := []volumeComparisonTestCase{
		{
			"coincident",
			[2]Volume{
				makePoint(r3.Vector{}),
				makeSphere(r3.Vector{}, 1),
			},
			-1,
		},
		{
			"separated",
			[2]Volume{
				makePoint(r3.Vector{2, 0, 0}),
				makeSphere(r3.Vector{}, 1),
			},
			1,
		},
	}
	testVolumeComparisons(t, cases)
}
