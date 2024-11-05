package arm

import (
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/referenceframe/urdf"
	"go.viam.com/rdk/utils"
)

// MoveOptions define parameters to be obeyed during arm movement.
type MoveOptions struct {
	MaxVelRads, MaxAccRads float64
}

func moveOptionsFromProtobuf(protobuf *pb.MoveOptions) *MoveOptions {
	if protobuf == nil {
		protobuf = &pb.MoveOptions{}
	}

	var vel, acc float64
	if protobuf.MaxVelDegsPerSec != nil {
		vel = *protobuf.MaxVelDegsPerSec
	}
	if protobuf.MaxAccDegsPerSec2 != nil {
		acc = *protobuf.MaxAccDegsPerSec2
	}
	return &MoveOptions{
		MaxVelRads: utils.DegToRad(vel),
		MaxAccRads: utils.DegToRad(acc),
	}
}

func (opts MoveOptions) toProtobuf() *pb.MoveOptions {
	vel := utils.RadToDeg(opts.MaxVelRads)
	acc := utils.RadToDeg(opts.MaxAccRads)
	return &pb.MoveOptions{
		MaxVelDegsPerSec:  &vel,
		MaxAccDegsPerSec2: &acc,
	}
}

func parseKinematicsResponse(name string, resp *commonpb.GetKinematicsResponse) (referenceframe.Model, error) {
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
