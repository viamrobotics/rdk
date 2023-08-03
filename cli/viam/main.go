// Package main is the CLI command itself.
package main

import (
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
	// data command flags.
	dataFlagDestination       = "destination"
	dataFlagDataType          = "data-type"
	dataFlagOrgIDs            = "org-ids"
	dataFlagLocationIDs       = "location-ids"
	dataFlagRobotID           = "robot-id"
	dataFlagPartID            = "part-id"
	dataFlagRobotName         = "robot-name"
	dataFlagPartName          = "part-name"
	dataFlagComponentType     = "component-type"
	dataFlagComponentName     = "component-name"
	dataFlagMethod            = "method"
	dataFlagMimeTypes         = "mime-types"
	dataFlagStart             = "start"
	dataFlagEnd               = "end"
	dataFlagParallelDownloads = "parallel"
	dataFlagTags              = "tags"
	dataFlagBboxLabels        = "bbox-labels"

	dataTypeBinary  = "binary"
	dataTypeTabular = "tabular"
)

func main() {
	var logger golog.Logger

	app := &cli.App{
		Name:            "viam",
		Usage:           "interact with your Viam robots",
		HideHelpCommand: true,
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
				Usage:   "load configuration from `FILE`",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"vvv"},
				Usage:   "enable debug logging",
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("debug") {
				logger = golog.NewDebugLogger("cli")
			} else {
				logger = zap.NewNop().Sugar()
			}

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:            "auth",
				Usage:           "authenticate to app.viam.com",
				HideHelpCommand: true,
				Action: func(c *cli.Context) error {
					client, err := rdkcli.NewAppClient(c)
					if err != nil {
						return err
					}

					loggedInMessage := func(token *rdkcli.Token) {
						fmt.Fprintf(c.App.Writer, "Already authenticated as %q, expires %s\n", token.User.Email,
							token.ExpiresAt.Format("Mon Jan 2 15:04:05 MST 2006"))
					}

					if client.Config().Auth != nil && !client.Config().Auth.IsExpired() {
						loggedInMessage(client.Config().Auth)
						return nil
					}

					if err := client.Login(); err != nil {
						return err
					}

					loggedInMessage(client.Config().Auth)
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "print-access-token",
						Usage: "print the access token associated with current credentials",
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}

							if client.Config().Auth == nil || client.Config().Auth.IsExpired() {
								return errors.New("not authenticated. run \"auth\" command")
							}

							fmt.Fprintln(c.App.Writer, client.Config().Auth.AccessToken)

							return nil
						},
					},
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
					auth := client.Config().Auth
					if auth == nil {
						fmt.Fprintf(c.App.Writer, "Already logged out\n")
						return nil
					}
					if err := client.Logout(); err != nil {
						return err
					}
					fmt.Fprintf(c.App.Writer, "Logged out from %q\n", auth.User.Email)
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
					auth := client.Config().Auth
					if auth == nil {
						fmt.Fprintf(c.App.Writer, "Not logged in\n")
						return nil
					}
					fmt.Fprintf(c.App.Writer, "%s\n", auth.User.Email)
					return nil
				},
			},
			{
				Name:            "organizations",
				Usage:           "work with organizations",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list organizations for the current user",
						Action: func(c *cli.Context) error {
							client, err := rdkcli.NewAppClient(c)
							if err != nil {
								return err
							}
							orgs, err := client.ListOrganizations()
							if err != nil {
								return err
							}
							for i, org := range orgs {
								if i == 0 {
									fmt.Fprintf(c.App.Writer, "Organizations for %q:\n", client.Config().Auth.User.Email)
								}
								fmt.Fprintf(c.App.Writer, "\t%s (id: %s)\n", org.Name, org.Id)
							}
							return nil
						},
					},
				},
			},
			{
				Name:            "locations",
				Usage:           "work with locations",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:      "list",
						Usage:     "list locations for the current user",
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
									fmt.Fprintf(c.App.Writer, "\t%s (id: %s)\n", loc.Name, loc.Id)
								}
								return nil
							}
							if orgStr == "" {
								orgs, err := client.ListOrganizations()
								if err != nil {
									return err
								}
								for i, org := range orgs {
									if i == 0 {
										fmt.Fprintf(c.App.Writer, "Locations for %q:\n", client.Config().Auth.User.Email)
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
				Name:            "data",
				Usage:           "work with data",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:      "export",
						Usage:     "download data from Viam cloud",
						UsageText: fmt.Sprintf("viam data export <%s> <%s> [other options]", dataFlagDestination, dataFlagDataType),
						Flags: []cli.Flag{
							&cli.PathFlag{
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
								Name:  dataFlagOrgIDs,
								Usage: "orgs filter",
							},
							&cli.StringSliceFlag{
								Name:  dataFlagLocationIDs,
								Usage: "locations filter",
							},
							&cli.StringFlag{
								Name:  dataFlagRobotID,
								Usage: "robot-id filter",
							},
							&cli.StringFlag{
								Name:  dataFlagPartID,
								Usage: "part id filter",
							},
							&cli.StringFlag{
								Name:  dataFlagRobotName,
								Usage: "robot name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagPartName,
								Usage: "part name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagComponentType,
								Usage: "component type filter",
							},
							&cli.StringFlag{
								Name:  dataFlagComponentName,
								Usage: "component name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagMethod,
								Usage: "method filter",
							},
							&cli.StringSliceFlag{
								Name:  dataFlagMimeTypes,
								Usage: "mime types filter",
							},
							&cli.UintFlag{
								Name:        dataFlagParallelDownloads,
								Usage:       "number of download requests to make in parallel",
								DefaultText: "10",
							},
							&cli.StringFlag{
								Name:  dataFlagStart,
								Usage: "ISO-8601 timestamp indicating the start of the interval filter",
							},
							&cli.StringFlag{
								Name:  dataFlagEnd,
								Usage: "ISO-8601 timestamp indicating the end of the interval filter",
							},
							&cli.StringSliceFlag{
								Name: dataFlagTags,
								Usage: "tags filter. " +
									"accepts tagged for all tagged data, untagged for all untagged data, or a list of tags for all data matching any of the tags",
							},
							&cli.StringSliceFlag{
								Name: dataFlagBboxLabels,
								Usage: "bbox labels filter. " +
									"accepts string labels corresponding to bounding boxes within images",
							},
						},
						Action: DataCommand,
					},
					{
						Name:      "delete",
						Usage:     "delete data from Viam cloud",
						UsageText: fmt.Sprintf("viam data delete <%s> [other options]", dataFlagDataType),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     dataFlagDataType,
								Required: true,
								Usage:    "data type to be deleted: either binary or tabular",
							},
							&cli.StringSliceFlag{
								Name:  dataFlagOrgIDs,
								Usage: "orgs filter",
							},
							&cli.StringSliceFlag{
								Name:  dataFlagLocationIDs,
								Usage: "locations filter",
							},
							&cli.StringFlag{
								Name:  dataFlagRobotID,
								Usage: "robot id filter",
							},
							&cli.StringFlag{
								Name:  dataFlagPartID,
								Usage: "part id filter",
							},
							&cli.StringFlag{
								Name:  dataFlagRobotName,
								Usage: "robot name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagPartName,
								Usage: "part name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagComponentType,
								Usage: "component type filter",
							},
							&cli.StringFlag{
								Name:  dataFlagComponentName,
								Usage: "component name filter",
							},
							&cli.StringFlag{
								Name:  dataFlagMethod,
								Usage: "method filter",
							},
							&cli.StringSliceFlag{
								Name:  dataFlagMimeTypes,
								Usage: "mime types filter",
							},
							&cli.StringFlag{
								Name:  dataFlagStart,
								Usage: "ISO-8601 timestamp indicating the start of the interval filter",
							},
							&cli.StringFlag{
								Name:  dataFlagEnd,
								Usage: "ISO-8601 timestamp indicating the end of the interval filter",
							},
						},
						Action: DeleteCommand,
					},
				},
			},
			{
				Name:            "robots",
				Usage:           "work with robots",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list robots in an organization and location",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "organization",
								DefaultText: "currently selected organization",
							},
							&cli.StringFlag{
								Name:        "location",
								DefaultText: "currently selected location",
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
				Name:            "robot",
				Usage:           "work with a robot",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:      "status",
						Usage:     "display robot status",
						UsageText: "viam robot status <robot> [other options]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "organization",
								DefaultText: "currently selected organization",
							},
							&cli.StringFlag{
								Name:        "location",
								DefaultText: "currently selected location",
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
						Name:      "logs",
						Usage:     "display robot logs",
						UsageText: "viam robot logs <robot> [other options]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "organization",
								DefaultText: "currently selected organization",
							},
							&cli.StringFlag{
								Name:        "location",
								DefaultText: "currently selected location",
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
						Name:            "part",
						Usage:           "work with a robot part",
						HideHelpCommand: true,
						Subcommands: []*cli.Command{
							{
								Name:      "status",
								Usage:     "display part status",
								UsageText: "viam robot part status <robot> <part> [other options]",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:        "organization",
										DefaultText: "currently selected organization",
									},
									&cli.StringFlag{
										Name:        "location",
										DefaultText: "currently selected location",
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
								Name:      "logs",
								Usage:     "display part logs",
								UsageText: "viam robot part logs <robot> <part> [other options]",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:        "organization",
										DefaultText: "currently selected organization",
									},
									&cli.StringFlag{
										Name:        "location",
										DefaultText: "currently selected location",
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
								UsageText: "viam robot part run <organization> <location> <robot> <part> [other options] <service.method>",
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
								// TODO: remove this warning
								Description: `WARNING: Functionality of the shell command is highly experimental. In particular, there may be text-input issues
in the opened shell.

In order to use the shell command, the robot must have a valid shell type service.`,
								UsageText: "viam robot part shell <organization> <location> <robot> <part>",
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
			{
				Name:            "module",
				Usage:           "manage your modules in Viam's registry",
				HideHelpCommand: true,
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "create & register a module on app.viam.com",
						Description: `Creates a module in app.viam.com to simplify code deployment.
Ex: 'viam module create --name my-great-module --org-id <my org id>'
Will create the module and a corresponding meta.json file in the current directory.

If your org has set a namespace in app.viam.com then your module name will be 'my-namespace:my-great-module' and
you won't have to pass a namespace or org-id in future commands. Otherwise there will be no namespace
and you will have to provide the org-id to future cli commands. You cannot make your module public until you claim an org-id.

After creation, use 'viam module update' to push your new module to app.viam.com.`,
						UsageText: "viam module create <name> [other options]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "name of your module (cannot be changed once set)",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "public-namespace",
								Usage: "the public namespace where the module will reside (alternative way of specifying the org id)",
							},
							&cli.StringFlag{
								Name:  "org-id",
								Usage: "id of the organization that will host the module",
							},
						},
						Action: rdkcli.CreateModuleCommand,
					},
					{
						Name:  "update",
						Usage: "update a module's metadata on app.viam.com",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "module",
								Usage:       "path to meta.json",
								DefaultText: "./meta.json",
								TakesFile:   true,
							},
							&cli.StringFlag{
								Name:  "public-namespace",
								Usage: "the public namespace where the module resides (alternative way of specifying the org id)",
							},
							&cli.StringFlag{
								Name:  "org-id",
								Usage: "id of the organization that hosts the module",
							},
						},
						Action: rdkcli.UpdateModuleCommand,
					},
					{
						Name:  "upload",
						Usage: "upload a new version of your module",
						Description: `Upload an archive containing your module's file(s) for a specified platform

Example for linux/amd64:
tar -czf packaged-module.tar.gz my-binary   # the meta.json entrypoint is relative to the root of the archive, so it should be "./my-binary"
viam module upload --version "0.1.0" --platform "linux/amd64" packaged-module.tar.gz
                        `,
						UsageText: "viam module upload <version> <platform> [other options] <package-module.tar.gz>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "module",
								Usage:       "path to meta.json",
								DefaultText: "./meta.json",
								TakesFile:   true,
							},
							&cli.StringFlag{
								Name:  "public-namespace",
								Usage: "the public namespace where the module resides (alternative way of specifying the org id)",
							},
							&cli.StringFlag{
								Name:  "org-id",
								Usage: "id of the organization that hosts the module",
							},
							&cli.StringFlag{
								Name:  "name",
								Usage: "name of the module (used if you don't have a meta.json)",
							},
							&cli.StringFlag{
								Name:     "version",
								Usage:    "version of the module to upload (semver2.0) ex: \"0.1.0\"",
								Required: true,
							},
							&cli.StringFlag{
								Name: "platform",
								Usage: `platform of the binary you are uploading. Must be one of:
                        linux/amd64
                        linux/arm64
                        darwin/amd64 (for intel macs)
                        darwin/arm64 (for non-intel macs)`,
								Required: true,
							},
						},
						Action: rdkcli.UploadModuleCommand,
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
		if err := client.BinaryData(c.Path(dataFlagDestination), filter, c.Uint(dataFlagParallelDownloads)); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := client.TabularData(c.Path(dataFlagDestination), filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("%s must be binary or tabular, got %q", dataFlagDataType, c.String(dataFlagDataType))
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
		return errors.Errorf("%s must be binary or tabular, got %q", dataFlagDataType, c.String(dataFlagDataType))
	}

	return nil
}

func createDataFilter(c *cli.Context) (*datapb.Filter, error) {
	filter := &datapb.Filter{}

	if c.StringSlice(dataFlagOrgIDs) != nil {
		filter.OrganizationIds = c.StringSlice(dataFlagOrgIDs)
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
	if c.String(dataFlagComponentName) != "" {
		filter.ComponentName = c.String(dataFlagComponentName)
	}
	if c.String(dataFlagMethod) != "" {
		filter.Method = c.String(dataFlagMethod)
	}
	if len(c.StringSlice(dataFlagMimeTypes)) != 0 {
		filter.MimeType = c.StringSlice(dataFlagMimeTypes)
	}
	if c.StringSlice(dataFlagTags) != nil {
		switch {
		case len(c.StringSlice(dataFlagTags)) == 1 && c.StringSlice(dataFlagTags)[0] == "tagged":
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_TAGGED,
			}
		case len(c.StringSlice(dataFlagTags)) == 1 && c.StringSlice(dataFlagTags)[0] == "untagged":
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_UNTAGGED,
			}
		default:
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_MATCH_BY_OR,
				Tags: c.StringSlice(dataFlagTags),
			}
		}
	}
	if len(c.StringSlice(dataFlagBboxLabels)) != 0 {
		filter.BboxLabels = c.StringSlice(dataFlagBboxLabels)
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
