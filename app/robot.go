package app

import (
	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Robot struct {
	Id string
	Name string
	Location string
	LastAccess *timestamppb.Timestamp
	CreatedOn *timestamppb.Timestamp
}

func ProtoToRobot(robot *pb.Robot) *Robot {
	return &Robot{
		Id: robot.Id,
		Name: robot.Name,
		Location: robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn: robot.CreatedOn,
	}
}

func RobotToProto(robot *Robot) *pb.Robot {
	return &pb.Robot{
		Id: robot.Id,
		Name: robot.Name,
		Location: robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn: robot.CreatedOn,
	}
}

type RoverRentalRobot struct {
	RobotId string
	LocationId string
	RobotName string
	RobotMainPartId string
}

func ProtoToRoverRentalRobot(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotId: rrRobot.RobotId,
		LocationId: rrRobot.LocationId,
		RobotName: rrRobot.RobotName,
		RobotMainPartId: rrRobot.RobotMainPartId,
	}
}

func RoverRentalRobotToProto(rrRobot *RoverRentalRobot) *pb.RoverRentalRobot {
	return &pb.RoverRentalRobot{
		RobotId: rrRobot.RobotId,
		LocationId: rrRobot.LocationId,
		RobotName: rrRobot.RobotName,
		RobotMainPartId: rrRobot.RobotMainPartId,
	}
}

type RobotPart struct {
	Id string
	Name string
	DnsName string
	Secret string
	Robot string
	LocationId string
	RobotConfig *map[string]interface{}
	LastAccess *timestamppb.Timestamp
	UserSuppliedInfo *map[string]interface{}
	MainPart bool
	Fqdn string
	LocalFqdn string
	CreatedOn *timestamppb.Timestamp
	Secrets []*SharedSecret
	LastUpdated *timestamppb.Timestamp
}

func ProtoToRobotPart(robotPart *pb.RobotPart) (*RobotPart, error) {
	var secrets []*SharedSecret
	for _, secret := range(robotPart.Secrets) {
		s, err := ProtoToSharedSecret(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	cfg := robotPart.RobotConfig.AsMap()
	info := robotPart.UserSuppliedInfo.AsMap()
	return &RobotPart{
		Id: robotPart.Id,
		Name: robotPart.Name,
		DnsName: robotPart.DnsName,
		Secret: robotPart.Secret,
		Robot: robotPart.DnsName,
		LocationId: robotPart.LocationId,
		RobotConfig: &cfg,
		LastAccess: robotPart.LastAccess,
		UserSuppliedInfo: &info,
		MainPart: robotPart.MainPart,
		Fqdn: robotPart.Fqdn,
		LocalFqdn: robotPart.LocalFqdn,
		CreatedOn: robotPart.CreatedOn,
		Secrets: secrets,
		LastUpdated: robotPart.LastUpdated,
	}, nil
}

func RobotPartToProto(robotPart *RobotPart) (*pb.RobotPart, error) {
	var secrets []*pb.SharedSecret
	for _, secret := range(robotPart.Secrets) {
		s, err := SharedSecretToProto(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	robotConfig, err := protoutils.StructToStructPb(robotPart.RobotConfig)
	if err != nil {
		return nil, err
	}
	userSuppliedInfo, err := protoutils.StructToStructPb(robotPart.UserSuppliedInfo)
	if err != nil {
		return nil, err
	}
	return &pb.RobotPart{
		Id: robotPart.Id,
		Name: robotPart.Name,
		DnsName: robotPart.DnsName,
		Secret: robotPart.Secret,
		Robot: robotPart.DnsName,
		LocationId: robotPart.LocationId,
		RobotConfig: robotConfig,
		LastAccess: robotPart.LastAccess,
		UserSuppliedInfo: userSuppliedInfo,
		MainPart: robotPart.MainPart,
		Fqdn: robotPart.Fqdn,
		LocalFqdn: robotPart.LocalFqdn,
		CreatedOn: robotPart.CreatedOn,
		Secrets: secrets,
		LastUpdated: robotPart.LastUpdated,
	}, nil
}

type RobotPartHistoryEntry struct {
	Part string
	Robot string
	When *timestamppb.Timestamp
	Old *RobotPart
	EditedBy *AuthenticatorInfo
}

func ProtoToRobotPartHistoryEntry(entry *pb.RobotPartHistoryEntry) (*RobotPartHistoryEntry, error) {
	old, err := ProtoToRobotPart(entry.Old)
	if err != nil {
		return nil, err
	}
	info, err := ProtoToAuthenticatorInfo(entry.EditedBy)
	if err != nil {
		return nil, err
	}
	return &RobotPartHistoryEntry{
		Part: entry.Part,
		Robot: entry.Robot,
		When: entry.When,
		Old: old,
		EditedBy: info,
	}, nil
}
