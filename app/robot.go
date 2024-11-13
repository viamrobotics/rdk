package app

import (
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Robot holds the information of a machine.
type Robot struct {
	ID         string
	Name       string
	Location   string
	LastAccess *timestamppb.Timestamp
	CreatedOn  *timestamppb.Timestamp
}

func robotFromProto(robot *pb.Robot) *Robot {
	return &Robot{
		ID:         robot.Id,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn:  robot.CreatedOn,
	}
}

// RoverRentalRobot holds the information of a rover rental robot.
type RoverRentalRobot struct {
	RobotID         string
	LocationID      string
	RobotName       string
	RobotMainPartID string
}

func roverRentalRobotFromProto(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotID:         rrRobot.RobotId,
		LocationID:      rrRobot.LocationId,
		RobotName:       rrRobot.RobotName,
		RobotMainPartID: rrRobot.RobotMainPartId,
	}
}

// RobotPart is a specific machine part.
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

func robotPartFromProto(robotPart *pb.RobotPart) (*RobotPart, error) {
	var secrets []*SharedSecret
	for _, secret := range robotPart.Secrets {
		s, err := sharedSecretFromProto(secret)
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

// RobotPartHistoryEntry is a history entry of a robot part.
type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *timestamppb.Timestamp
	Old      *RobotPart
	EditedBy *AuthenticatorInfo
}

func robotPartHistoryEntryFromProto(entry *pb.RobotPartHistoryEntry) (*RobotPartHistoryEntry, error) {
	old, err := robotPartFromProto(entry.Old)
	if err != nil {
		return nil, err
	}
	info, err := authenticatorInfoFromProto(entry.EditedBy)
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
