package models3d

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/referenceframe"
)

func TestAttachVisuals(t *testing.T) {
	for _, armModel := range []string{"ur5e", "ur20", "xarm6", "lite6"} {
		t.Run(armModel, func(t *testing.T) {
			m, err := kinematics.ModelFromName(armModel, "arm")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, AttachVisuals(m, armModel), test.ShouldBeNil)
			meshes := referenceframe.VisualMeshes(m)
			want := Visuals(armModel)
			test.That(t, len(meshes), test.ShouldEqual, len(want))
			for link, glb := range want {
				test.That(t, meshes[link].GetMesh(), test.ShouldResemble, glb)
				test.That(t, meshes[link].GetContentType(), test.ShouldEqual, ContentType)
				test.That(t, meshes[link].GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, armModel+"/"+link+".glb")
			}
		})
	}

	t.Run("an arm model with no visuals is left alone", func(t *testing.T) {
		m, err := kinematics.ModelFromName(kinematics.Fake, "arm")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, AttachVisuals(m, kinematics.Fake), test.ShouldBeNil)
		test.That(t, referenceframe.VisualMeshes(m), test.ShouldBeEmpty)
		test.That(t, Visuals(kinematics.Fake), test.ShouldBeNil)
	})

	t.Run("a link the model does not have is an error", func(t *testing.T) {
		m, err := kinematics.ModelFromName(kinematics.Fake, "arm")
		test.That(t, err, test.ShouldBeNil)
		err = AttachVisuals(m, "ur5e")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `arm model "ur5e"`)
	})
}
