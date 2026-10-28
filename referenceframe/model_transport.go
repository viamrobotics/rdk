package referenceframe

import (
	"bytes"
	"fmt"
	"maps"
	"slices"

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
// the resource, since that is what every client addresses it by. A request that asks for either
// mesh role comes from a client that reads the typed model, so its reply carries the model alone,
// with bytes only for the roles asked for and every other mesh reduced to its path and content
// type. A request with no flags may come from a client that predates them, one that reads the
// file or one that reads the model and expects its meshes whole, so it gets both encodings as a
// server sent them before the flags existed.
func KinematicsResponse(model Model, req *commonpb.GetKinematicsRequest) *commonpb.GetKinematicsResponse {
	collision, visual := req.GetIncludeCollisionMeshes(), req.GetIncludeVisualMeshes()
	if collision || visual {
		if pb, err := ModelToProto(model); err == nil {
			for _, link := range pb.GetLinks() {
				if !collision {
					dropMeshBytes(link.GetCollision())
				}
				if !visual {
					dropMeshBytes(link.GetVisual())
				}
			}
			if !collision {
				for _, joint := range pb.GetJoints() {
					dropMeshBytes([]*commonpb.Geometry{joint.GetGeometry()})
				}
			}
			return stampName(&commonpb.GetKinematicsResponse{Model: pb}, req.GetName())
		}
	}
	// a model that cannot be written as a message is served as the file alone, as it always was
	return stampName(KinematicModelToProtobuf(model), req.GetName())
}

func stampName(resp *commonpb.GetKinematicsResponse, name string) *commonpb.GetKinematicsResponse {
	if name != "" && resp.GetModel() != nil {
		resp.Model.Name = name
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
// meshes keeps the link id for the first and uses the geometry label for the rest, with a number
// added when a label repeats a key already taken. A visual that is not a mesh, or a mesh with no
// bytes, is left out since a viewer can do nothing with it. The bytes are copies.
func VisualMeshes(model Model) map[string]*commonpb.Mesh {
	sm, ok := model.(*SimpleModel)
	if !ok {
		return nil
	}
	meshes := map[string]*commonpb.Mesh{}
	for _, link := range slices.Sorted(maps.Keys(sm.visual)) {
		first := true
		for i, gc := range sm.visual[link] {
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
			for base, n := key, 2; ; n++ {
				if _, taken := meshes[key]; !taken {
					break
				}
				key = fmt.Sprintf("%s_%d", base, n)
			}
			mesh := &commonpb.Mesh{ContentType: gc.MeshContentType, Mesh: bytes.Clone(gc.MeshData)}
			if gc.MeshFilePath != "" {
				mesh.OriginalFileSourcePathDiagnostic = proto.String(gc.MeshFilePath)
			}
			meshes[key] = mesh
		}
	}
	return meshes
}
