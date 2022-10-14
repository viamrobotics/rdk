package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	datapb "go.viam.com/api/app/data/v1"
	rdkcli "go.viam.com/rdk/cli"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Flags
	dataFlagDestination    = "destination"
	dataFlagType           = "type"
	dataFlagOrgs           = "orgs"
	dataFlagLocation       = "location"
	dataFlagRobotID        = "robot_id"
	dataFlagPartID         = "part_id"
	dataFlagRobotName      = "robot_name"
	dataFlagPartName       = "part_name"
	dataFlagComponentType  = "component_type"
	dataFlagComponentModel = "component_model"
	dataFlagComponentName  = "component_name"
	dataFlagMethod         = "method"
	dataFlagMimeTypes      = "mime_types"
	dataFlagStart          = "start"
	dataFlagEnd            = "end"
)

func DataCommand(c *cli.Context) error {
	if c.String(dataFlagType) != "binary" && c.String(dataFlagType) != "tabular" {
		return errors.Errorf("type must be binary or tabular, got %s", c.String("type"))
	}

	filter := &datapb.Filter{}
	if c.StringSlice(dataFlagOrgs) != nil {
		filter.OrgIds = c.StringSlice(dataFlagOrgs)
	}
	if c.String(dataFlagLocation) != "" {
		filter.LocationId = c.String(dataFlagLocation)
	}
	if c.String(dataFlagRobotID) != "" {
		filter.RobotId = c.String(dataFlagRobotID)
	}
	if c.String(dataFlagPartID) != "" {
		filter.PartId = c.String(dataFlagPartID)
	}
	if c.String(dataFlagRobotName) != "" {
		filter.RobotName = c.String(dataFlagRobotName)
	}
	if c.String(dataFlagPartName) != "" {
		filter.PartName = c.String(dataFlagPartName)
	}
	if c.String(dataFlagComponentType) != "" {
		filter.ComponentType = c.String(dataFlagComponentType)
	}
	if c.String(dataFlagComponentModel) != "" {
		filter.ComponentModel = c.String(dataFlagComponentModel)
	}
	if c.String(dataFlagComponentName) != "" {
		filter.ComponentName = c.String(dataFlagComponentName)
	}
	if c.String(dataFlagMethod) != "" {
		filter.Method = c.String(dataFlagMethod)
	}
	if len(c.StringSlice(dataFlagMimeTypes)) != 0 {
		filter.MimeType = c.StringSlice(dataFlagMimeTypes)
	}

	var start *timestamppb.Timestamp
	var end *timestamppb.Timestamp
	if c.Timestamp(dataFlagStart) != nil {
		start = timestamppb.New(*c.Timestamp(dataFlagStart))
	}
	if c.Timestamp(dataFlagEnd) != nil {
		end = timestamppb.New(*c.Timestamp(dataFlagEnd))
	}
	if start != nil || end != nil {
		filter.Interval = &datapb.CaptureInterval{
			Start: start,
			End:   end,
		}
	}

	fmt.Println("Building app client")
	client, err := rdkcli.NewAppClient(c)
	if err != nil {
		return err
	}

	dataType := c.String(dataFlagType)
	switch dataType {
	case dataFlagType:
		if err := client.BinaryData(c.String(dataFlagDestination), filter); err != nil {
			return err
		}
	case "tabular":
		if err := client.TabularData(c.String("destination"), filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("invalid data type %s", dataType)
	}

	return nil
}
