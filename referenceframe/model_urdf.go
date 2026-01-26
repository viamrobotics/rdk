package referenceframe

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
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
}

// jointXML is a struct which details the XML used in a URDF jointXML element.
type jointXML struct {
	XMLName xml.Name `xml:"joint"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"type,attr"`
	Parent  frame    `xml:"parent"`
	Child   frame    `xml:"child"`
	Origin  *pose    `xml:"origin,omitempty"`
	Axis    *axis    `xml:"axis,omitempty"`
	Limit   *limit   `xml:"limit,omitempty"`
}

// NewModelFromWorldState creates a ModelConfigURDF struct which can be marshalled into xml and will be a
// valid .urdf file representing the geometries in the given worldstate.
func NewModelFromWorldState(ws *WorldState, name string) (*ModelConfigURDF, error) {
	// the link we initialize this list with represents the world frame
	links := []linkXML{{Name: World}}
	joints := make([]jointXML, 0)
	emptyFS := NewEmptyFrameSystem("")
	gf, err := ws.ObstaclesInWorldFrame(emptyFS, NewZeroInputs(emptyFS))
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
func UnmarshalModelXML(xmlData []byte, modelName string, meshMap map[string]*commonpb.Mesh) (*ModelConfigJSON, error) {
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
				geometry, err = linkElem.Collision[0].toGeometry(meshMap)
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
func buildMeshMapFromURDF(xmlData []byte, urdfDir string) (map[string]*commonpb.Mesh, error) {
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
			meshPath := normalizeURDFMeshPath(originalPath)

			// Check if we've already loaded this mesh
			if _, exists := meshMap[meshPath]; exists {
				continue
			}

			// Resolve path relative to URDF directory
			var absolutePath string
			if filepath.IsAbs(meshPath) {
				absolutePath = meshPath
			} else {
				absolutePath = filepath.Join(urdfDir, meshPath)
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
func ParseModelXMLFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read URDF file")
	}

	// Build mesh map by loading mesh files from disk
	urdfDir := filepath.Dir(filename)
	meshMap, err := buildMeshMapFromURDF(xmlData, urdfDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build mesh map")
	}

	mc, err := UnmarshalModelXML(xmlData, modelName, meshMap)
	if err != nil {
		return nil, err
	}

	return mc.ParseConfig(modelName)
}
