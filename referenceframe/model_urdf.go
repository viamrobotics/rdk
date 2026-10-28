package referenceframe

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ModelConfigURDF represents all supported fields in a Universal Robot Description Format (URDF) file.
type ModelConfigURDF struct {
	XMLName xml.Name   `xml:"robot"`
	Name    string     `xml:"name,attr"`
	Links   []linkXML  `xml:"link"`
	Joints  []jointXML `xml:"joint"`
}

// linkXML is a struct which details the XML used in a URDF linkXML element.
type linkXML struct {
	XMLName   xml.Name    `xml:"link"`
	Name      string      `xml:"name,attr"`
	Collision []collision `xml:"collision"`
	Visual    []visual    `xml:"visual"`
}

// jointXML is a struct which details the XML used in a URDF jointXML element.
type jointXML struct {
	XMLName xml.Name  `xml:"joint"`
	Name    string    `xml:"name,attr"`
	Type    string    `xml:"type,attr"`
	Parent  frame     `xml:"parent"`
	Child   frame     `xml:"child"`
	Origin  *pose     `xml:"origin,omitempty"`
	Axis    *axis     `xml:"axis,omitempty"`
	Limit   *limit    `xml:"limit,omitempty"`
	Mimic   *mimicXML `xml:"mimic,omitempty"`
}

// NewModelFromWorldState creates a ModelConfigURDF struct which can be marshalled into xml and will be a
// valid .urdf file representing the geometries in the given worldstate.
func NewModelFromWorldState(ws *WorldState, name string) (*ModelConfigURDF, error) {
	// the link we initialize this list with represents the world frame
	links := []linkXML{{Name: World}}
	joints := make([]jointXML, 0)
	emptyFS := NewEmptyFrameSystem("")
	gf, err := ws.ObstaclesInWorldFrame(emptyFS, NewNeutralFrameSystemInputs(emptyFS))
	if err != nil {
		return nil, err
	}
	for _, g := range gf.Geometries() {
		colls, err := newCollisions(g)
		if err != nil {
			return nil, err
		}
		links = append(links, linkXML{
			Name:      g.Label(),
			Collision: colls,
		})
		joints = append(joints, jointXML{
			Name:   g.Label() + "_joint",
			Type:   "fixed",
			Parent: frame{gf.Parent()},
			Child:  frame{g.Label()},
			Origin: newPose(spatialmath.NewZeroPose()),
		})
	}
	return &ModelConfigURDF{
		Name:   name,
		Links:  links,
		Joints: joints,
	}, nil
}

// UnmarshalModelXML will transfer the given URDF XML data into an equivalent ModelConfig. Direct unmarshaling in the
// same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accommodate differences
// between the two kinematics encoding schemes.
// The meshMap parameter provides mesh proto messages keyed by URDF file path (e.g., "meshes/base.stl" -> proto Mesh).
// The meshDecimationRatios parameter controls per-mesh decimation: meshDecimationRatios[i] is the fraction of
// triangles to keep for the i-th mesh collision geometry in the URDF (in document order, skipping capsules and
// non-mesh collisions). Values must be in (0, 1] where 1.0 means no decimation. If nil or shorter than the
// number of mesh collisions, unspecified meshes are left undecimated.
func UnmarshalModelXML(
	xmlData []byte,
	modelName string,
	meshMap map[string]*commonpb.Mesh,
	meshDecimationRatios []float64,
) (*ModelConfigJSON, error) {
	// Unmarshal into a URDF ModelConfig
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal(xmlData, urdf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert URDF data to equivalent URDFConfig struct")
	}

	// Use default name if none is provided
	if modelName == "" {
		modelName = urdf.Name
	}

	// Read all links first
	links := make(map[string]*LinkConfig, 0)
	meshIndex := 0
	for _, linkElem := range urdf.Links {
		// Skip any world links
		if linkElem.Name == World {
			continue
		}

		link := &LinkConfig{ID: linkElem.Name}
		if len(linkElem.Collision) > 0 {
			var geometry spatialmath.Geometry
			var err error

			// Try to detect capsule pattern (cylinder + 2 spheres)
			geometry, err = tryParseCapsuleFromCollisions(linkElem.Collision)
			if err != nil {
				return nil, fmt.Errorf("failed to parse capsule from collision geometries %v: %w", linkElem.Name, err)
			}

			// If not a capsule, fall back to first collision element
			if geometry == nil {
				// Look up per-mesh decimation ratio if this is a mesh collision.
				// Only mesh geometries support decimation; boxes, spheres, and cylinders are skipped.
				// meshIndex tracks position in the meshDecimationRatios array across mesh collisions only.
				decimationRatio := 1.0 // 1.0 = keep all triangles (no decimation)
				if linkElem.Collision[0].Geometry.Mesh != nil {
					if meshIndex < len(meshDecimationRatios) {
						decimationRatio = meshDecimationRatios[meshIndex]
					}
					meshIndex++
				}
				geometry, err = linkElem.Collision[0].toGeometry(meshMap, decimationRatio)
				if err != nil {
					return nil, fmt.Errorf("failed to convert collision geometry %v to geometry config: %w", linkElem.Name, err)
				}
			}

			geoCfg, err := spatialmath.NewGeometryConfig(geometry)
			if err != nil {
				return nil, err
			}
			link.Geometry = geoCfg
		}
		links[linkElem.Name] = link
	}

	// Read the joints next
	joints := make([]JointConfig, 0)
	for _, jointElem := range urdf.Joints {
		switch jointElem.Type {
		case ContinuousJoint, RevoluteJoint, PrismaticJoint:
			// Parse important details about each joint, including axes and limits
			thisJoint := JointConfig{
				ID:     jointElem.Name,
				Type:   jointElem.Type,
				Parent: jointElem.Parent.Link,
			}
			if jointElem.Axis != nil {
				thisJoint.Axis = jointElem.Axis.Parse()
			}

			// Slightly different limits handling for continuous, revolute, and prismatic joints
			switch jointElem.Type {
			case ContinuousJoint:
				thisJoint.Type = RevoluteJoint // Currently, we treate a continuous joint as a special case of a revolute joint
				thisJoint.Min, thisJoint.Max = math.Inf(-1), math.Inf(1)
			case PrismaticJoint:
				thisJoint.Min, thisJoint.Max = utils.MetersToMM(jointElem.Limit.Lower), utils.MetersToMM(jointElem.Limit.Upper)
			case RevoluteJoint:
				thisJoint.Min, thisJoint.Max = utils.RadToDeg(jointElem.Limit.Lower), utils.RadToDeg(jointElem.Limit.Upper)
			default:
				return nil, err
			}
			if jointElem.Mimic != nil {
				thisJoint.Mimic = &MimicConfig{
					Joint:           jointElem.Mimic.Joint,
					ValueMultiplier: jointElem.Mimic.ValueMultiplier,
					ValueOffset:     jointElem.Mimic.ValueOffset,
				}
				// URDF requires <limit> on revolute/prismatic joints, but limits are
				// meaningless for mimic joints. Clear them so buildMimicMappings
				// validation passes; the source joint's limits govern the range.
				thisJoint.Min, thisJoint.Max = 0, 0
			}
			joints = append(joints, thisJoint)

			// Generate child link translation and orientation data, which is held by this joint per the URDF design
			childXYZ := spaceDelimitedStringToFloatSlice(jointElem.Origin.XYZ)
			childRPY := spaceDelimitedStringToFloatSlice(jointElem.Origin.RPY)
			childOrient, err := spatialmath.NewOrientationConfig(&spatialmath.EulerAngles{
				Roll:  childRPY[0],
				Pitch: childRPY[1],
				Yaw:   childRPY[2],
			})
			if err != nil {
				return nil, err
			}

			// Add the transformation to the parent link which should be in the map of links
			parentLink, ok := links[jointElem.Parent.Link]
			if !ok {
				return nil, NewFrameNotInListOfTransformsError(jointElem.Parent.Link)
			}
			parentLink.Translation = r3.Vector{
				X: utils.MetersToMM(childXYZ[0]),
				Y: utils.MetersToMM(childXYZ[1]),
				Z: utils.MetersToMM(childXYZ[2]),
			}
			parentLink.Orientation = childOrient

		case FixedJoint:
			// Handle fixed joints by converting them to links rather than a joint
			linkXYZ := spaceDelimitedStringToFloatSlice(jointElem.Origin.XYZ)
			linkRPY := spaceDelimitedStringToFloatSlice(jointElem.Origin.RPY)
			linkOrient, err := spatialmath.NewOrientationConfig(&spatialmath.EulerAngles{
				Roll:  linkRPY[0],
				Pitch: linkRPY[1],
				Yaw:   linkRPY[2],
			})
			if err != nil {
				return nil, err
			}

			link := &LinkConfig{
				ID:          jointElem.Name,
				Translation: r3.Vector{X: utils.MetersToMM(linkXYZ[0]), Y: utils.MetersToMM(linkXYZ[1]), Z: utils.MetersToMM(linkXYZ[2])},
				Orientation: linkOrient,
				Parent:      jointElem.Parent.Link,
			}
			links[jointElem.Name] = link
		default:
			return nil, NewUnsupportedJointTypeError(jointElem.Type)
		}

		// Point the child link to this joint
		childLink, ok := links[jointElem.Child.Link]
		if !ok {
			return nil, NewFrameNotInListOfTransformsError(jointElem.Child.Link)
		}
		childLink.Parent = jointElem.Name
	}

	// Return as a ModelConfig
	linkSlice := make([]LinkConfig, 0, len(links))
	for _, link := range links {
		linkSlice = append(linkSlice, *link)
	}
	return &ModelConfigJSON{
		Name:         modelName,
		KinParamType: "SVA",
		Links:        linkSlice,
		Joints:       joints,
		OriginalFile: &ModelFile{
			Bytes:     xmlData,
			Extension: "urdf",
		},
	}, nil
}

// buildMeshMapFromURDF extracts mesh file paths from URDF and loads their bytes from disk.
// It resolves paths relative to the URDF file's directory and handles package:// URIs.
// Note: This function is only used when we are reading a URDF file, not when a URDF is sent over the wire.
func buildMeshMapFromURDF(xmlData []byte, urdfDir, packageRoot string) (map[string]*commonpb.Mesh, error) {
	// Parse URDF to find mesh references
	urdf := &ModelConfigURDF{}
	if err := xml.Unmarshal(xmlData, urdf); err != nil {
		return nil, errors.Wrap(err, "failed to parse URDF for mesh extraction")
	}

	meshMap := make(map[string]*commonpb.Mesh)

	// Iterate through all links and their collision geometries
	for _, link := range urdf.Links {
		for _, collision := range link.Collision {
			if collision.Geometry.Mesh == nil {
				continue
			}

			originalPath := collision.Geometry.Mesh.Filename
			meshPath, absolutePath := resolveURDFMeshPath(originalPath, urdfDir, packageRoot)

			// Check if we've already loaded this mesh
			if _, exists := meshMap[meshPath]; exists {
				continue
			}

			// Load mesh file bytes
			//nolint:gosec
			meshBytes, err := os.ReadFile(absolutePath)
			if err != nil {
				return nil, fmt.Errorf("failed to load mesh file %s (referenced as %s): %w", absolutePath, originalPath, err)
			}

			// Determine mesh content type from file extension
			var contentType string
			if strings.HasSuffix(strings.ToLower(meshPath), ".ply") {
				contentType = "ply"
			} else if strings.HasSuffix(strings.ToLower(meshPath), ".stl") {
				contentType = "stl"
			} else {
				return nil, fmt.Errorf("unsupported mesh file type (only .ply and .stl supported): %s", meshPath)
			}

			meshMap[meshPath] = &commonpb.Mesh{
				Mesh:        meshBytes,
				ContentType: contentType,
			}
		}
	}

	return meshMap, nil
}

// ParseModelXMLFile will read a given file and parse the contained URDF XML data into an equivalent Model.
// It automatically loads mesh files referenced in the URDF from the local filesystem.
func ParseModelXMLFile(filename, modelName string, meshDecimationRatios []float64) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read URDF file")
	}

	// Build mesh map by loading mesh files from disk
	urdfDir := filepath.Dir(filename)
	meshMap, err := buildMeshMapFromURDF(xmlData, urdfDir, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to build mesh map")
	}

	mc, err := UnmarshalModelXML(xmlData, modelName, meshMap, meshDecimationRatios)
	if err != nil {
		return nil, err
	}

	return mc.ParseConfig(modelName)
}

// resolveURDFMeshPath turns a mesh reference into the relative path the model keeps and the place
// on disk to read it from. A package:// reference loses its scheme and package name, and is read
// under packageRoot when one is given, since the URDF is then not at its package root.
func resolveURDFMeshPath(original, urdfDir, packageRoot string) (meshPath, absolutePath string) {
	meshPath = normalizeURDFMeshPath(original)
	switch {
	case filepath.IsAbs(meshPath):
		return meshPath, meshPath
	case packageRoot != "" && strings.HasPrefix(original, urdfPackagePrefix):
		return meshPath, filepath.Join(packageRoot, meshPath)
	default:
		return meshPath, filepath.Join(urdfDir, meshPath)
	}
}

// ReadURDFModel reads a URDF for conversion to a kinematic model file. Beyond what
// ParseModelXMLFile does, it resolves package:// mesh references under packageRoot when the file
// does not sit at its package root, packageRoot may be empty, and attaches each link's visual
// geometries to the model with their bytes, in whatever format the file names, so the converted
// model carries them. A visual nothing of ours can show, a DAE mesh or a cylinder, is left out and
// described in the second result so the caller can say so.
func ReadURDFModel(filename, packageRoot string) (Model, []string, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read URDF file")
	}
	urdfDir := filepath.Dir(filename)
	meshMap, err := buildMeshMapFromURDF(xmlData, urdfDir, packageRoot)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to build mesh map")
	}
	mc, err := UnmarshalModelXML(xmlData, "", meshMap, nil)
	if err != nil {
		return nil, nil, err
	}
	model, err := mc.ParseConfig("")
	if err != nil {
		return nil, nil, err
	}
	sm, ok := model.(*SimpleModel)
	if !ok {
		return nil, nil, fmt.Errorf("URDF %s did not parse into a simple model", filename)
	}
	visuals, skipped, err := urdfVisuals(xmlData, urdfDir, packageRoot)
	if err != nil {
		return nil, nil, err
	}
	for link, geometries := range visuals {
		if err := sm.SetVisualGeometries(link, geometries); err != nil {
			return nil, nil, errors.Wrapf(err, "visuals of link %q", link)
		}
	}
	return sm, skipped, nil
}

// urdfVisuals reads every link's visual elements with their mesh bytes, keyed by link name, and
// describes the ones it had to leave out.
func urdfVisuals(xmlData []byte, urdfDir, packageRoot string) (map[string][]spatialmath.GeometryConfig, []string, error) {
	urdf := &ModelConfigURDF{}
	if err := xml.Unmarshal(xmlData, urdf); err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse URDF for visual extraction")
	}
	visuals := map[string][]spatialmath.GeometryConfig{}
	var skipped []string
	for _, link := range urdf.Links {
		for i, visual := range link.Visual {
			gc, reason, err := visual.toVisualConfig(urdfDir, packageRoot)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "link %q visual %d", link.Name, i)
			}
			if reason != "" {
				skipped = append(skipped, fmt.Sprintf("link %q visual %d: %s", link.Name, i, reason))
				continue
			}
			visuals[link.Name] = append(visuals[link.Name], *gc)
		}
	}
	return visuals, skipped, nil
}

// toVisualConfig reads a visual element as a geometry config with any mesh bytes inline. A mesh
// in a format no viewer of ours loads, or a shape the message cannot carry, comes back with the
// reason instead of a config, since one odd visual should not fail the whole file.
func (v *visual) toVisualConfig(urdfDir, packageRoot string) (*spatialmath.GeometryConfig, string, error) {
	origin := spatialmath.NewZeroPose()
	if v.Origin != nil {
		origin = v.Origin.Parse()
	}
	switch {
	case v.Geometry.Mesh != nil:
		meshPath, absolutePath := resolveURDFMeshPath(v.Geometry.Mesh.Filename, urdfDir, packageRoot)
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(meshPath)), ".")
		if ext == "dae" {
			return nil, fmt.Sprintf("%s is a DAE mesh, which nothing reads", v.Geometry.Mesh.Filename), nil
		}
		//nolint:gosec
		data, err := os.ReadFile(absolutePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load mesh file %s (referenced as %s): %w",
				absolutePath, v.Geometry.Mesh.Filename, err)
		}
		oc, err := spatialmath.NewOrientationConfig(origin.Orientation())
		if err != nil {
			return nil, "", err
		}
		return &spatialmath.GeometryConfig{
			Type:              spatialmath.MeshType,
			MeshData:          data,
			MeshContentType:   ext,
			MeshFilePath:      filepath.ToSlash(meshPath),
			TranslationOffset: origin.Point(),
			OrientationOffset: *oc,
		}, "", nil
	case v.Geometry.Cylinder != nil:
		return nil, "a cylinder has no shape in the kinematic model", nil
	case v.Geometry.Box != nil, v.Geometry.Sphere != nil:
		g, err := (&collision{Origin: v.Origin, Geometry: v.Geometry}).toGeometry(nil, 1)
		if err != nil {
			return nil, "", err
		}
		gc, err := spatialmath.NewGeometryConfig(g)
		if err != nil {
			return nil, "", err
		}
		return gc, "", nil
	default:
		return nil, "no geometry", nil
	}
}
