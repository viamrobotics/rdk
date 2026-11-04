package referenceframe

import (
	"os"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// meshModel is a slide with a collision mesh on its base and on the sliding joint, and a visual
// mesh on the base, so every place a mesh can sit on the wire is covered.
func meshModel(t *testing.T) *SimpleModel {
	t.Helper()
	ply, err := os.ReadFile(utils.ResolveFile("referenceframe/testfiles/test_simple.ply"))
	test.That(t, err, test.ShouldBeNil)
	collision := func(path string) *spatial.GeometryConfig {
		return &spatial.GeometryConfig{Type: spatial.MeshType, MeshData: ply, MeshContentType: "ply", MeshFilePath: path}
	}
	cfg := &ModelConfigJSON{
		Name: "meshy",
		Links: []LinkConfig{
			{ID: "base", Parent: World, Geometry: collision("meshes/base.ply")},
			{ID: "tool", Parent: "slide"},
		},
		Joints: []JointConfig{{
			ID: "slide", Parent: "base", Type: PrismaticJoint, Axis: spatial.AxisConfig{Z: 1}, Min: 0, Max: 100,
			Geometry: collision("meshes/slide.ply"),
		}},
	}
	model, err := cfg.ParseConfig("meshy")
	test.That(t, err, test.ShouldBeNil)
	sm, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sm.SetVisualGeometries("base", []spatial.GeometryConfig{{
		Type: spatial.MeshType, MeshData: []byte("glb bytes"), MeshContentType: "glb", MeshFilePath: "meshes/base.glb",
	}}), test.ShouldBeNil)
	return sm
}

func meshOf(resp *commonpb.GetKinematicsResponse) (collision, joint, visual *commonpb.Mesh) {
	model := resp.GetModel()
	return model.GetLinks()[0].GetCollision()[0].GetMesh(),
		model.GetJoints()[0].GetGeometry().GetMesh(),
		model.GetLinks()[0].GetVisual()[0].GetMesh()
}

//nolint:staticcheck // the deprecated fields are what an old client reads
func TestKinematicsResponseFlags(t *testing.T) {
	model := meshModel(t)
	ply := model.ModelConfig().Links[0].Geometry.MeshData

	t.Run("no flags sends both encodings whole", func(t *testing.T) {
		resp := KinematicsResponse(model, &commonpb.GetKinematicsRequest{Name: "arm1"})
		test.That(t, resp.GetModel().GetName(), test.ShouldEqual, "arm1")
		test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA)
		test.That(t, len(resp.GetKinematicsData()), test.ShouldBeGreaterThan, 0)
		collision, joint, visual := meshOf(resp)
		test.That(t, collision.GetMesh(), test.ShouldResemble, ply)
		test.That(t, joint.GetMesh(), test.ShouldResemble, ply)
		test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glb bytes")
		test.That(t, collision.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "meshes/base.ply")

		// a client that reads the model without setting the flags, as the first one did, gets it whole
		got, err := KinematicModelFromProtobuf("", resp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got.Name(), test.ShouldEqual, "arm1")
		assertSameKinematics(t, model, got)

		// an old client reads the file as it always has
		legacy, err := KinematicModelFromProtobuf("arm1", &commonpb.GetKinematicsResponse{
			Format: resp.GetFormat(), KinematicsData: resp.GetKinematicsData(),
		})
		test.That(t, err, test.ShouldBeNil)
		assertSameKinematics(t, model, legacy)
	})

	t.Run("visual flag alone leaves the collision meshes unreadable, and the reader says why", func(t *testing.T) {
		resp := KinematicsResponse(model, &commonpb.GetKinematicsRequest{Name: "arm1", IncludeVisualMeshes: true})
		test.That(t, resp.GetKinematicsData(), test.ShouldBeEmpty)
		collision, joint, visual := meshOf(resp)
		test.That(t, collision.GetMesh(), test.ShouldBeEmpty)
		test.That(t, joint.GetMesh(), test.ShouldBeEmpty)
		test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glb bytes")
		test.That(t, collision.GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "meshes/base.ply")
		test.That(t, collision.GetContentType(), test.ShouldEqual, "ply")

		_, err := ModelFromProto(resp.GetModel(), "")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `collision mesh has no bytes: "meshes/base.ply"`)
		test.That(t, err.Error(), test.ShouldNotContainSubstring, "include_collision_meshes")
		_, err = KinematicModelFromProtobuf("arm1", resp)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "include_collision_meshes")

		// the model itself still holds its bytes
		test.That(t, model.ModelConfig().Links[0].Geometry.MeshData, test.ShouldResemble, ply)
	})

	t.Run("collision flag drops the deprecated fields and sends collision bytes", func(t *testing.T) {
		resp := KinematicsResponse(model, &commonpb.GetKinematicsRequest{Name: "arm1", IncludeCollisionMeshes: true})
		test.That(t, resp.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED)
		test.That(t, resp.GetKinematicsData(), test.ShouldBeEmpty)
		test.That(t, resp.GetMeshesByUrdfFilepath(), test.ShouldBeEmpty)
		collision, joint, visual := meshOf(resp)
		test.That(t, collision.GetMesh(), test.ShouldResemble, ply)
		test.That(t, joint.GetMesh(), test.ShouldResemble, ply)
		test.That(t, visual.GetMesh(), test.ShouldBeEmpty)

		got, err := KinematicModelFromProtobuf("", resp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got.Name(), test.ShouldEqual, "arm1")
		assertSameKinematics(t, model, got)
		sm, ok := got.(*SimpleModel)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, sm.VisualGeometries("base")[0].MeshData, test.ShouldBeEmpty)
		test.That(t, sm.VisualGeometries("base")[0].MeshFilePath, test.ShouldEqual, "meshes/base.glb")
	})

	t.Run("both flags send everything", func(t *testing.T) {
		resp := KinematicsResponse(model, KinematicsRequest("arm1"))
		collision, joint, visual := meshOf(resp)
		test.That(t, collision.GetMesh(), test.ShouldResemble, ply)
		test.That(t, joint.GetMesh(), test.ShouldResemble, ply)
		test.That(t, string(visual.GetMesh()), test.ShouldEqual, "glb bytes")
		got, err := KinematicModelFromProtobuf("arm1", resp)
		test.That(t, err, test.ShouldBeNil)
		meshes := VisualMeshes(got)
		test.That(t, meshes, test.ShouldHaveLength, 1)
		test.That(t, string(meshes["base"].GetMesh()), test.ShouldEqual, "glb bytes")
		test.That(t, meshes["base"].GetContentType(), test.ShouldEqual, "glb")
		test.That(t, meshes["base"].GetOriginalFileSourcePathDiagnostic(), test.ShouldEqual, "meshes/base.glb")
	})

	t.Run("the request a client sends asks for everything", func(t *testing.T) {
		req := KinematicsRequest("arm1")
		test.That(t, req.GetName(), test.ShouldEqual, "arm1")
		test.That(t, req.GetIncludeCollisionMeshes(), test.ShouldBeTrue)
		test.That(t, req.GetIncludeVisualMeshes(), test.ShouldBeTrue)
	})

	t.Run("an empty request name keeps the model's own", func(t *testing.T) {
		resp := KinematicsResponse(model, &commonpb.GetKinematicsRequest{})
		test.That(t, resp.GetModel().GetName(), test.ShouldEqual, "meshy")
	})
}

// A module built before the typed model sends the file alone. The client reads it into a model,
// and the server in front of that client serves the typed model from it, so everyone past the
// server sees a model.
//
//nolint:staticcheck // the deprecated fields are what an old module sends
func TestKinematicsResponseFromOldModule(t *testing.T) {
	original, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "ur5e", nil)
	test.That(t, err, test.ShouldBeNil)
	legacy := KinematicModelToProtobuf(original)
	legacy.Model = nil
	test.That(t, legacy.GetFormat(), test.ShouldEqual, commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF)

	fromClient, err := KinematicModelFromProtobuf("arm1", legacy)
	test.That(t, err, test.ShouldBeNil)
	resp := KinematicsResponse(fromClient, KinematicsRequest("arm1"))
	test.That(t, resp.GetModel(), test.ShouldNotBeNil)
	test.That(t, resp.GetModel().GetName(), test.ShouldEqual, "arm1")
	test.That(t, resp.GetKinematicsData(), test.ShouldBeEmpty)
	served, err := KinematicModelFromProtobuf("", resp)
	test.That(t, err, test.ShouldBeNil)
	assertSameKinematics(t, original, served)
}

// A model that cannot be written as a message is sent as it always was, with nothing to stamp.
func TestKinematicsResponseWithoutMessage(t *testing.T) {
	resp := KinematicsResponse(NewSimpleModel("empty"), KinematicsRequest("arm1"))
	test.That(t, resp.GetModel(), test.ShouldBeNil)
	resp = KinematicsResponse(nil, KinematicsRequest("arm1"))
	test.That(t, resp.GetModel(), test.ShouldBeNil)
}

func TestVisualMeshes(t *testing.T) {
	model := meshModel(t)
	test.That(t, model.SetVisualGeometries("base", []spatial.GeometryConfig{
		{Type: spatial.BoxType, X: 1, Y: 1, Z: 1},
		{Type: spatial.MeshType, MeshData: []byte("first"), MeshContentType: "glb"},
		{Type: spatial.MeshType, MeshContentType: "glb", MeshFilePath: "meshes/no_bytes.glb"},
		{Type: spatial.MeshType, MeshData: []byte("cover"), MeshContentType: "glb", Label: "base_cover"},
		{Type: spatial.MeshType, MeshData: []byte("fourth"), MeshContentType: "glb"},
	}), test.ShouldBeNil)
	test.That(t, model.SetVisualGeometries("tool", []spatial.GeometryConfig{
		{Type: spatial.MeshType, MeshData: []byte("tool"), MeshContentType: "glb"},
	}), test.ShouldBeNil)

	meshes := VisualMeshes(model)
	test.That(t, meshes, test.ShouldHaveLength, 4)
	test.That(t, string(meshes["base"].GetMesh()), test.ShouldEqual, "first")
	test.That(t, string(meshes["base_cover"].GetMesh()), test.ShouldEqual, "cover")
	test.That(t, string(meshes["base_visual_4"].GetMesh()), test.ShouldEqual, "fourth")
	test.That(t, string(meshes["tool"].GetMesh()), test.ShouldEqual, "tool")

	test.That(t, VisualMeshes(NewSimpleModel("bare")), test.ShouldBeEmpty)
	test.That(t, VisualMeshes(nil), test.ShouldBeNil)
}

// A server that fills the deprecated bytes but sends a model without links or joints has not
// migrated yet. The bytes carry the kinematics, so the client reads those instead.
func TestKinematicModelFromProtobufHollowModel(t *testing.T) {
	model := meshModel(t)
	resp := KinematicModelToProtobuf(model)
	test.That(t, len(resp.GetKinematicsData()), test.ShouldBeGreaterThan, 0) //nolint:staticcheck // deprecated fields are still served
	resp.Model = &commonpb.KinematicModel{Name: "stopgap"}

	got, err := KinematicModelFromProtobuf("arm1", resp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Name(), test.ShouldEqual, "arm1")
	assertSameKinematics(t, model, got)

	// with no bytes either, an empty model is all the server has, and it keeps what the server put on it
	resp.KinematicsData = nil //nolint:staticcheck // deprecated fields are still served
	//nolint:staticcheck // deprecated fields are still served
	resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED
	resp.Model.Properties = &commonpb.KinematicProperties{OperatingFreqHz: proto.Float64(30)}
	got, err = KinematicModelFromProtobuf("arm1", resp)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Name(), test.ShouldEqual, "arm1")
	test.That(t, got.DoF(), test.ShouldBeEmpty)
	sm, ok := got.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proto.Equal(propertiesToProto(sm.KinematicProperties()), resp.Model.Properties), test.ShouldBeTrue)
}

// A visual whose label repeats a key taken by another mesh gets a number instead of hiding it,
// and the bytes handed out are copies.
func TestVisualMeshesKeysAndCopies(t *testing.T) {
	model := meshModel(t)
	test.That(t, model.SetVisualGeometries("tool", []spatial.GeometryConfig{
		{Type: spatial.MeshType, MeshData: []byte("tool"), MeshContentType: "glb"},
		{Type: spatial.MeshType, MeshData: []byte("clash"), MeshContentType: "glb", Label: "base"},
	}), test.ShouldBeNil)
	meshes := VisualMeshes(model)
	test.That(t, meshes, test.ShouldHaveLength, 3)
	test.That(t, string(meshes["base"].GetMesh()), test.ShouldEqual, "glb bytes")
	test.That(t, string(meshes["tool"].GetMesh()), test.ShouldEqual, "tool")
	test.That(t, string(meshes["base_2"].GetMesh()), test.ShouldEqual, "clash")

	meshes["tool"].Mesh[0] = 'X'
	test.That(t, string(VisualMeshes(model)["tool"].GetMesh()), test.ShouldEqual, "tool")
}
