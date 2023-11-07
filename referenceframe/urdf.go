package referenceframe

import (
	"encoding/xml"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/spatialmath"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// URDFConfig represents all supported fields in a Universal Robot Description Format (URDF) file.
type URDFConfig struct {
	XMLName xml.Name    `xml:"robot"`
	Name    string      `xml:"name,attr"`
	Links   []URDFLink  `xml:"link"`
	Joints  []URDFJoint `xml:"joint"`
}

// URDFLink is a struct which details the XML used in a URDF link element.
type URDFLink struct {
	XMLName   xml.Name                       `xml:"link"`
	Name      string                         `xml:"name,attr"`
	Collision []spatialmath.URDFCollisionXML `xml:"collision"`
}

// URDFJoint is a struct which details the XML used in a URDF joint element.
type URDFJoint struct {
	XMLName xml.Name `xml:"joint"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"type,attr"`
	Origin  struct {
		XMLName xml.Name `xml:"origin"`
		RPY     string   `xml:"rpy,attr"` // Fixed frame angle "r p y" format, in radians
		XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
	} `xml:"origin"`
	Parent struct {
		XMLName xml.Name `xml:"parent"`
		Link    string   `xml:"link,attr"`
	} `xml:"parent"`
	Child struct {
		XMLName xml.Name `xml:"child"`
		Link    string   `xml:"link,attr"`
	} `xml:"child"`
	Axis struct {
		XMLName xml.Name `xml:"axis"`
		XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
	} `xml:"axis"`
	Limit struct {
		XMLName xml.Name `xml:"limit"`
		Lower   float64  `xml:"lower,attr"` // translation limits are in meters, revolute limits are in radians
		Upper   float64  `xml:"upper,attr"` // translation limits are in meters, revolute limits are in radians
	} `xml:"limit"`
}

// ParseURDFFile will read a given file and parse the contained URDF XML data into an equivalent ModelConfig struct.
func ParseURDFFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read URDF file")
	}

	mc, err := ConvertURDFToConfig(xmlData, modelName)
	if err != nil {
		return nil, err
	}

	return mc.ParseConfig(modelName)
}

// ConvertURDFToConfig will transfer the given URDF XML data into an equivalent ModelConfig. Direct unmarshaling in the
// same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accommodate differences
// between the two kinematics encoding schemes.
func ConvertURDFToConfig(xmlData []byte, modelName string) (*ModelConfig, error) {
	// empty data probably means that the read URDF has no actionable information
	if len(xmlData) == 0 {
		return nil, ErrNoModelInformation
	}

	mc := &ModelConfig{}
	urdf := &URDFConfig{}
	err := xml.Unmarshal(xmlData, urdf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert URDF data to equivalent URDFConfig struct")
	}

	if modelName == "" {
		modelName = urdf.Name
	}

	parentMap := map[string]string{}

	// Migrate URDF elements into an equivalent ModelConfig representation
	mc.Name = modelName
	mc.KinParamType = "SVA"

	// Handle joints
	for _, jointElem := range urdf.Joints {
		// Checking for reserved names in this or adjacent elements
		if jointElem.Name == World {
			return nil, errors.New("Joints with the name 'world' are not supported by config parsers")
		}

		// Relationship tracking
		parentMap[jointElem.Name] = jointElem.Parent.Link
		parentMap[jointElem.Child.Link] = jointElem.Name

		// Set up the child link mentioned in this joint; fill out the details in the link parsing section later

		childLink := LinkConfig{ID: jointElem.Child.Link, Parent: jointElem.Name}

		switch jointElem.Type {
		case ContinuousJoint, RevoluteJoint, PrismaticJoint:
			// Parse important details about each joint, including axes and limits
			jointAxes := convStringAttrToFloats(jointElem.Axis.XYZ)
			thisJoint := JointConfig{
				ID:     jointElem.Name,
				Type:   jointElem.Type,
				Parent: jointElem.Parent.Link,
				Axis:   spatial.AxisConfig{jointAxes[0], jointAxes[1], jointAxes[2]},
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

			mc.Joints = append(mc.Joints, thisJoint)

			// Generate child link translation and orientation data, which is held by this joint per the URDF design
			childXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
			childRPY := convStringAttrToFloats(jointElem.Origin.RPY)
			childEA := spatial.EulerAngles{Roll: childRPY[0], Pitch: childRPY[1], Yaw: childRPY[2]}
			childOrient, err := spatial.NewOrientationConfig(childEA.AxisAngles())

			// Note the conversion from meters to mm
			childLink.Translation = r3.Vector{utils.MetersToMM(childXYZ[0]), utils.MetersToMM(childXYZ[1]), utils.MetersToMM(childXYZ[2])}
			childLink.Orientation = childOrient

			if err != nil {
				return nil, err
			}
		case FixedJoint:
			// Handle fixed joint -> static link conversion instead of adding to Joints[]
			thisLink := LinkConfig{ID: jointElem.Name, Parent: jointElem.Parent.Link}

			linkXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
			linkRPY := convStringAttrToFloats(jointElem.Origin.RPY)
			linkEA := spatial.EulerAngles{Roll: linkRPY[0], Pitch: linkRPY[1], Yaw: linkRPY[2]}
			linkOrient, err := spatial.NewOrientationConfig(linkEA.AxisAngles())

			// Note the conversion from meters to mm
			thisLink.Translation = r3.Vector{utils.MetersToMM(linkXYZ[0]), utils.MetersToMM(linkXYZ[1]), utils.MetersToMM(linkXYZ[2])}
			thisLink.Orientation = linkOrient

			if err != nil {
				return nil, err
			}

			mc.Links = append(mc.Links, thisLink)
		default:
			return nil, NewUnsupportedJointTypeError(jointElem.Type)
		}

		mc.Links = append(mc.Links, childLink)
	}

	// Handle links
	for _, linkElem := range urdf.Links {
		// Skip any world links
		if linkElem.Name == World {
			continue
		}

		// Find matching links which already exist, take care of geometry if collision elements are detected
		hasCollision := len(linkElem.Collision) > 0
		for idx, prefabLink := range mc.Links {
			if prefabLink.ID == linkElem.Name && hasCollision {
				geoCfg, err := linkElem.Collision[0].ToConfig()
				if err != nil {
					return nil, err
				}
				mc.Links[idx].Geometry = geoCfg
				break
			}
		}

		// In the event the link does not already exist in the ModelConfig, we will have to generate it now
		// Most likely, this is a link normally whose parent is the World
		if _, ok := parentMap[linkElem.Name]; !ok {
			thisLink := LinkConfig{ID: linkElem.Name, Parent: World}
			thisLink.Translation = r3.Vector{0.0, 0.0, 0.0}
			thisLink.Orientation = &spatial.OrientationConfig{} // Orientation is guaranteed to be zero for this

			if hasCollision {
				geoCfg, err := linkElem.Collision[0].ToConfig()
				if err != nil {
					return nil, err
				}
				thisLink.Geometry = geoCfg
			}

			mc.Links = append(mc.Links, thisLink)
		}
	}
	return mc, nil
}
