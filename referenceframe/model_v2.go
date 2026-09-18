package referenceframe

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// The mesh formats a collision geometry may use. RDK has to parse a collision mesh, so it is
// limited to what spatialmath reads. A visual mesh is passed through untouched and may be
// anything a viewer can load.
var collisionMeshTypes = map[string]bool{"stl": true, "ply": true}

// ParseModelV2File reads an SVA v2 file, which is the proto JSON encoding of KinematicModel, and
// builds a model from it. Meshes referenced by source_path are loaded relative to the file. An
// empty name keeps the name in the file.
func ParseModelV2File(filename, modelName string) (Model, error) {
	//nolint:gosec
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read kinematics file")
	}
	return UnmarshalModelV2(data, filepath.Dir(filename), modelName)
}

// UnmarshalModelV2 builds a model from SVA v2 bytes. baseDir is where relative source_path entries
// resolve; pass "" when the bytes did not come from a file, in which case any mesh that is
// referenced by path alone is an error, since there is nowhere to load it from.
func UnmarshalModelV2(data []byte, baseDir, modelName string) (Model, error) {
	pb, err := LoadKinematicModelV2(data, baseDir)
	if err != nil {
		return nil, err
	}
	return ModelFromProto(pb, modelName)
}

// LoadKinematicModelV2 parses SVA v2 bytes strictly, so an unknown field is an error rather than
// something silently ignored, and then runs the post-pass every SDK shares: resolve and load each
// mesh referenced by source_path, fill content types from the file extension, and refuse the
// values proto JSON accepts but a kinematic model cannot use, such as Infinity in a limit.
func LoadKinematicModelV2(data []byte, baseDir string) (*commonpb.KinematicModel, error) {
	pb := &commonpb.KinematicModel{}
	if err := protojson.Unmarshal(data, pb); err != nil {
		return nil, errors.Wrap(err, "not a valid SVA v2 kinematics file")
	}
	for _, link := range pb.GetLinks() {
		for i, g := range link.GetCollision() {
			if err := loadMesh(g, baseDir, true); err != nil {
				return nil, errors.Wrapf(err, "link %q collision geometry %d", link.GetId(), i)
			}
		}
		for i, g := range link.GetVisual() {
			if err := loadMesh(g, baseDir, false); err != nil {
				return nil, errors.Wrapf(err, "link %q visual geometry %d", link.GetId(), i)
			}
		}
	}
	for _, joint := range pb.GetJoints() {
		if err := loadMesh(joint.GetGeometry(), baseDir, true); err != nil {
			return nil, errors.Wrapf(err, "joint %q geometry", joint.GetId())
		}
		for _, limits := range []*commonpb.JointLimits{joint.GetHardwareLimits(), joint.GetUserLimits()} {
			if err := checkFiniteLimits(limits); err != nil {
				return nil, errors.Wrapf(err, "joint %q", joint.GetId())
			}
		}
	}
	return pb, nil
}

// loadMesh fills in a mesh that a file referenced by path. The rules are the ones in the scope:
// the path is relative to the file, never absolute, never a URI, and the content type comes from
// the extension. A collision mesh has to be a format RDK can parse.
func loadMesh(g *commonpb.Geometry, baseDir string, collision bool) error {
	mesh := g.GetMesh()
	if mesh == nil {
		return nil
	}
	if len(mesh.GetMesh()) == 0 && mesh.GetSourcePath() != "" {
		source := mesh.GetSourcePath()
		switch {
		case strings.Contains(source, "://"):
			return fmt.Errorf("source_path %q must be a plain relative path, not a URI", source)
		case filepath.IsAbs(source) || strings.HasPrefix(source, "/"):
			return fmt.Errorf("source_path %q must be relative to the kinematics file, not absolute", source)
		case baseDir == "":
			return fmt.Errorf("source_path %q cannot be resolved, the model did not come from a file", source)
		}
		resolved := filepath.Join(baseDir, filepath.FromSlash(source))
		//nolint:gosec
		data, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("mesh %q (resolved to %q) could not be read: %w", source, resolved, err)
		}
		mesh.Mesh = data
	}
	if mesh.GetSourcePath() != "" {
		fromExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(mesh.GetSourcePath())), ".")
		switch {
		case mesh.GetContentType() == "":
			mesh.ContentType = fromExt
		case fromExt != "" && !strings.EqualFold(mesh.GetContentType(), fromExt):
			return fmt.Errorf("content_type %q disagrees with the %q extension of %q", mesh.GetContentType(), fromExt, mesh.GetSourcePath())
		}
	}
	if collision && !collisionMeshTypes[strings.ToLower(mesh.GetContentType())] {
		return fmt.Errorf("collision meshes must be stl or ply, not %q", mesh.GetContentType())
	}
	return nil
}

func checkFiniteLimits(limits *commonpb.JointLimits) error {
	if limits == nil {
		return nil
	}
	for name, v := range map[string]*float64{
		"min": limits.Min, "max": limits.Max, "max_velocity": limits.MaxVelocity, "max_acceleration": limits.MaxAcceleration,
	} {
		if v != nil && (math.IsInf(*v, 0) || math.IsNaN(*v)) {
			return fmt.Errorf("%s must be finite; leave it out to mean unbounded", name)
		}
	}
	return nil
}
