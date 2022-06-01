package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	rdkcli "go.viam.com/rdk/cli"
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
