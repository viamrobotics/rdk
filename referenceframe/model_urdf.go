package referenceframe

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

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
		coll, err := newCollision(g)
		if err != nil {
			return nil, err
		}
		links = append(links, linkXML{
			Name:      g.Label(),
			Collision: []collision{*coll},
		})
		joints = append(joints, jointXML{
			Name:   g.Label() + "_joint",
			Type:   "fixed",
			Parent: frame{gf.Parent()},
			Child:  frame{g.Label()},
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
func UnmarshalModelXML(xmlData []byte, modelName string) (*ModelConfigJSON, error) {
	return unmarshalModelXMLWithBasePath(xmlData, modelName, "")
}

func unmarshalModelXMLWithBasePath(xmlData []byte, modelName, basePath string) (*ModelConfigJSON, error) {
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
			geometry, err := linkElem.Collision[0].toGeometryWithBasePath(basePath)
			if err != nil {
				return nil, fmt.Errorf("failed to convert collision geometry %v to geometry config: %w", linkElem.Name, err)
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
	modelConfig := &ModelConfigJSON{
		Name:         modelName,
		KinParamType: "SVA",
		Links:        linkSlice,
		Joints:       joints,
	}

	// Marshal to JSON to preserve embedded mesh data when sent over RPC
	jsonData, err := json.Marshal(modelConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal model config to JSON")
	}

	modelConfig.OriginalFile = &ModelFile{
		Bytes:     jsonData,
		Extension: "json",
	}

	return modelConfig, nil
}

// ParseModelXMLFile will read a given file and parse the contained URDF XML data into an equivalent Model.
func ParseModelXMLFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read URDF file")
	}

	// Extract the base directory for resolving relative mesh paths
	basePath := filepath.Dir(filename)

	mc, err := unmarshalModelXMLWithBasePath(xmlData, modelName, basePath)
	if err != nil {
		return nil, err
	}
	// if it sees that I have meshes, then load them into bytes

	return mc.ParseConfig(modelName)
}
