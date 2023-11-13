package spatialmath

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
)

func TestGeometrySerializationJSON(t *testing.T) {
	translation := r3.Vector{1, 1, 1}
	orientation := OrientationConfig{}
	testMap := loadOrientationTests(t)
	err := json.Unmarshal(testMap["euler"], &orientation)
	test.That(t, err, test.ShouldBeNil)

	testCases := []struct {
		name    string
		config  GeometryConfig
		success bool
	}{
		{
			"box",
			GeometryConfig{Type: "box", X: 1, Y: 1, Z: 1, TranslationOffset: translation, OrientationOffset: orientation, Label: "box"},
			true,
		},
		{"bounding box dims", GeometryConfig{Type: "box", X: 1, Y: 0, Z: 1, Label: "bounding box dims"}, true},
		{"box bad dims", GeometryConfig{Type: "box", X: 1, Y: 0, Z: -1}, false},
		{"infer box", GeometryConfig{X: 1, Y: 1, Z: 1, Label: "infer box"}, true},
		{"sphere", GeometryConfig{Type: "sphere", R: 1, TranslationOffset: translation, OrientationOffset: orientation, Label: "sphere"}, true},
		{"sphere bad dims", GeometryConfig{Type: "sphere", R: -1}, false},
		{"infer sphere", GeometryConfig{R: 1, OrientationOffset: orientation, Label: "infer sphere"}, true},
		{"point", GeometryConfig{Type: "point", TranslationOffset: translation, OrientationOffset: orientation, Label: "point"}, true},
		{"infer point", GeometryConfig{}, false},
		{"bad type", GeometryConfig{Type: "bad"}, false},
		{"c", GeometryConfig{Type: "capsule", L: 4, R: 1, TranslationOffset: translation, OrientationOffset: orientation, Label: "c"}, true},
		{"infer c", GeometryConfig{L: 4, R: 1, TranslationOffset: translation, OrientationOffset: orientation, Label: "infer c"}, true},
	}

	pose := NewPoseFromPoint(r3.Vector{X: 1, Y: 1, Z: 1})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gc, err := testCase.config.ParseConfig()
			if testCase.success == false {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			data, err := gc.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)
			config := GeometryConfig{}
			err = json.Unmarshal(data, &config)
			test.That(t, err, test.ShouldBeNil)
			newVc, err := config.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, gc.Transform(pose).AlmostEqual(newVc.Transform(pose)), test.ShouldBeTrue)
			test.That(t, config.Label, test.ShouldEqual, testCase.name)
		})
	}
}

func TestGeometryToFromProtobuf(t *testing.T) {
	deg45 := math.Pi / 4
	testCases := []struct {
		name     string
		geometry Geometry
	}{
		{"box", makeTestBox(&EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, "box")},
		{"sphere", makeTestSphere(r3.Vector{3, 4, 5}, 10, "sphere")},
		{"point", NewPoint(r3.Vector{3, 4, 5}, "point")},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			newVol, err := NewGeometryFromProto(testCase.geometry.ToProtobuf())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, testCase.geometry.AlmostEqual(newVol), test.ShouldBeTrue)
			test.That(t, testCase.geometry.Label(), test.ShouldEqual, testCase.name)
		})
	}

	// test that bad message does not generate error
	_, err := NewGeometryFromProto(&commonpb.Geometry{Center: PoseToProtobuf(NewZeroPose())})
	test.That(t, err.Error(), test.ShouldContainSubstring, errGeometryTypeUnsupported.Error())
}

type geometryComparisonTestCase struct {
	testname   string
	geometries [2]Geometry
	expected   float64
}

func testGeometryCollision(t *testing.T, cases []geometryComparisonTestCase) {
	t.Helper()
	for _, c := range cases {
		for i := 0; i < 2; i++ {
			t.Run(fmt.Sprintf("%s %T %T collision", c.testname, c.geometries[i], c.geometries[(i+1)%2]), func(t *testing.T) {
				fn := test.ShouldBeFalse
				if c.expected <= CollisionBuffer {
					fn = test.ShouldBeTrue
				}
				collides, err := c.geometries[i].CollidesWith(c.geometries[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, collides, fn)
			})
			t.Run(fmt.Sprintf("%s %T %T distance", c.testname, c.geometries[i], c.geometries[(i+1)%2]), func(t *testing.T) {
				distance, err := c.geometries[i].DistanceFrom(c.geometries[(i+1)%2])
				test.That(t, err, test.ShouldBeNil)
				test.That(t, distance, test.ShouldAlmostEqual, c.expected, 1e-3)
			})
		}
	}
}

func TestBoxVsBoxCollision(t *testing.T) {
	deg45 := math.Pi / 4.
	cases := []geometryComparisonTestCase{
		{
			"inscribed",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{1, 1, 1}, ""),
			},
			-1.5,
		},
		{
			"face to face contact",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 0, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"face to face near contact",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2.01, 0, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0.01,
		},
		{
			"coincident edge contact",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 4, 0}, r3.Vector{2, 6, 2}, ""),
			},
			0,
		},
		{
			"coincident edges near contact",
			[2]Geometry{
				makeTestBox((NewZeroOrientation()), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 4.01, 0}, r3.Vector{2, 6, 2}, ""),
			},
			0.01,
		},
		{
			"vertex to vertex contact",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"vertex to vertex near contact",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2.01, 2, 2}, r3.Vector{2, 2, 2}, ""),
			},
			0.005,
		},
		{
			"edge along face contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 1 + math.Sqrt2, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"edge along face near contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{deg45, 0, 0}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 1.01 + math.Sqrt2, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0.01,
		},
		{
			"edge to edge contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{0, 0, deg45}, r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt2, 0, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"edge to edge near contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{0, 0, deg45}, r3.Vector{-.01, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(&EulerAngles{0, deg45, 0}, r3.Vector{2 * math.Sqrt2, 0, 0}, r3.Vector{2, 2, 2}, ""),
			},
			0.01,
		},
		{
			"vertex to face contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0.5, -.5, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}, ""),
			},
			-.005,
		},
		{
			"vertex to face near contact",
			[2]Geometry{
				makeTestBox(&EulerAngles{deg45, deg45, 0}, r3.Vector{0, 0, -0.01}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(&EulerAngles{0, 0, 0}, r3.Vector{0, 0, 0.97 + math.Sqrt(3)}, r3.Vector{2, 2, 2}, ""),
			},
			0.005,
		},
		{
			"separated axis aligned",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{5, 6, 0}, r3.Vector{2, 2, 2}, ""),
			},
			4.346, // upper bound on separation distance
		},
		{
			"axis aligned overlap",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{20, 20, 20}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{20, 20, 20}, r3.Vector{24, 26, 28}, ""),
			},
			-2,
		},
		{
			"full overlap",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{10, 10, 10}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{10, 10, 10}, ""),
			},
			-10,
		},
		{
			"zero geometry box",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 0}, r3.Vector{20, 20, 20}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{0, 0, 0}, ""),
			},
			-8,
		},
	}
	testGeometryCollision(t, cases)
}

func TestSphereVsSphereCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"test inscribed spheres",
			[2]Geometry{makeTestSphere(r3.Vector{}, 1, ""), makeTestSphere(r3.Vector{}, 2, "")},
			-3,
		},
		{
			"test tangent spheres",
			[2]Geometry{makeTestSphere(r3.Vector{}, 1, ""), makeTestSphere(r3.Vector{0, 0, 2}, 1, "")},
			0,
		},
		{
			"separated spheres",
			[2]Geometry{makeTestSphere(r3.Vector{}, 1, ""), makeTestSphere(r3.Vector{0, 0, 2 + 1e-3}, 1, "")},
			1e-3,
		},
	}
	testGeometryCollision(t, cases)
}

func TestPointVsPointCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"coincident",
			[2]Geometry{NewPoint(r3.Vector{}, ""), NewPoint(r3.Vector{}, "")},
			0,
		},
		{
			"separated",
			[2]Geometry{NewPoint(r3.Vector{}, ""), NewPoint(r3.Vector{1, 0, 0}, "")},
			1,
		},
	}
	testGeometryCollision(t, cases)
}

func TestSphereVsBoxCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"separated face closest",
			[2]Geometry{
				makeTestSphere(r3.Vector{0, 0, 2 + 1e-3}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			1e-3,
		},
		{
			"separated edge closest",
			[2]Geometry{
				makeTestSphere(r3.Vector{0, 2, 2}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt2 - 1,
		},
		{
			"separated vertex closest",
			[2]Geometry{
				makeTestSphere(r3.Vector{2, 2, 2}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt(3) - 1,
		},
		{
			"face tangent",
			[2]Geometry{
				makeTestSphere(r3.Vector{0, 0, 2}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"edge tangent",
			[2]Geometry{
				makeTestSphere(r3.Vector{0, 2, 2}, math.Sqrt2, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"vertex tangent",
			[2]Geometry{
				makeTestSphere(r3.Vector{2, 2, 2}, math.Sqrt(3), ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"center point inside",
			[2]Geometry{
				makeTestSphere(r3.Vector{-.2, 0.1, .75}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			-1.25,
		},
		{
			"inscribed",
			[2]Geometry{
				makeTestSphere(r3.Vector{2, 2, 2}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{2, 2, 2}, r3.Vector{2, 2, 2}, ""),
			},
			-2,
		},
	}
	testGeometryCollision(t, cases)
}

func TestPointVsBoxCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"separated face closest",
			[2]Geometry{
				NewPoint(r3.Vector{2, 0, 0}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			1,
		},
		{
			"separated edge closest",
			[2]Geometry{
				NewPoint(r3.Vector{2, 2, 0}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt2,
		},
		{
			"separated vertex closest",
			[2]Geometry{
				NewPoint(r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt(3),
		},
		{
			"inside",
			[2]Geometry{
				NewPoint(r3.Vector{0, 0.3, 0.5}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			-0.5,
		},
	}
	testGeometryCollision(t, cases)
}

func TestPointVsSphereCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"coincident",
			[2]Geometry{
				NewPoint(r3.Vector{}, ""),
				makeTestSphere(r3.Vector{}, 1, ""),
			},
			-1,
		},
		{
			"separated",
			[2]Geometry{
				NewPoint(r3.Vector{2, 0, 0}, ""),
				makeTestSphere(r3.Vector{}, 1, ""),
			},
			1,
		},
	}
	testGeometryCollision(t, cases)
}

func testGeometryEncompassed(t *testing.T, cases []geometryComparisonTestCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.testname, func(t *testing.T) {
			fn := test.ShouldBeTrue
			if c.expected > 0.0 {
				fn = test.ShouldBeFalse
			}
			collides, err := c.geometries[0].EncompassedBy(c.geometries[1])
			test.That(t, err, test.ShouldBeNil)
			test.That(t, collides, fn)
		})
	}
}

func TestBoxVsBoxEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 1, 0}, r3.Vector{2, 3, 2}, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			1,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestBoxVsSphereEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
				makeTestSphere(r3.Vector{}, math.Sqrt(3), ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 1, 0}, r3.Vector{2, 2.1, 2}, ""),
				makeTestSphere(r3.Vector{}, math.Sqrt(3), ""),
			},
			.1,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestBoxVsPointEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"coincident",
			[2]Geometry{makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{1, 1, 1}, ""), NewPoint(r3.Vector{}, "")},
			math.Sqrt(3),
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestSphereVsBoxEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestSphere(r3.Vector{3, 0, 0}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{8, 8, 8}, ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestSphere(r3.Vector{3.5, 0, 0}, 1, ""),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{8, 8, 8}, ""),
			},
			0.5,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestSphereVsSphereEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestSphere(r3.Vector{3, 0, 0}, 1, ""),
				makeTestSphere(r3.Vector{}, 4, ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestSphere(r3.Vector{3, 0, 0}, 1, ""),
				makeTestSphere(r3.Vector{}, 3.5, ""),
			},
			0.5,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestSphereVsPointEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"coincident",
			[2]Geometry{makeTestSphere(r3.Vector{}, 1, ""), NewPoint(r3.Vector{}, "")},
			1,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestCapsuleVsBoxCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"separated face closest",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3 + 1e-3}, 1, 4),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			1e-3,
		},
		{
			"separated edge closest",
			[2]Geometry{
				makeTestCapsule(&OrientationVector{0, 0, 1, 1}, r3.Vector{0, 4, 4}, 1, 4*math.Sqrt2),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt2,
		},
		{
			"separated vertex closest",
			[2]Geometry{
				makeTestCapsule(&OrientationVector{0, 2, 2, 2}, r3.Vector{4, 4, 4}, 1, 4*math.Sqrt(3)),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			math.Sqrt(3),
		},
		{
			"face tangent",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3}, 1, 4),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"edge tangent to capsule cylinder",
			[2]Geometry{
				makeTestCapsule(&OrientationVector{0, 0, -2, 2}, r3.Vector{0, 3, 0}, math.Sqrt2/2, 6),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			0,
		},
		{
			"center line segment inside",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0.3, 0.3, -0.75}, 1, 4),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
			},
			-1.7,
		},
		{
			"inscribed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0}, 1, 40),
				makeTestBox(NewZeroOrientation(), r3.Vector{0, 0, 1}, r3.Vector{2, 2, 2}, ""),
			},
			-2,
		},
	}

	adjust := func(n float64) float64 {
		return n * (2 + math.Abs(n) - 1e-3)
	}

	for _, norm := range boxNormals {
		// Test all 6 faces with a tiny collision
		cases = append(cases,
			geometryComparisonTestCase{
				"colliding face closest",
				[2]Geometry{
					makeTestCapsule(&OrientationVector{0, norm.X, norm.Y, norm.Z}, r3.Vector{adjust(norm.X), adjust(norm.Y), adjust(norm.Z)}, 1, 4),
					makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{2, 2, 2}, ""),
				},
				-1e-3,
			},
		)
	}
	testGeometryCollision(t, cases)
}

func TestCapsuleVsCapsuleCollision(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"separated ends closest",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{1e-3, 0, 0}, 1, 4),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{-2, 0, 0}, 1, 4),
			},
			1e-3,
		},
		{
			"separated cylinders closest",
			[2]Geometry{
				makeTestCapsule(&OrientationVector{0, 0, 0, -1}, r3.Vector{0, 0, -2 - 1e-3}, 1, 4),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 2}, 1, 4),
			},
			1e-3,
		},
		{
			"separated cylinder closest to end",
			[2]Geometry{
				makeTestCapsule(&OrientationVector{0, 1, 1, 0}, r3.Vector{0, 0, -1}, 1, 10),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 2 + 1e-3}, 1, 4),
			},
			1e-3,
		},
		{
			"parallel cylinders touching",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{1, 0, 0}, 1, 4),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{-1, 0, 0}, 1, 4),
			},
			0,
		},
		{
			"orthogonal cylinders touching",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0}, 1, 6),
				makeTestCapsule(&OrientationVector{0, 1, 0, 0}, r3.Vector{0, 2, 0}, 1, 6),
			},
			0,
		},
		{
			"orthogonal cylinders slightly colliding",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0}, 1, 6),
				makeTestCapsule(&OrientationVector{0, 1, 0, 0}, r3.Vector{0, 1.8, 0}, 1, 6),
			},
			-0.2,
		},
		{
			"inscribed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 1, 1}, 2, 40),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0}, 4, 40),
			},
			-5,
		},
	}
	testGeometryCollision(t, cases)
}

func TestCapsuleVsBoxEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3}, 1, 4.75),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{16, 16, 16}, ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 5.875}, 1, 4.75),
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{16, 16, 16}, ""),
			},
			0.25,
		},
		{
			"encompassed box",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{4, 4, 4}, ""),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0}, 4, 10),
			},
			0,
		},
		{
			"not encompassed box",
			[2]Geometry{
				makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{16, 16, 16}, ""),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3.5}, 1, 4.75),
			},
			0.25,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestCapsuleVsSphereEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 0.1}, 1, 6.75),
				makeTestSphere(r3.Vector{}, 4, ""),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3}, 1, 6.75),
				makeTestSphere(r3.Vector{}, 3.5, ""),
			},
			0.5,
		},
		{
			"encompassed sphere",
			[2]Geometry{
				makeTestSphere(r3.Vector{}, 2, ""),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 1.5}, 2.5, 9.75),
			},
			0,
		},
		{
			"not encompassed sphere",
			[2]Geometry{
				makeTestSphere(r3.Vector{}, 3.5, ""),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3}, 1, 6.75),
			},
			0.5,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestCapsuleVsCapsuleEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{0, 0, 3}, 1, 3),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 4, 10),
			},
			0,
		},
		{
			"not encompassed",
			[2]Geometry{
				makeTestCapsule(NewZeroOrientation(), r3.Vector{3, 0, 0}, 1, 3),
				makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 3.5, 8),
			},
			0.5,
		},
	}
	testGeometryEncompassed(t, cases)
}

func TestCapsuleVsPointEncompassed(t *testing.T) {
	cases := []geometryComparisonTestCase{
		{
			"coincident",
			[2]Geometry{makeTestCapsule(NewZeroOrientation(), r3.Vector{}, 1, 2), NewPoint(r3.Vector{}, "")},
			1,
		},
	}
	testGeometryEncompassed(t, cases)
}
