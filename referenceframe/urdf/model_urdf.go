// Package urdf provides functions which enable *.urdf files to be used within RDK
package urdf

import (
	"encoding/xml"
	"math"
	"os"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Extension is the file extension associated with URDF files.
const Extension string = "urdf"

// ModelConfig represents all supported fields in a Universal Robot Description Format (URDF) file.
type ModelConfig struct {
	XMLName xml.Name `xml:"robot"`
	Name    string   `xml:"name,attr"`
	Links   []link   `xml:"link"`
	Joints  []joint  `xml:"joint"`
}

// link is a struct which details the XML used in a URDF link element.
type link struct {
	XMLName   xml.Name    `xml:"link"`
	Name      string      `xml:"name,attr"`
	Collision []collision `xml:"collision"`
}

// joint is a struct which details the XML used in a URDF joint element.
type joint struct {
	XMLName xml.Name `xml:"joint"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"type,attr"`
	Parent  frame    `xml:"parent"`
	Child   frame    `xml:"child"`
	Origin  *pose    `xml:"origin,omitempty"`
	Axis    *axis    `xml:"axis,omitempty"`
	Limit   *limit   `xml:"limit,omitempty"`
}

// NewModelFromWorldState creates a urdf.Config struct which can be marshalled into xml and will be a
// valid .urdf file representing the geometries in the given worldstate.
func NewModelFromWorldState(ws *referenceframe.WorldState, name string) (*ModelConfig, error) {
	// the link we initialize this list with represents the world frame
	links := []link{{Name: referenceframe.World}}
	joints := make([]joint, 0)
	emptyFS := referenceframe.NewEmptyFrameSystem("")
	gf, err := ws.ObstaclesInWorldFrame(emptyFS, referenceframe.NewZeroInputs(emptyFS))
	if err != nil {
		return nil, err
	}
	for _, g := range gf.Geometries() {
		coll, err := newCollision(g)
		if err != nil {
			return nil, err
		}
		links = append(links, link{
			Name:      g.Label(),
			Collision: []collision{*coll},
		})
		joints = append(joints, joint{
			Name:   g.Label() + "_joint",
			Type:   "fixed",
			Parent: frame{gf.Parent()},
			Child:  frame{g.Label()},
		})
	}
	return &ModelConfig{
		Name:   name,
		Links:  links,
		Joints: joints,
	}, nil
}

// UnmarshalModelXML will transfer the given URDF XML data into an equivalent ModelConfig. Direct unmarshaling in the
// same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accommodate differences
// between the two kinematics encoding schemes.
func UnmarshalModelXML(xmlData []byte, modelName string) (*referenceframe.ModelConfig, error) {
	// Unmarshal into a URDF ModelConfig
	urdf := &ModelConfig{}
	err := xml.Unmarshal(xmlData, urdf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert URDF data to equivalent URDFConfig struct")
	}

	// Use default name if none is provided
	if modelName == "" {
		modelName = urdf.Name
	}

	// Read all links first
	links := make(map[string]*referenceframe.LinkConfig, 0)
	for _, linkElem := range urdf.Links {
		// Skip any world links
		if linkElem.Name == referenceframe.World {
			continue
		}

		link := &referenceframe.LinkConfig{ID: linkElem.Name}
		if len(linkElem.Collision) > 0 {
			geometry, err := linkElem.Collision[0].toGeometry()
			if err != nil {
				return nil, err
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
	joints := make([]referenceframe.JointConfig, 0)
	for _, jointElem := range urdf.Joints {
		switch jointElem.Type {
		case referenceframe.ContinuousJoint, referenceframe.RevoluteJoint, referenceframe.PrismaticJoint:
			// Parse important details about each joint, including axes and limits
			thisJoint := referenceframe.JointConfig{
				ID:     jointElem.Name,
				Type:   jointElem.Type,
				Parent: jointElem.Parent.Link,
			}
			if jointElem.Axis != nil {
				thisJoint.Axis = jointElem.Axis.Parse()
			}

			// Slightly different limits handling for continuous, revolute, and prismatic joints
			switch jointElem.Type {
			case referenceframe.ContinuousJoint:
				thisJoint.Type = referenceframe.RevoluteJoint // Currently, we treate a continuous joint as a special case of a revolute joint
				thisJoint.Min, thisJoint.Max = math.Inf(-1), math.Inf(1)
			case referenceframe.PrismaticJoint:
				thisJoint.Min, thisJoint.Max = utils.MetersToMM(jointElem.Limit.Lower), utils.MetersToMM(jointElem.Limit.Upper)
			case referenceframe.RevoluteJoint:
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
				return nil, referenceframe.NewFrameNotInListOfTransformsError(jointElem.Parent.Link)
			}
			parentLink.Translation = r3.Vector{
				X: utils.MetersToMM(childXYZ[0]),
				Y: utils.MetersToMM(childXYZ[1]),
				Z: utils.MetersToMM(childXYZ[2]),
			}
			parentLink.Orientation = childOrient

		case referenceframe.FixedJoint:
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

			link := &referenceframe.LinkConfig{
				ID:          jointElem.Name,
				Translation: r3.Vector{X: utils.MetersToMM(linkXYZ[0]), Y: utils.MetersToMM(linkXYZ[1]), Z: utils.MetersToMM(linkXYZ[2])},
				Orientation: linkOrient,
				Parent:      jointElem.Parent.Link,
			}
			links[jointElem.Name] = link
		default:
			return nil, referenceframe.NewUnsupportedJointTypeError(jointElem.Type)
		}

		// Point the child link to this joint
		childLink, ok := links[jointElem.Child.Link]
		if !ok {
			return nil, referenceframe.NewFrameNotInListOfTransformsError(jointElem.Child.Link)
		}
		childLink.Parent = jointElem.Name
	}

	// Return as a referenceframe.ModelConfig
	linkSlice := make([]referenceframe.LinkConfig, 0, len(links))
	for _, link := range links {
		linkSlice = append(linkSlice, *link)
	}
	return &referenceframe.ModelConfig{
		Name:         modelName,
		KinParamType: "SVA",
		Links:        linkSlice,
		Joints:       joints,
		OriginalFile: &referenceframe.ModelFile{
			Bytes:     xmlData,
			Extension: Extension,
		},
	}, nil
}

// ParseModelXMLFile will read a given file and parse the contained URDF XML data into an equivalent Model.
func ParseModelXMLFile(filename, modelName string) (referenceframe.Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read URDF file")
	}

	mc, err := UnmarshalModelXML(xmlData, modelName)
	if err != nil {
		return nil, err
	}

	return mc.ParseConfig(modelName)
}
