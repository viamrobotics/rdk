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
)

// URDFConfig represents all supported fields in a Universal Robot Description Format (URDF) file.
type URDFConfig struct {
	XMLName xml.Name `xml:"robot"`
	Name    string   `xml:"name,attr"`
	Links   []struct{
		XMLName   xml.Name `xml:"link"`
		Name      string   `xml:"name,attr"`
		Collision []struct{
			XMLName xml.Name `xml:"collision"`
			Name    string   `xml:"name,attr"`
			Origin  struct{
				XMLName xml.Name `xml:"origin"`
				RPY     string   `xml:"rpy,attr"`  // Fixed frame angle "r p y" format, in radians
				XYZ     string   `xml:"xyz,attr"`  // "x y z" format, in meters
			} `xml:"origin"`
			Geometry struct{
				XMLName xml.Name `xml:"geometry"`
				Box     struct{
					XMLName xml.Name `xml:"box"`
					Size    string   `xml:"size,attr"`  // "x y z" format, in meters
				} `xml:"box"`
				Sphere struct{
					XMLName xml.Name `xml:"sphere"`
					Radius  float64   `xml:"radius,attr"`  // in meters
				} `xml:"sphere"`
			} `xml:"geometry"`
		} `xml:"collision"`
	} `xml:"link"`
	Joints            []struct{
		XMLName xml.Name `xml:"joint"`
		Name    string   `xml:"name,attr"`
		Type    string   `xml:"type,attr"`
		Origin  struct{
			XMLName xml.Name `xml:"origin"`
			RPY     string   `xml:"rpy,attr"`  // Fixed frame angle "r p y" format, in radians
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format, in meters
		} `xml:"origin"`
		Parent struct{
			XMLName xml.Name `xml:"parent"`
			Link    string   `xml:"link,attr"`
		} `xml:"parent"`
		Child struct{
			XMLName xml.Name `xml:"child"`
			Link    string   `xml:"link,attr"`
		} `xml:"child"`
		Axis struct{
			XMLName xml.Name `xml:"axis"`
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format, in meters
		} `xml:"axis"`
		Limit struct{
			XMLName xml.Name `xml:"limit"`
			Lower   float64  `xml:"lower,attr"`  // translation limits are in meters, revolute limits are in radians
			Upper   float64  `xml:"upper,attr"`  // translation limits are in meters, revolute limits are in radians
		} `xml:"limit"`
	} `xml:"joint"`
}

// ParseURDFFile will read a given file and parse the contained URDF XML data.
func ParseURDFFile(filename, modelName string) (Model, error) {
	//nolint:gosec
	xmlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read URDF file")
	}
	return ConvertURDFToConfig(xmlData, modelName)
}

// ConvertURDFToConfig will transfer the given URDF XML data into an equivalent Config. Direct conversion to a model in
// the same fashion as ModelJSON is not possible, as URDF data will need to be evaluated to accomodate differences
// between the two kinematics encoding schemes.
func ConvertURDFToConfig(xmlData []byte, modelName string) (Model, error) {
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

		// TODO(wspies): Consider this joint more carefully
		if strings.Contains(jointName, World) {
			firstLink = jointElem.Child.Link
			transforms[firstLink] = NewZeroStaticFrame(firstLink)
			continue
		}

		// Parse important details about each joint, including axes, limits, and relationships
		jointAxes := convStringAttrToFloats(jointElem.Axis.XYZ)
		jointLimits := Limit{Min: jointElem.Limit.Lower, Max: jointElem.Limit.Upper}
		childMap[jointElem.Parent.Link] = jointName  // This will get used to find the terminal link(s) quickly
		parentMap[jointElem.Child.Link] = jointName
		parentMap[jointName] = jointElem.Parent.Link

		// Generate transform data for the given joint type
		switch jointElem.Type {
		case "continuous", "revolute":
			if jointElem.Type == "continuous" {
				jointLimits.Min, jointLimits.Max = -math.Pi, math.Pi  // TODO(wspies): PR comment bait for limit constants
			}
			transforms[jointName], err = NewRotationalFrame(jointName,
				spatial.R4AA{RX: jointAxes[0], RY: jointAxes[1], RZ: jointAxes[2]}, jointLimits)
		case "prismatic":
			transforms[jointName], err = NewTranslationalFrame(jointName,
				r3.Vector{X: jointAxes[0]*1000, Y: jointAxes[1]*1000, Z: jointAxes[2]*1000}, jointLimits)
		case "fixed":
			transforms[jointName] = NewZeroStaticFrame(jointName)
		default:
			return nil, NewUnsupportedJointTypeError(jointElem.Type)
		}

		// Create static link frames from the joint transformation data, we can replace those later if we find that a link
		// has geometry data
		linkXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
		linkRPY := convStringAttrToFloats(jointElem.Origin.RPY)
		linkPose := spatial.NewPoseFromOrientation(r3.Vector{X: linkXYZ[0]*1000, Y: linkXYZ[1]*1000, Z: linkXYZ[2]*1000},
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

		// TODO(wspies): Consider this link more carefully
		if strings.Contains(linkName, World) {
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
				r3.Vector{X: offsetXYZ[0]*1000, Y: offsetXYZ[1]*1000, Z: offsetXYZ[2]*1000},
				&spatial.EulerAngles{Roll: offsetRPY[0], Pitch: offsetRPY[1], Yaw: offsetRPY[2]})

			// Select the geometry creator for the appropriate geometry element
			// Note that dimensions are converted from meters to millimeters
			var geometryCreator spatial.GeometryCreator
			if len(boxGeometry.Size) > 0 {
				boxDims := convStringAttrToFloats(linkElem.Collision[0].Geometry.Box.Size)
				boxSize := r3.Vector{X: boxDims[0]*1000, Y: boxDims[1]*1000, Z: boxDims[2]*1000}
				geometryCreator, err = spatial.NewBoxCreator(boxSize, offsetPose, linkElem.Collision[0].Name)
				transforms[linkName], err = NewStaticFrameWithGeometry(linkName, refLinkPose, geometryCreator)
			} else if sphereGeometry.Radius > 0 {
				sphereRadius := linkElem.Collision[0].Geometry.Sphere.Radius*1000
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

// Convenience method to split up space-delimited fields in URDFs, such as xyz or rpy attributes
func convStringAttrToFloats(attr string) []float64 {
	var converted []float64
	attr_slice := strings.Fields(attr)

	for _, value := range attr_slice {
		value, _ := strconv.ParseFloat(value, 64)
		converted = append(converted, value)
	}

	return converted
}
