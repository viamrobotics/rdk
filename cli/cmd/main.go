// Package main is the CLI command itself.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	rdkcli "go.viam.com/rdk/cli"
)

const (
	// Flags.
	dataFlagDestination       = "destination"
	dataFlagDataType          = "data_type"
	dataFlagOrgIDs            = "org_ids"
	dataFlagLocationIDs       = "location_ids"
	dataFlagRobotID           = "robot_id"
	dataFlagPartID            = "part_id"
	dataFlagRobotName         = "robot_name"
	dataFlagPartName          = "part_name"
	dataFlagComponentType     = "component_type"
	dataFlagComponentModel    = "component_model"
	dataFlagComponentName     = "component_name"
	dataFlagMethod            = "method"
	dataFlagMimeTypes         = "mime_types"
	dataFlagStart             = "start"
	dataFlagEnd               = "end"
	dataFlagParallelDownloads = "parallel"

	dataTypeBinary  = "binary"
	dataTypeTabular = "tabular"
)

func main() {
	var logger golog.Logger

	app := &cli.App{
		Name:  "viam",
		Usage: "interact with your robots",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:   "base-url",
				Hidden: true,
				Value:  "https://app.viam.com:443",
				Usage:  "base URL of app",
			},
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `FILE`",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"vvv"},
				Usage:   "enable debug logging",
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("debug") {
				logger = golog.NewDevelopmentLogger("cli")
			} else {
				logger = zap.NewNop().Sugar()
			}

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "auth",
				Usage: "authenticate to app.viam.com",
				Action: func(c *cli.Context) error {
					client, err := rdkcli.NewAppClient(c)
					if err != nil {
						return err
					}
					if email := client.Config().AuthEmail; email != "" {
						fmt.Fprintf(c.App.Writer, "Already authenticated as %q\n", email)
						return nil
					}

					token, authURL, err := client.PrepareAuthorization()
					if err != nil {
						return err
					}
					fmt.Fprintf(c.App.Writer, "To authorize this device, visit:\n\t%s\n", authURL)

					ctx, cancel := context.WithTimeout(c.Context, time.Minute)
					defer cancel()

					if err := client.Authenticate(ctx, token); err != nil {
						return err
					}
					fmt.Fprintf(c.App.Writer, "Authenticated as %q\n", client.Config().AuthEmail)
					return nil
				},
			},
			{
				Name:  "logout",
				Usage: "logout from current session",
				Action: func(c *cli.Context) error {
					client, err := rdkcli.NewAppClient(c)
					if err != nil {
						return err
					}
					email := client.Config().AuthEmail
					if email == "" {
						fmt.Fprintf(c.App.Writer, "Already logged out\n")
						return nil
					}
					if err := client.Logout(); err != nil {
						return err
					}
					fmt.Fprintf(c.App.Writer, "Logged out from %q\n", email)
					return nil
				},
			},
			{
				Name:  "whoami",
				Usage: "get currently authenticated user",
				Action: func(c *cli.Context) error {
					client, err := rdkcli.NewAppClient(c)
					if err != nil {
						return err
					}
					email := client.Config().AuthEmail
					if email == "" {
						fmt.Fprintf(c.App.Writer, "Not logged in\n")
						return nil
					}
					fmt.Fprintf(c.App.Writer, "%s\n", email)
					return nil
				},
			},
			{
				Name:  "organizations",
				Usage: "work with organizations",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list organizations",
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}
							orgs, err := client.ListOrganizations()
							if err != nil {
								return err
							}
							for _, org := range orgs {
								fmt.Fprintf(c.App.Writer, "%s (id: %s)\n", org.Name, org.Id)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "locations",
				Usage: "work with locations",
				Subcommands: []*cli.Command{
					{
						Name:      "list",
						Usage:     "list locations",
						ArgsUsage: "[organization]",
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}
							orgStr := c.Args().First()
							listLocations := func(orgID string) error {
								locs, err := client.ListLocations(orgID)
								if err != nil {
									return err
								}
								for _, loc := range locs {
									fmt.Fprintf(c.App.Writer, "%s (id: %s)\n", loc.Name, loc.Id)
								}
								return nil
							}
							if orgStr == "" {
								orgs, err := client.ListOrganizations()
								if err != nil {
									return err
								}
								for i, org := range orgs {
									if i != 0 {
										fmt.Fprintln(c.App.Writer, "")
									}

									fmt.Fprintf(c.App.Writer, "%s:\n", org.Name)
									if err := listLocations(org.Id); err != nil {
										return err
									}
								}
								return nil
							}
							return listLocations(orgStr)
						},
					},
				},
			},
			{
				Name:  "data",
				Usage: "work with data",
				Subcommands: []*cli.Command{
					{
						Name:  "export",
						Usage: "download data from Viam cloud",
						UsageText: fmt.Sprintf("viam data export <%s> <%s> [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s]",
							dataFlagDestination, dataFlagDataType, dataFlagOrgIDs, dataFlagLocationIDs, dataFlagRobotID, dataFlagRobotName,
							dataFlagPartID, dataFlagPartName, dataFlagComponentType, dataFlagComponentModel, dataFlagComponentName,
							dataFlagStart, dataFlagEnd, dataFlagMethod, dataFlagMimeTypes, dataFlagParallelDownloads),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     dataFlagDestination,
								Required: true,
								Usage:    "output directory for downloaded data",
							},
							&cli.StringFlag{
								Name:     dataFlagDataType,
								Required: true,
								Usage:    "data type to be downloaded: either binary or tabular",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagOrgIDs,
								Required: false,
								Usage:    "orgs filter",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagLocationIDs,
								Required: false,
								Usage:    "locations filter",
							},
							&cli.StringFlag{
								Name:     dataFlagRobotID,
								Required: false,
								Usage:    "robot_id filter",
							},
							&cli.StringFlag{
								Name:     dataFlagPartID,
								Required: false,
								Usage:    "part_id filter",
							},
							&cli.StringFlag{
								Name:     dataFlagRobotName,
								Required: false,
								Usage:    "robot_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagPartName,
								Required: false,
								Usage:    "part_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentType,
								Required: false,
								Usage:    "component_type filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentModel,
								Required: false,
								Usage:    "component_model filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentName,
								Required: false,
								Usage:    "component_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagMethod,
								Required: false,
								Usage:    "method filter",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagMimeTypes,
								Required: false,
								Usage:    "mime_types filter",
							},
							&cli.UintFlag{
								Name:     dataFlagParallelDownloads,
								Required: false,
								Usage:    "number of download requests to make in parallel, with a default value of 10",
							},
							&cli.StringFlag{
								Name:     dataFlagStart,
								Required: false,
								Usage:    "ISO-8601 timestamp indicating the start of the interval filter",
							},
							&cli.StringFlag{
								Name:     dataFlagEnd,
								Required: false,
								Usage:    "ISO-8601 timestamp indicating the end of the interval filter",
							},
						},
						Action: DataCommand,
					},
					{
						Name:  "delete",
						Usage: "delete data from Viam cloud",
						UsageText: fmt.Sprintf("viam data delete [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s] [%s]",
							dataFlagDataType, dataFlagOrgIDs, dataFlagLocationIDs, dataFlagRobotID, dataFlagRobotName,
							dataFlagPartID, dataFlagPartName, dataFlagComponentType, dataFlagComponentModel, dataFlagComponentName,
							dataFlagStart, dataFlagEnd, dataFlagMethod, dataFlagMimeTypes),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     dataFlagDataType,
								Required: false,
								Usage:    "data type to be deleted: either binary or tabular",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagOrgIDs,
								Required: false,
								Usage:    "orgs filter",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagLocationIDs,
								Required: false,
								Usage:    "locations filter",
							},
							&cli.StringFlag{
								Name:     dataFlagRobotID,
								Required: false,
								Usage:    "robot_id filter",
							},
							&cli.StringFlag{
								Name:     dataFlagPartID,
								Required: false,
								Usage:    "part_id filter",
							},
							&cli.StringFlag{
								Name:     dataFlagRobotName,
								Required: false,
								Usage:    "robot_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagPartName,
								Required: false,
								Usage:    "part_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentType,
								Required: false,
								Usage:    "component_type filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentModel,
								Required: false,
								Usage:    "component_model filter",
							},
							&cli.StringFlag{
								Name:     dataFlagComponentName,
								Required: false,
								Usage:    "component_name filter",
							},
							&cli.StringFlag{
								Name:     dataFlagMethod,
								Required: false,
								Usage:    "method filter",
							},
							&cli.StringSliceFlag{
								Name:     dataFlagMimeTypes,
								Required: false,
								Usage:    "mime_types filter",
							},
							&cli.StringFlag{
								Name:     dataFlagStart,
								Required: false,
								Usage:    "ISO-8601 timestamp indicating the start of the interval filter",
							},
							&cli.StringFlag{
								Name:     dataFlagEnd,
								Required: false,
								Usage:    "ISO-8601 timestamp indicating the end of the interval filter",
							},
						},
						Action: DeleteCommand,
					},
				},
			},
			{
				Name:  "robots",
				Usage: "work with robots",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list robots",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "organization",
							},
							&cli.StringFlag{
								Name: "location",
							},
						},
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}
							orgStr := c.String("organization")
							locStr := c.String("location")
							robots, err := client.ListRobots(orgStr, locStr)
							if err != nil {
								return err
							}

							if orgStr == "" || locStr == "" {
								fmt.Fprintf(c.App.Writer, "%s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name)
							}

							for _, robot := range robots {
								fmt.Fprintf(c.App.Writer, "%s (id: %s)\n", robot.Name, robot.Id)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "robot",
				Usage: "work with a robot",
				Subcommands: []*cli.Command{
					{
						Name:  "status",
						Usage: "display robot status",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "organization",
							},
							&cli.StringFlag{
								Name: "location",
							},
							&cli.StringFlag{
								Name:     "robot",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}

							orgStr := c.String("organization")
							locStr := c.String("location")
							robot, err := client.Robot(orgStr, locStr, c.String("robot"))
							if err != nil {
								return err
							}
							parts, err := client.RobotParts(client.SelectedOrg().Id, client.SelectedLoc().Id, robot.Id)
							if err != nil {
								return err
							}

							if orgStr == "" || locStr == "" {
								fmt.Fprintf(c.App.Writer, "%s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name)
							}

							fmt.Fprintf(
								c.App.Writer,
								"ID: %s\nName: %s\nLast Access: %s (%s ago)\n",
								robot.Id,
								robot.Name,
								robot.LastAccess.AsTime().Format(time.UnixDate),
								time.Since(robot.LastAccess.AsTime()),
							)

							if len(parts) != 0 {
								fmt.Fprintln(c.App.Writer, "Parts:")
							}
							for i, part := range parts {
								name := part.Name
								if part.MainPart {
									name += " (main)"
								}
								fmt.Fprintf(
									c.App.Writer,
									"\tID: %s\n\tName: %s\n\tLast Access: %s (%s ago)\n",
									part.Id,
									name,
									part.LastAccess.AsTime().Format(time.UnixDate),
									time.Since(part.LastAccess.AsTime()),
								)
								if i != len(parts)-1 {
									fmt.Fprintln(c.App.Writer, "")
								}
							}

							return nil
						},
					},
					{
						Name:  "logs",
						Usage: "display robot logs",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "organization",
							},
							&cli.StringFlag{
								Name: "location",
							},
							&cli.StringFlag{
								Name:     "robot",
								Required: true,
							},
							&cli.BoolFlag{
								Name:  "errors",
								Usage: "show only errors",
							},
						},
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}

							orgStr := c.String("organization")
							locStr := c.String("location")
							robotStr := c.String("robot")
							robot, err := client.Robot(orgStr, locStr, robotStr)
							if err != nil {
								return err
							}

							parts, err := client.RobotParts(orgStr, locStr, robotStr)
							if err != nil {
								return err
							}

							for i, part := range parts {
								if i != 0 {
									fmt.Fprintln(c.App.Writer, "")
								}

								var header string
								if orgStr == "" || locStr == "" || robotStr == "" {
									header = fmt.Sprintf("%s -> %s -> %s -> %s", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name, part.Name)
								} else {
									header = part.Name
								}
								if err := client.PrintRobotPartLogs(
									orgStr, locStr, robotStr, part.Id,
									c.Bool("errors"),
									"\t",
									header,
								); err != nil {
									return err
								}
							}

							return nil
						},
					},
					{
						Name:  "part",
						Usage: "work with robot part",
						Subcommands: []*cli.Command{
							{
								Name:  "status",
								Usage: "display part status",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name: "organization",
									},
									&cli.StringFlag{
										Name: "location",
									},
									&cli.StringFlag{
										Name:     "robot",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "part",
										Required: true,
									},
								},
								Action: func(c *cli.Context) error {
									client, err := rdkcli.NewAppClient(c)
									if err != nil {
										return err
									}

									orgStr := c.String("organization")
									locStr := c.String("location")
									robotStr := c.String("robot")
									robot, err := client.Robot(orgStr, locStr, robotStr)
									if err != nil {
										return err
									}

									part, err := client.RobotPart(orgStr, locStr, robotStr, c.String("part"))
									if err != nil {
										return err
									}

									if orgStr == "" || locStr == "" || robotStr == "" {
										fmt.Fprintf(c.App.Writer, "%s -> %s -> %s\n", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name)
									}

									name := part.Name
									if part.MainPart {
										name += " (main)"
									}
									fmt.Fprintf(
										c.App.Writer,
										"ID: %s\nName: %s\nLast Access: %s (%s ago)\n",
										part.Id,
										name,
										part.LastAccess.AsTime().Format(time.UnixDate),
										time.Since(part.LastAccess.AsTime()),
									)

									return nil
								},
							},
							{
								Name:  "logs",
								Usage: "display part logs",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name: "organization",
									},
									&cli.StringFlag{
										Name: "location",
									},
									&cli.StringFlag{
										Name:     "robot",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "part",
										Required: true,
									},
									&cli.BoolFlag{
										Name:  "errors",
										Usage: "show only errors",
									},
									&cli.BoolFlag{
										Name:    "tail",
										Aliases: []string{"f"},
										Usage:   "follow logs",
									},
								},
								Action: func(c *cli.Context) error {
									client, err := rdkcli.NewAppClient(c)
									if err != nil {
										return err
									}

									orgStr := c.String("organization")
									locStr := c.String("location")
									robotStr := c.String("robot")
									robot, err := client.Robot(orgStr, locStr, robotStr)
									if err != nil {
										return err
									}

									var header string
									if orgStr == "" || locStr == "" || robotStr == "" {
										header = fmt.Sprintf("%s -> %s -> %s", client.SelectedOrg().Name, client.SelectedLoc().Name, robot.Name)
									}
									if c.Bool("tail") {
										return client.TailRobotPartLogs(
											orgStr, locStr, robotStr, c.String("part"),
											c.Bool("errors"),
											"",
											header,
										)
									}
									return client.PrintRobotPartLogs(
										orgStr, locStr, robotStr, c.String("part"),
										c.Bool("errors"),
										"",
										header,
									)
								},
							},
							{
								Name:      "run",
								Usage:     "run a command on a robot part",
								ArgsUsage: "<service.method>",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:     "organization",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "location",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "robot",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "part",
										Required: true,
									},
									&cli.StringFlag{
										Name:    "data",
										Aliases: []string{"d"},
									},
									&cli.DurationFlag{
										Name:    "stream",
										Aliases: []string{"s"},
									},
								},
								Action: func(c *cli.Context) error {
									svcMethod := c.Args().First()
									if svcMethod == "" {
										fmt.Fprintln(c.App.ErrWriter, "service method required")
										cli.ShowSubcommandHelpAndExit(c, 1)
										return nil
									}

									client, err := rdkcli.NewAppClient(c)
									if err != nil {
										return err
									}

									return client.RunRobotPartCommand(
										c.String("organization"),
										c.String("location"),
										c.String("robot"),
										c.String("part"),
										svcMethod,
										c.String("data"),
										c.Duration("stream"),
										c.Bool("debug"),
										logger,
									)
								},
							},
							{
								Name:  "shell",
								Usage: "start a shell on a robot part",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:     "organization",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "location",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "robot",
										Required: true,
									},
									&cli.StringFlag{
										Name:     "part",
										Required: true,
									},
								},
								Action: func(c *cli.Context) error {
									client, err := rdkcli.NewAppClient(c)
									if err != nil {
										return err
									}

									return client.StartRobotPartShell(
										c.String("organization"),
										c.String("location"),
										c.String("robot"),
										c.String("part"),
										c.Bool("debug"),
										logger,
									)
								},
							},
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// DataCommand runs the data command for downloading data from the Viam cloud.
func DataCommand(c *cli.Context) error {
	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}

	client, err := rdkcli.NewAppClient(c)
	if err != nil {
		return err
	}

	switch c.String(dataFlagDataType) {
	case dataTypeBinary:
		if err := client.BinaryData(c.String(dataFlagDestination), filter, c.Uint(dataFlagParallelDownloads)); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := client.TabularData(c.String(dataFlagDestination), filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("type must be binary or tabular, got %s", c.String("type"))
	}
	return nil
}

// DeleteCommand runs the command for deleting data from the Viam cloud.
func DeleteCommand(c *cli.Context) error {
	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}

	client, err := rdkcli.NewAppClient(c)
	if err != nil {
		return err
	}

	switch c.String(dataFlagDataType) {
	case dataTypeBinary:
		if err := client.DeleteBinaryData(filter); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := client.DeleteTabularData(filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("type must be binary or tabular, got %s", c.String("type"))
	}

	return nil
}

func createDataFilter(c *cli.Context) (*datapb.Filter, error) {
	filter := &datapb.Filter{}

	if c.StringSlice(dataFlagOrgIDs) != nil {
		filter.OrgIds = c.StringSlice(dataFlagOrgIDs)
	}
	if c.StringSlice(dataFlagLocationIDs) != nil {
		filter.LocationIds = c.StringSlice(dataFlagLocationIDs)
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
	timeLayout := time.RFC3339
	if c.String(dataFlagStart) != "" {
		t, err := time.Parse(timeLayout, c.String(dataFlagStart))
		if err != nil {
			return nil, errors.Wrap(err, "error parsing start flag")
		}
		start = timestamppb.New(t)
	}
	if c.String(dataFlagEnd) != "" {
		t, err := time.Parse(timeLayout, c.String(dataFlagEnd))
		if err != nil {
			return nil, errors.Wrap(err, "error parsing end flag")
		}
		end = timestamppb.New(t)
	}
	if start != nil || end != nil {
		filter.Interval = &datapb.CaptureInterval{
			Start: start,
			End:   end,
		}
	}
	return filter, nil
}
