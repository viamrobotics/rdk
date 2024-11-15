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
	FQDN             string
	LocalFQDN        string
	CreatedOn        *timestamppb.Timestamp
	Secrets          []*SharedSecret
	LastUpdated      *timestamppb.Timestamp
}

func robotPartFromProto(robotPart *pb.RobotPart) *RobotPart {
	var secrets []*SharedSecret
	for _, secret := range robotPart.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	cfg := robotPart.RobotConfig.AsMap()
	info := robotPart.UserSuppliedInfo.AsMap()
	return &RobotPart{
		ID:               robotPart.Id,
		Name:             robotPart.Name,
		DNSName:          robotPart.DnsName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.Robot,
		LocationID:       robotPart.LocationId,
		RobotConfig:      &cfg,
		LastAccess:       robotPart.LastAccess,
		UserSuppliedInfo: &info,
		MainPart:         robotPart.MainPart,
		FQDN:             robotPart.Fqdn,
		LocalFQDN:        robotPart.LocalFqdn,
		CreatedOn:        robotPart.CreatedOn,
		Secrets:          secrets,
		LastUpdated:      robotPart.LastUpdated,
	}
}

// RobotPartHistoryEntry is a history entry of a robot part.
type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *timestamppb.Timestamp
	Old      *RobotPart
	EditedBy *AuthenticatorInfo
}

func robotPartHistoryEntryFromProto(entry *pb.RobotPartHistoryEntry) *RobotPartHistoryEntry {
	return &RobotPartHistoryEntry{
		Part:     entry.Part,
		Robot:    entry.Robot,
		When:     entry.When,
		Old:      robotPartFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}
