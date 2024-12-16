package arm_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testArmName    = "arm1"
	testArmName2   = "arm2"
	failArmName    = "arm3"
	missingArmName = "arm4"
)

func TestXArm6Locations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fake.Config{
			ModelFilePath: "example_kinematics/xarm6_kinematics_test.json",
		},
	}

	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("home location check", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			402.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			102.997474683058328537299531,
			0.000000000000000000000000,
			502.002525316941671462700469,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			131.000000000000000000000000,
			-27.500000000000000000000000,
			274.200000000000000000000000,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			206.000000000000000000000000,
			10.000000000000000000000000,
			141.500000000000000000000000,
		}

		in := make([]referenceframe.Input, 6)
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check1", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-13.477511247321800169629569,
			0.000000000000000000000000,
			401.325562312533520525903441,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			83.174566602088916056345624,
			0.000000000000000000000000,
			516.742582359123957758129109,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			158.566974381217988820935716,
			-27.362614545145717670493468,
			299.589614460540474283334333,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			259.050559200277916716004256,
			18.072894555453149934010071,
			192.543551746158527748775668,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check2", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-26.820359657333263214695762,
			0.000000000000000000000000,
			399.308988008567553151806351,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			60.777555075062835499011271,
			0.000000000000000000000000,
			530.142781900699674224597402,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			180.312201371473605604478507,
			-26.951830890634148829576588,
			333.355009225598280409030849,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			297.258065257027055849903263,
			27.068045067389423508075197,
			256.363984524505951867467957,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
}

func TestUR5ELocations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}

	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("home location check", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-817.200000000000045474735089,
			-66.649999999999948840923025,
			162.500000000000000000000000,
		}
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-817.200000000000045474735089,
			-133.300000000000011368683772,
			112.650000000000005684341886,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-817.200000000000045474735089,
			-183.149999999999920419213595,
			62.799999999999940314410196,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-212.500000000000000000000000,
			-130.000000000000000000000000,
			162.499999999999971578290570,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-621.100000000000022737367544,
			0.000000000000000000000000,
			162.500000000000000000000000,
		}

		in := make([]referenceframe.Input, 6)
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check1", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-821.990564374563746241619810,
			-133.300000000000068212102633,
			235.223789629813609280972742,
		}
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-807.258882072496135151595809,
			-66.650000000000062527760747,
			282.847313612725088205479551,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-831.967827564655408423277549,
			-182.900957639109606134297792,
			186.129551469731126189799397,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-211.438385121580523673401331,
			-130.000000000000028421709430,
			183.714601037450989906574250,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-615.067826157828449140652083,
			-0.000000000000000000000000,
			243.888257843813590852732887,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check2", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-777.768417430459294337197207,
			-66.650000000000005684341886,
			399.664339441353661186440149,
		}
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-805.915844729201694462972227,
			-133.300000000000011368683772,
			358.521359038106197658635210,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-825.889423644316707395773847,
			-182.156318905385916195882601,
			311.786348089814850936818402,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-208.264147791263866338340449,
			-130.000000000000028421709430,
			204.717232793950500990831642,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-597.148356506493314554973040,
			-0.000000000000000000000000,
			323.299402514627388427470578,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
}

func locationCheckTestHelper(geomList []spatialmath.Geometry, checkMap map[string]r3.Vector) bool {
	for _, g := range geomList {
		vecCheck := checkMap[g.Label()]
		vecActual := g.Pose().Point()
		if !spatialmath.R3VectorAlmostEqual(vecCheck, vecActual, 1e-2) {
			return false
		}
	}
	return true
}

func TestFromRobot(t *testing.T) {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{
		arm.Named("arm1"):  inject.NewArm("arm1"),
		generic.Named("g"): inject.NewGenericComponent("g"),
	}
	r.MockResourcesFromMap(rs)

	expected := []string{"arm1"}
	testutils.VerifySameElements(t, arm.NamesFromRobot(r), expected)

	_, err := arm.FromRobot(r, "arm1")
	test.That(t, err, test.ShouldBeNil)

	_, err = arm.FromRobot(r, "arm0")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = arm.FromRobot(r, "g")
	test.That(t, err, test.ShouldNotBeNil)
}
