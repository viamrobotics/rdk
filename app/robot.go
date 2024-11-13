package app

import (
	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Robot struct {
	ID         string
	Name       string
	Location   string
	LastAccess *timestamppb.Timestamp
	CreatedOn  *timestamppb.Timestamp
}

func ProtoToRobot(robot *pb.Robot) *Robot {
	return &Robot{
		ID:         robot.Id,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn:  robot.CreatedOn,
	}
}

func RobotToProto(robot *Robot) *pb.Robot {
	return &pb.Robot{
		Id:         robot.ID,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn:  robot.CreatedOn,
	}
}

type RoverRentalRobot struct {
	RobotID         string
	LocationID      string
	RobotName       string
	RobotMainPartID string
}

func ProtoToRoverRentalRobot(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotID:         rrRobot.RobotId,
		LocationID:      rrRobot.LocationId,
		RobotName:       rrRobot.RobotName,
		RobotMainPartID: rrRobot.RobotMainPartId,
	}
}

func RoverRentalRobotToProto(rrRobot *RoverRentalRobot) *pb.RoverRentalRobot {
	return &pb.RoverRentalRobot{
		RobotId:         rrRobot.RobotID,
		LocationId:      rrRobot.LocationID,
		RobotName:       rrRobot.RobotName,
		RobotMainPartId: rrRobot.RobotMainPartID,
	}
}

type RobotPart struct {
	ID               string
	Name             string
	DNSName          string
	Secret           string
	Robot            string
	LocationID       string
	RobotConfig      *map[string]interface{}
	LastAccess       *timestamppb.Timestamp
	UserSuppliedInfo *map[string]interface{}
	MainPart         bool
	Fqdn             string
	LocalFqdn        string
	CreatedOn        *timestamppb.Timestamp
	Secrets          []*SharedSecret
	LastUpdated      *timestamppb.Timestamp
}

func ProtoToRobotPart(robotPart *pb.RobotPart) (*RobotPart, error) {
	var secrets []*SharedSecret
	for _, secret := range robotPart.Secrets {
		s, err := ProtoToSharedSecret(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	cfg := robotPart.RobotConfig.AsMap()
	info := robotPart.UserSuppliedInfo.AsMap()
	return &RobotPart{
		ID:               robotPart.Id,
		Name:             robotPart.Name,
		DNSName:          robotPart.DnsName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.DnsName,
		LocationID:       robotPart.LocationId,
		RobotConfig:      &cfg,
		LastAccess:       robotPart.LastAccess,
		UserSuppliedInfo: &info,
		MainPart:         robotPart.MainPart,
		Fqdn:             robotPart.Fqdn,
		LocalFqdn:        robotPart.LocalFqdn,
		CreatedOn:        robotPart.CreatedOn,
		Secrets:          secrets,
		LastUpdated:      robotPart.LastUpdated,
	}, nil
}

func RobotPartToProto(robotPart *RobotPart) (*pb.RobotPart, error) {
	var secrets []*pb.SharedSecret
	for _, secret := range robotPart.Secrets {
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
		Id:               robotPart.ID,
		Name:             robotPart.Name,
		DnsName:          robotPart.DNSName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.DNSName,
		LocationId:       robotPart.LocationID,
		RobotConfig:      robotConfig,
		LastAccess:       robotPart.LastAccess,
		UserSuppliedInfo: userSuppliedInfo,
		MainPart:         robotPart.MainPart,
		Fqdn:             robotPart.Fqdn,
		LocalFqdn:        robotPart.LocalFqdn,
		CreatedOn:        robotPart.CreatedOn,
		Secrets:          secrets,
		LastUpdated:      robotPart.LastUpdated,
	}, nil
}

type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *timestamppb.Timestamp
	Old      *RobotPart
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
		Part:     entry.Part,
		Robot:    entry.Robot,
		When:     entry.When,
		Old:      old,
		EditedBy: info,
	}, nil
}
