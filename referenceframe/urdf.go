package referenceframe

import (
	"encoding/xml"
	"fmt"
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
				RPY     string   `xml:"rpy,attr"`  // "r p y" format
				XYZ     string   `xml:"xyz,attr"`  // "x y z" format
			} `xml:"origin"`
			Geometry struct{
				XMLName xml.Name `xml:"geometry"`
				Box     struct{
					XMLName xml.Name `xml:"box"`
					Size    string   `xml:"size,attr"`  // "x y z" format
				} `xml:"box"`
				Sphere struct{
					XMLName xml.Name `xml:"sphere"`
					Radius  float64   `xml:"radius,attr"`
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
			RPY     string   `xml:"rpy,attr"`  // "r p y" format
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format
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
			XYZ     string   `xml:"xyz,attr"`  // "x y z" format
		} `xml:"axis"`
		Limit struct{
			XMLName xml.Name `xml:"limit"`
			Lower   float64  `xml:"lower,attr"`
			Upper   float64  `xml:"upper,attr"`
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

	zeroPosition := r3.Vector{X: 0.0, Y: 0.0, Z: 0.0}
	zeroOrientation := spatial.NewZeroOrientation()
	zeroPose := spatial.NewPoseFromOrientation(zeroPosition, zeroOrientation)

	// Handle joints
	var firstLink string
	childMap := map[string]string{}
	parentMap := map[string]string{}
	jointOrigins := map[string]spatial.Pose{}

	for _, jointElem := range urdf.Joints {
		jointName := jointElem.Name

		// TODO(wspies): Consider this joint more carefully
		if strings.Contains(jointName, "world") {
			firstLink = jointElem.Child.Link
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
				r3.Vector{X: jointAxes[0], Y: jointAxes[1], Z: jointAxes[2]}, jointLimits)
		case "fixed":
			transforms[jointName], err = NewStaticFrame(jointName, zeroPose)
		default:
			return nil, errors.Errorf("Unsupported joint type detected: %v", jointElem.Type)
		}

		// Preserve transform data for a given joint so the parent link can create a static frame more easily later, since
		// URDFs manage frame origins within joints instead of within links
		geoXYZ := convStringAttrToFloats(jointElem.Origin.XYZ)
		geoRPY := convStringAttrToFloats(jointElem.Origin.RPY)
		jointPose := spatial.NewPoseFromOrientation(r3.Vector{X: geoXYZ[0], Y: geoXYZ[1], Z: geoXYZ[2]},
			&spatial.EulerAngles{Roll: geoRPY[0], Pitch: geoRPY[1], Yaw: geoRPY[2]})
		jointOrigins[jointName] = jointPose
	}

	// Handle links
	terminalLinks := []string{}

	for _, linkElem := range urdf.Links {
		linkName := linkElem.Name

		// TODO(wspies): Consider this link more carefully
		if strings.Contains(linkName, "world") {
			continue
		}

		// In a majority of cases, the end effector link will not have any joint listed as a child, nor will it have any
		// associated spatial transforms.
		// TODO(wspies): There should be only one of these terminal links, but some robots may have additional terminal
		// links, so as to accomodate actuators, sensors, or other devices. Might not need to deal with that now...
		if _, is_ok := childMap[linkName]; !is_ok {
			terminalLinks = append(terminalLinks, linkName)
			transforms[linkName], err = NewStaticFrame(linkName, zeroPose)
			continue
		}

		// Otherwise, parse important details about each link, overriding where necessary
		// The child joint origin element will have translation and rotation details for each link
		linkPose := jointOrigins[childMap[linkName]]

		if len(linkElem.Collision) > 0 {
			// TODO(wspies): Finish up handling collision geometry
			// Create the geometry for the collision object
			//geoXYZ := convStringAttrToFloats(linkElem.Collision[0].Origin.XYZ)
			//geoRPY := convStringAttrToFloats(linkElem.Collision[0].Origin.RPY)
			//linkPosition = r3.Vector{X: geoXYZ[0], Y: geoXYZ[1], Z: geoXYZ[2]}
			//linkOrientation = &spatial.EulerAngles{Roll: geoRPY[0], Pitch: geoRPY[1], Yaw: geoRPY[2]}
			//linkPose = spatial.NewPoseFromOrientation(linkPosition, linkOrientation)

			// TODO(wspies): Decide what to do about box versus sphere
			//var geometryCreator spatial.GeometryCreator
			//if box {
			//	geometryCreator = spatial.NewBoxCreator()
			//	transforms[linkName], err = NewStaticFrameWithGeometry(linkName, linkPose, geometryCreator)
			//} else if sphere {
			//	geometryCreator = spatial.NewSphereCreator()
			//	transforms[linkName], err = NewStaticFrameWithGeometry(linkName, linkPose, geometryCreator)
			//} else {
			//	err = errors.Errorf("Unsupported collision geometry type detected for [ %v ] link", linkElem.Collision[0].Name)
			//}
			fmt.Println("Working with collision object...")  // DEBUG(wspies): Remove after testing
		} else {
			transforms[linkName], err = NewStaticFrame(linkName, linkPose)
		}
	}

	if len(terminalLinks) != 1 {
		return nil, errors.Errorf("Invalid terminal link count: %d", len(terminalLinks))
	}

	// Create joint and link ordering, starting with the end effector link and going backwards
	seen := map[string]bool{}
	nextTransform := transforms[terminalLinks[0]]
	orderedTransforms := []Frame{nextTransform}
	seen[nextTransform.Name()] = true

	for {
		parent := parentMap[nextTransform.Name()]

		if seen[parent] {
			return nil, errors.New("Multiple links detected with same parent")
		}

		// We have reached the most proximal "base" link, no further logic required
		if parent == firstLink {
			break
		}
		seen[parent] = true

		// Otherwise, add this transform to the ordered list
		nextTransform = transforms[parent]
		orderedTransforms = append(orderedTransforms, nextTransform)
	}

	// As with ModelJSON, reverse the orderedTransform list so that the base link is first
	for i, j := 0, len(orderedTransforms)-1; i < j; i, j = i+1, j-1 {
		orderedTransforms[i], orderedTransforms[j] = orderedTransforms[j], orderedTransforms[i]
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
