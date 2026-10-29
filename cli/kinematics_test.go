package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
)

const convertTestURDF = `<robot name="tester">
  <link name="base">
    <collision>
      <geometry><mesh filename="package://tester/meshes/base.ply"/></geometry>
    </collision>
    <visual>
      <geometry><mesh filename="package://tester/meshes/base.glb"/></geometry>
    </visual>
    <visual>
      <geometry><mesh filename="package://tester/meshes/base.dae"/></geometry>
    </visual>
  </link>
  <link name="tip"/>
  <joint name="j1" type="revolute">
    <origin xyz="0 0 0.1" rpy="0 0 0"/>
    <parent link="base"/>
    <child link="tip"/>
    <axis xyz="0 0 1"/>
    <limit lower="-1.57" upper="1.57" velocity="1.0"/>
  </joint>
</robot>`

func writeConvertFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ply, err := os.ReadFile("../referenceframe/testfiles/test_simple.ply")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, os.MkdirAll(filepath.Join(root, "meshes"), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.ply"), ply, 0o600), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.glb"), []byte("glTF bytes"), 0o600), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "meshes", "base.dae"), []byte("<COLLADA/>"), 0o600), test.ShouldBeNil)
	test.That(t, os.WriteFile(filepath.Join(root, "robot.urdf"), []byte(convertTestURDF), 0o600), test.ShouldBeNil)
	return root
}

func runConvert(t *testing.T, args kinematicsConvertArgs) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cli.Command{Writer: &out}
	err := KinematicsConvertAction(context.Background(), cmd, args)
	return out.String(), err
}

func TestKinematicsConvert(t *testing.T) {
	t.Run("urdf with meshes", func(t *testing.T) {
		root := writeConvertFixture(t)
		outputFile := filepath.Join(t.TempDir(), "tester.json")
		out, err := runConvert(t, kinematicsConvertArgs{InputFile: filepath.Join(root, "robot.urdf"), OutputFile: outputFile})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, out, test.ShouldContainSubstring, "base.dae")
		test.That(t, out, test.ShouldContainSubstring, "Wrote "+outputFile+" with 2 meshes")

		pb, err := referenceframe.ReadKinematicModelFile(outputFile)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pb.GetName(), test.ShouldEqual, "tester")
		test.That(t, string(pb.GetLinks()[0].GetVisual()[0].GetMesh().GetMesh()), test.ShouldEqual, "glTF bytes")
		_, err = os.Stat(filepath.Join(filepath.Dir(outputFile), "meshes", "tester", "collision", "base.ply"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("converts the same way twice", func(t *testing.T) {
		root := writeConvertFixture(t)
		var files [][]byte
		for range 2 {
			outputFile := filepath.Join(t.TempDir(), "tester.json")
			_, err := runConvert(t, kinematicsConvertArgs{InputFile: filepath.Join(root, "robot.urdf"), OutputFile: outputFile})
			test.That(t, err, test.ShouldBeNil)
			raw, err := os.ReadFile(outputFile)
			test.That(t, err, test.ShouldBeNil)
			files = append(files, raw)
		}
		test.That(t, string(files[0]), test.ShouldEqual, string(files[1]))
	})

	t.Run("decimated collision meshes are ply", func(t *testing.T) {
		root := writeConvertFixture(t)
		outputFile := filepath.Join(t.TempDir(), "tester.json")
		_, err := runConvert(t, kinematicsConvertArgs{
			InputFile: filepath.Join(root, "robot.urdf"), OutputFile: outputFile, Decimate: 0.5,
		})
		test.That(t, err, test.ShouldBeNil)
		pb, err := referenceframe.ReadKinematicModelFile(outputFile)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pb.GetLinks()[0].GetCollision()[0].GetMesh().GetContentType(), test.ShouldEqual, "ply")
	})

	t.Run("sva file", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "ur5e.json")
		_, err := runConvert(t, kinematicsConvertArgs{InputFile: "../referenceframe/testfiles/ur5e.json", OutputFile: outputFile})
		test.That(t, err, test.ShouldBeNil)
		pb, err := referenceframe.ReadKinematicModelFile(outputFile)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pb.GetJoints(), test.ShouldHaveLength, 6)
	})

	t.Run("dh file", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "ur5e.json")
		_, err := runConvert(t, kinematicsConvertArgs{InputFile: "../referenceframe/testfiles/ur5eDH.json", OutputFile: outputFile})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("refusals", func(t *testing.T) {
		root := writeConvertFixture(t)
		urdf := filepath.Join(root, "robot.urdf")
		for _, tc := range []struct {
			name string
			args kinematicsConvertArgs
			want string
		}{
			{"output is not json", kinematicsConvertArgs{InputFile: urdf, OutputFile: "out.urdf"}, "must name a .json file"},
			{"ratio out of range", kinematicsConvertArgs{InputFile: urdf, OutputFile: "out.json", Decimate: 1.5}, "must be in (0, 1]"},
			{"package root on sva", kinematicsConvertArgs{
				InputFile: "../referenceframe/testfiles/ur5e.json", OutputFile: "out.json", PackageRoot: root,
			}, "applies to a URDF"},
			{"unknown input", kinematicsConvertArgs{InputFile: "robot.txt", OutputFile: "out.json"}, "not a .urdf or .json file"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := runConvert(t, tc.args)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.want)
			})
		}
	})
}
