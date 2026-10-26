package referenceframe

import (
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/spatialmath"
)

// KinematicsRequest asks a server for a resource's model with every mesh in it. A Go client
// fetches the model once, caches it and answers every later question from it, including a
// request for visual meshes from a client further down the line, so it wants everything.
func KinematicsRequest(name string) *commonpb.GetKinematicsRequest {
	return &commonpb.GetKinematicsRequest{Name: name, IncludeCollisionMeshes: true, IncludeVisualMeshes: true}
}

// KinematicsResponse builds the GetKinematics reply for a resource. The typed model is named for
// the resource, since that is what every client addresses it by, and carries mesh bytes only for
// the roles the request asked for. Every other mesh goes out with its path and content type and
// no bytes, so a client can see what it did not ask for. A request that asks for either role can
// only have come from a client that reads the typed model, so the deprecated fields are left out
// of its reply. A request with no flags cannot be told apart from an old client and keeps both
// encodings, with the deprecated fields carrying the file as they always have.
//
//nolint:staticcheck // the deprecated fields are cleared for clients that read the typed model
func KinematicsResponse(model Model, req *commonpb.GetKinematicsRequest) *commonpb.GetKinematicsResponse {
	resp := KinematicModelToProtobuf(model)
	if resp.GetModel() == nil {
		return resp
	}
	if name := req.GetName(); name != "" {
		resp.Model.Name = name
	}
	collision, visual := req.GetIncludeCollisionMeshes(), req.GetIncludeVisualMeshes()
	for _, link := range resp.Model.GetLinks() {
		if !collision {
			dropMeshBytes(link.GetCollision())
		}
		if !visual {
			dropMeshBytes(link.GetVisual())
		}
	}
	if !collision {
		for _, joint := range resp.Model.GetJoints() {
			dropMeshBytes([]*commonpb.Geometry{joint.GetGeometry()})
		}
	}
	if collision || visual {
		resp.Format = commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED
		resp.KinematicsData = nil
		resp.MeshesByUrdfFilepath = nil
	}
	return resp
}

func dropMeshBytes(geometries []*commonpb.Geometry) {
	for _, g := range geometries {
		if mesh := g.GetMesh(); mesh != nil {
			mesh.Mesh = nil
		}
	}
}

// VisualMeshes returns the visual meshes of a model keyed by link id, which is the shape
// Get3DModels serves and the name a viewer attaches each mesh to. A link with several visual
// meshes keeps the link id for the first and uses the geometry label for the rest. A visual that
// is not a mesh, or a mesh with no bytes, is left out since a viewer can do nothing with it.
func VisualMeshes(model Model) map[string]*commonpb.Mesh {
	sm, ok := model.(*SimpleModel)
	if !ok {
		return nil
	}
	meshes := map[string]*commonpb.Mesh{}
	for link, geometries := range sm.visual {
		first := true
		for i, gc := range geometries {
			if gc.Type != spatialmath.MeshType || len(gc.MeshData) == 0 {
				continue
			}
			key := link
			switch {
			case first:
				first = false
			case gc.Label != "":
				key = gc.Label
			default:
				key = fmt.Sprintf("%s_visual_%d", link, i)
			}
			mesh := &commonpb.Mesh{ContentType: gc.MeshContentType, Mesh: gc.MeshData}
			if gc.MeshFilePath != "" {
				mesh.OriginalFileSourcePathDiagnostic = proto.String(gc.MeshFilePath)
			}
			meshes[key] = mesh
		}
	}
	return meshes
}
