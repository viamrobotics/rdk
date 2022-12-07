package referenceframe

import (
	"encoding/xml"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

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
	XMLName   xml.Name `xml:"link"`
	Name      string   `xml:"name,attr"`
	Collision []struct {
		XMLName xml.Name `xml:"collision"`
		Name    string   `xml:"name,attr"`
		Origin  struct {
			XMLName xml.Name `xml:"origin"`
			RPY     string   `xml:"rpy,attr"` // Fixed frame angle "r p y" format, in radians
			XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
		} `xml:"origin"`
		Geometry struct {
			XMLName xml.Name `xml:"geometry"`
			Box     struct {
				XMLName xml.Name `xml:"box"`
				Size    string   `xml:"size,attr"` // "x y z" format, in meters
			} `xml:"box"`
			Sphere struct {
				XMLName xml.Name `xml:"sphere"`
				Radius  float64  `xml:"radius,attr"` // in meters
			} `xml:"sphere"`
		} `xml:"geometry"`
	} `xml:"collision"`
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
		childLink := JSONLink{ID: jointElem.Child.Link, Parent: jointElem.Name}

		switch jointElem.Type {
		case ContinuousJoint, RevoluteJoint, PrismaticJoint:
			// Parse important details about each joint, including axes and limits
			jointAxes := convStringAttrToFloats(jointElem.Axis.XYZ)
			thisJoint := JSONJoint{
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
				thisJoint.Min, thisJoint.Max = metersToMM(jointElem.Limit.Lower), metersToMM(jointElem.Limit.Upper)
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
			childLink.Translation = spatial.TranslationConfig{
				metersToMM(childXYZ[0]),
				metersToMM(childXYZ[1]),
				metersToMM(childXYZ[2]),
			}
			childLink.Orientation = *childOrient

			if err != nil {
				return nil, err
			}
		case FixedJoint:
			// Handle fixed joint -> static link conversion instead of adding to Joints[]
			thisLink := JSONLink{ID: jointElem.Name, Parent: jointElem.Parent.Link}

			linkXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
			linkRPY := convStringAttrToFloats(jointElem.Origin.RPY)
			linkEA := spatial.EulerAngles{Roll: linkRPY[0], Pitch: linkRPY[1], Yaw: linkRPY[2]}
			linkOrient, err := spatial.NewOrientationConfig(linkEA.AxisAngles())

			// Note the conversion from meters to mm
			thisLink.Translation = spatial.TranslationConfig{
				metersToMM(linkXYZ[0]),
				metersToMM(linkXYZ[1]),
				metersToMM(linkXYZ[2]),
			}
			thisLink.Orientation = *linkOrient

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
				geoCfg, err := createConfigFromCollision(linkElem)
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
			thisLink := JSONLink{ID: linkElem.Name, Parent: World}
			thisLink.Translation = spatial.TranslationConfig{0.0, 0.0, 0.0}
			thisLink.Orientation = spatial.OrientationConfig{} // Orientation is guaranteed to be zero for this

			if hasCollision {
				geoCfg, err := createConfigFromCollision(linkElem)
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

// Convenience method to split up space-delimited fields in URDFs, such as xyz or rpy attributes.
func convStringAttrToFloats(attr string) []float64 {
	var converted []float64
	attrSlice := strings.Fields(attr)

	for _, value := range attrSlice {
		value, err := strconv.ParseFloat(value, 64)
		if err != nil {
			value = math.NaN()
		}

		converted = append(converted, value)
	}

	return converted
}

// Convenience method to simplify creating geometry configs from URDF XML that has a collision element specified.
func createConfigFromCollision(link URDFLink) (spatial.GeometryConfig, error) {
	var geoCfg spatial.GeometryConfig
	boxGeometry := link.Collision[0].Geometry.Box
	sphereGeometry := link.Collision[0].Geometry.Sphere

	// Offset for the geometry origin from the reference link origin
	geomXYZ := convStringAttrToFloats(link.Collision[0].Origin.XYZ)
	geomTx := spatial.TranslationConfig{geomXYZ[0], geomXYZ[1], geomXYZ[2]}
	geomRPY := convStringAttrToFloats(link.Collision[0].Origin.RPY)
	geomEA := spatial.EulerAngles{
		Roll:  utils.RadToDeg(geomRPY[0]),
		Pitch: utils.RadToDeg(geomRPY[1]),
		Yaw:   utils.RadToDeg(geomRPY[2]),
	}
	geomOx, err := spatial.NewOrientationConfig(geomEA.AxisAngles())
	if err != nil {
		return spatial.GeometryConfig{}, err
	}

	// Logic specific to the geometry type
	switch {
	case len(boxGeometry.Size) > 0:
		boxDims := convStringAttrToFloats(boxGeometry.Size)
		geoCfg = spatial.GeometryConfig{
			Type:              "box",
			X:                 metersToMM(boxDims[0]),
			Y:                 metersToMM(boxDims[1]),
			Z:                 metersToMM(boxDims[2]),
			TranslationOffset: geomTx,
			OrientationOffset: *geomOx,
			Label:             "box",
		}
	case sphereGeometry.Radius > 0:
		sphereRadius := metersToMM(sphereGeometry.Radius)
		geoCfg = spatial.GeometryConfig{
			Type:              "sphere",
			R:                 sphereRadius,
			TranslationOffset: geomTx,
			OrientationOffset: *geomOx,
			Label:             "sphere",
		}
	default:
		return spatial.GeometryConfig{}, errors.Errorf("Unsupported collision geometry type detected for [ %v ] link", link.Collision[0].Name)
	}

	return geoCfg, nil
}

// Convenience function to change engineering unit scale for the given input.
func metersToMM(valMeters float64) float64 {
	return valMeters * 1000
}
