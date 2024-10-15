package arm

import (
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
)

func updateKinematicHelper(name string, resp *commonpb.GetKinematicsResponse) (referenceframe.Model, error) {
	format := resp.GetFormat()
	data := resp.GetKinematicsData()

	switch format {
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_SVA:
		return referenceframe.UnmarshalModelJSON(data, name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_URDF:
		modelconf, err := urdf.UnmarshalModelXML(data, name)
		if err != nil {
			return nil, err
		}
		return modelconf.ParseConfig(name)
	case commonpb.KinematicsFileFormat_KINEMATICS_FILE_FORMAT_UNSPECIFIED:
		fallthrough
	default:
		if formatName, ok := commonpb.KinematicsFileFormat_name[int32(format)]; ok {
			return nil, fmt.Errorf("unable to parse file of type %s", formatName)
		}
		return nil, fmt.Errorf("unable to parse unknown file type %d", format)
	}
}
