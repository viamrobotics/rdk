package referenceframe

import (
	"encoding/xml"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// URDFConfig represents all supported fields in a Universal Robot Description Format (URDF) file.
type URDFConfig struct {
	XMLName xml.Name    `xml:"robot"`
	Name    string      `xml:"name,attr"`
	Links   []UrdfLink  `xml:"link"`
	Joints  []UrdfJoint `xml:"joint"`
}

type UrdfLink struct {
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

type UrdfJoint struct {
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

// ParseURDFFileAsConfig will read a given file and parse the contained URDF XML data as a ModelConfig struct.
func ParseURDFFileAsConfig(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read URDF file")
	}
	return ConvertURDFToConfig(xmlData, modelName)
}

// ParseURDFFile will read a given file and parse the contained URDF XML data.
func ParseURDFFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read URDF file")
	}
	return ConvertURDFToModel(xmlData, modelName)
}

// ConvertURDFToConfig will transfer the given URDF XML data into an equivalent ModelConfig. Direct unmarshaling in the
// same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accommodate differences
// between the two kinematics encoding schemes.
func ConvertURDFToConfig(xmlData []byte, modelName string) (Model, error) {
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
			return nil, errors.Errorf("Joints with the name 'world' are not supported by config parsers")
		}

		// Relationship tracking
		parentMap[jointElem.Name] = jointElem.Parent.Link
		parentMap[jointElem.Child.Link] = jointElem.Name

		// Set up the child link mentioned in this joint; fill out the details in the link parsing section later
		childLink := JsonLink{ID: jointElem.Child.Link, Parent: jointElem.Name}

		switch jointElem.Type {
		case "continuous", "revolute", "prismatic":
			// Parse important details about each joint, including axes and limits
			jointAxes := convStringAttrToFloats(jointElem.Axis.XYZ)
			thisJoint := JsonJoint{
				ID:     jointElem.Name,
				Type:   jointElem.Type,
				Parent: jointElem.Parent.Link,
				Axis:   spatial.AxisConfig{jointAxes[0], jointAxes[1], jointAxes[2]},
			}

			// Slightly different limits handling for continuous, revolute, and prismatic joints
			if jointElem.Type == "continuous" {
				thisJoint.Type = "revolute" // Currently, we treate a continuous joint as a special case of the revolute joint
				thisJoint.Min, thisJoint.Max = -math.Pi, math.Pi
			} else if jointElem.Type == "prismatic" {
				thisJoint.Min, thisJoint.Max = jointElem.Limit.Lower*1000, jointElem.Limit.Upper*1000
			} else {
				thisJoint.Min, thisJoint.Max = jointElem.Limit.Lower, jointElem.Limit.Upper
			}

			mc.Joints = append(mc.Joints, thisJoint)

			// Generate child link translation and orientation data, which is held by this joint per the URDF design
			childXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
			childRPY := convStringAttrToFloats(jointElem.Origin.RPY)
			childEA := spatial.EulerAngles{Roll: utils.RadToDeg(childRPY[0]), Pitch: utils.RadToDeg(childRPY[1]), Yaw: utils.RadToDeg(childRPY[2])}
			childOrient, err := spatial.NewOrientationConfig(childEA.AxisAngles())

			childLink.Translation = spatial.TranslationConfig{childXYZ[0] * 1000, childXYZ[1] * 1000, childXYZ[2] * 1000}
			childLink.Orientation = *childOrient

			if err != nil {
				return nil, err
			}
		case "fixed":
			// Handle fixed joint -> static link conversion instead of adding to Joints[]
			thisLink := JsonLink{ID: jointElem.Name, Parent: jointElem.Parent.Link}

			linkXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
			linkRPY := convStringAttrToFloats(jointElem.Origin.RPY)
			linkEA := spatial.EulerAngles{Roll: utils.RadToDeg(linkRPY[0]), Pitch: utils.RadToDeg(linkRPY[1]), Yaw: utils.RadToDeg(linkRPY[2])}
			linkOrient, err := spatial.NewOrientationConfig(linkEA.AxisAngles())

			thisLink.Translation = spatial.TranslationConfig{linkXYZ[0] * 1000, linkXYZ[1] * 1000, linkXYZ[2] * 1000}
			thisLink.Orientation = *linkOrient

			if err != nil {
				return nil, err
			}

			mc.Links = append(mc.Links, thisLink)
		default:
			return nil, NewUnsupportedJointTypeError(jointElem.Type)
		}

		if err != nil {
			return nil, err
		}

		mc.Links = append(mc.Links, childLink)
	}

	// Handle links
	for _, linkElem := range urdf.Links {
		// Skip any world links
		if linkElem.Name == "world" {
			continue
		}

		// Find matching links which already exist, take care of geometry if collision elements are detected
		hasCollision := false
		if len(linkElem.Collision) > 0 {
			hasCollision = true
		}

		for idx, prefabLink := range mc.Links {
			if prefabLink.ID == linkElem.Name && hasCollision {
				geoCfg, _ := convURDFCollisionToConfig(linkElem)
				mc.Links[idx].Geometry = geoCfg
				break
			}
		}

		// In the event the link does not already exist in the ModelConfig, we will have to generate it now
		if _, ok := parentMap[linkElem.Name]; !ok {
			thisLink := JsonLink{ID: linkElem.Name, Parent: World}

			linkEA := spatial.EulerAngles{Roll: 0.0, Pitch: 0.0, Yaw: 0.0}
			linkOrient, err := spatial.NewOrientationConfig(linkEA.AxisAngles())

			thisLink.Translation = spatial.TranslationConfig{0.0, 0.0, 0.0}
			thisLink.Orientation = *linkOrient

			if err != nil {
				return nil, err
			}

			if hasCollision {
				geoCfg, _ := convURDFCollisionToConfig(linkElem)
				thisLink.Geometry = geoCfg
			}

			mc.Links = append(mc.Links, thisLink)
		}
	}

	return mc.ParseConfig(modelName)
}

// ConvertURDFToModel will transfer the given URDF XML data into an equivalent Model. Direct conversion to a model in
// the same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accommodate differences
// between the two kinematics encoding schemes.
func ConvertURDFToModel(xmlData []byte, modelName string) (Model, error) {
	// empty data probably means that the read URDF has no actionable information
	if len(xmlData) == 0 {
		return nil, ErrNoModelInformation
	}

	urdf := &URDFConfig{}
	err := xml.Unmarshal(xmlData, urdf)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert URDF data to equivalent URDFConfig struct")
	}

	// Code below this point could be split off into another function, similarly to ParseConfig
	if modelName == "" {
		modelName = urdf.Name
	}

	model := NewSimpleModel(modelName)
	transforms := map[string]Frame{}

	// Handle joints
	var firstLink string
	childMap := map[string]string{}
	parentMap := map[string]string{}

	for _, jointElem := range urdf.Joints {
		jointName := jointElem.Name

		// Special case in the event that a "world" link is present in the URDF
		if jointElem.Parent.Link == World {
			firstLink = jointElem.Child.Link
		}

		// Parse important details about each joint, including axes and limits
		jointAxes := convStringAttrToFloats(jointElem.Axis.XYZ)
		jointLimits := Limit{Min: jointElem.Limit.Lower, Max: jointElem.Limit.Upper}
		childMap[jointElem.Parent.Link] = jointName // This will get used to find the terminal link(s) quickly
		parentMap[jointElem.Child.Link] = jointName
		parentMap[jointName] = jointElem.Parent.Link

		// Generate transform data for the given joint type
		switch jointElem.Type {
		case "continuous", "revolute":
			if jointElem.Type == "continuous" {
				jointLimits.Min, jointLimits.Max = -math.Pi, math.Pi
			}
			transforms[jointName], err = NewRotationalFrame(jointName,
				spatial.R4AA{RX: jointAxes[0], RY: jointAxes[1], RZ: jointAxes[2]}, jointLimits)
		case "prismatic":
			transforms[jointName], err = NewTranslationalFrame(jointName,
				r3.Vector{X: jointAxes[0] * 1000, Y: jointAxes[1] * 1000, Z: jointAxes[2] * 1000}, jointLimits)
		case "fixed":
			transforms[jointName] = NewZeroStaticFrame(jointName)
		default:
			return nil, NewUnsupportedJointTypeError(jointElem.Type)
		}

		// Create static link frames from the joint transformation data, we can replace those later if we find that a link
		// has geometry data
		linkXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
		linkRPY := convStringAttrToFloats(jointElem.Origin.RPY)
		linkPose := spatial.NewPoseFromOrientation(r3.Vector{X: linkXYZ[0] * 1000, Y: linkXYZ[1] * 1000, Z: linkXYZ[2] * 1000},
			&spatial.EulerAngles{Roll: linkRPY[0], Pitch: linkRPY[1], Yaw: linkRPY[2]})
		transforms[jointElem.Child.Link], err = NewStaticFrame(jointElem.Child.Link, linkPose)

		if err != nil {
			return nil, err
		}
	}

	// Handle links
	terminalLinks := []string{}

	for _, linkElem := range urdf.Links {
		linkName := linkElem.Name
		var refLinkPose spatial.Pose

		// World links are ignored (that is a reserved name) when parsing links.
		if linkName == World {
			continue
		}

		// In a majority of cases, the end effector link will not have any joint listed as a child.
		if _, ok := childMap[linkName]; !ok {
			terminalLinks = append(terminalLinks, linkName)
		}

		// If any collision elements are found, generate geometry for that object with the given frame
		// TODO(wspies): Add functionality to handle multiple collision objects
		if len(linkElem.Collision) > 0 {
			refLinkPose, err = transforms[linkName].Transform([]Input{})
			boxGeometry := linkElem.Collision[0].Geometry.Box
			sphereGeometry := linkElem.Collision[0].Geometry.Sphere

			// Offset for the geometry origin from the reference link origin
			offsetXYZ := convStringAttrToFloats(linkElem.Collision[0].Origin.XYZ)
			offsetRPY := convStringAttrToFloats(linkElem.Collision[0].Origin.RPY)
			offsetPose := spatial.NewPoseFromOrientation(
				r3.Vector{X: offsetXYZ[0] * 1000, Y: offsetXYZ[1] * 1000, Z: offsetXYZ[2] * 1000},
				&spatial.EulerAngles{Roll: offsetRPY[0], Pitch: offsetRPY[1], Yaw: offsetRPY[2]})

			// Select the geometry creator for the appropriate geometry element
			// Note that dimensions are converted from meters to millimeters
			var geometryCreator spatial.GeometryCreator
			if len(boxGeometry.Size) > 0 {
				boxDims := convStringAttrToFloats(linkElem.Collision[0].Geometry.Box.Size)
				boxSize := r3.Vector{X: boxDims[0] * 1000, Y: boxDims[1] * 1000, Z: boxDims[2] * 1000}
				geometryCreator, err = spatial.NewBoxCreator(boxSize, offsetPose, linkElem.Collision[0].Name)
				transforms[linkName], err = NewStaticFrameWithGeometry(linkName, refLinkPose, geometryCreator)
			} else if sphereGeometry.Radius > 0 {
				sphereRadius := linkElem.Collision[0].Geometry.Sphere.Radius * 1000
				geometryCreator, err = spatial.NewSphereCreator(sphereRadius, offsetPose, linkElem.Collision[0].Name)
				transforms[linkName], err = NewStaticFrameWithGeometry(linkName, refLinkPose, geometryCreator)
			} else {
				err = errors.Errorf("Unsupported collision geometry type detected for [ %v ] link", linkElem.Collision[0].Name)
			}
		}

		if err != nil {
			return nil, err
		}
	}

	if len(terminalLinks) != 1 {
		return nil, errors.Errorf("Invalid terminal link count: %d", len(terminalLinks))
	}

	// Create joint and link ordering, starting with the end effector link and going backwards
	orderedTransforms, err := sortTransforms(transforms, parentMap, terminalLinks[0], firstLink)
	if err != nil {
		return nil, err
	}

	model.OrdTransforms = orderedTransforms
	return model, nil
}

// Convenience method to split up space-delimited fields in URDFs, such as xyz or rpy attributes.
func convStringAttrToFloats(attr string) []float64 {
	var converted []float64
	attr_slice := strings.Fields(attr)

	for _, value := range attr_slice {
		value, _ := strconv.ParseFloat(value, 64)
		converted = append(converted, value)
	}

	return converted
}

func convURDFCollisionToConfig(link UrdfLink) (spatial.GeometryConfig, error) {
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

	// Logic specific to the geometry type
	if len(boxGeometry.Size) > 0 {
		boxDims := convStringAttrToFloats(boxGeometry.Size)
		geoCfg = spatial.GeometryConfig{
			Type:              "box",
			X:                 boxDims[0] * 1000, // from meters to mm
			Y:                 boxDims[1] * 1000, // from meters to mm
			Z:                 boxDims[2] * 1000, // from meters to mm
			TranslationOffset: geomTx,
			OrientationOffset: *geomOx,
			Label:             "box",
		}
	} else if sphereGeometry.Radius > 0 {
		sphereRadius := sphereGeometry.Radius * 1000 // from meters to mm
		geoCfg = spatial.GeometryConfig{
			Type:              "sphere",
			R:                 sphereRadius,
			TranslationOffset: geomTx,
			OrientationOffset: *geomOx,
			Label:             "sphere",
		}
	} else {
		err = errors.Errorf("Unsupported collision geometry type detected for [ %v ] link", link.Collision[0].Name)
	}

	if err != nil {
		return spatial.GeometryConfig{}, err
	}

	return geoCfg, nil
}
