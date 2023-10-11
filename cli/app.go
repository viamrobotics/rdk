package cli

import (
	"fmt"
	"io"

	"github.com/urfave/cli/v2"
)

// CLI flags.
const (
	baseURLFlag      = "base-url"
	configFlag       = "config"
	debugFlag        = "debug"
	organizationFlag = "organization"
	locationFlag     = "location"
	robotFlag        = "robot"
	partFlag         = "part"

	logsFlagErrors = "errors"
	logsFlagTail   = "tail"

	runFlagData   = "data"
	runFlagStream = "stream"

	apiKeyCreateFlagOrgID = "org-id"
	apiKeyCreateFlagName  = "name"
	apiKeyFlagRobotID     = "robot-id"
	apiKeyFlagLocationID  = "location-id"

	loginFlagDisableBrowser = "disable-browser-open"
	loginFlagKeyID          = "key-id"
	loginFlagKey            = "key"

	moduleFlagName            = "name"
	moduleFlagPublicNamespace = "public-namespace"
	moduleFlagOrgID           = "org-id"
	moduleFlagPath            = "module"
	moduleFlagVersion         = "version"
	moduleFlagPlatform        = "platform"
	moduleFlagForce           = "force"

	dataFlagDestination                    = "destination"
	dataFlagDataType                       = "data-type"
	dataFlagOrgIDs                         = "org-ids"
	dataFlagLocationIDs                    = "location-ids"
	dataFlagRobotID                        = "robot-id"
	dataFlagPartID                         = "part-id"
	dataFlagRobotName                      = "robot-name"
	dataFlagPartName                       = "part-name"
	dataFlagComponentType                  = "component-type"
	dataFlagComponentName                  = "component-name"
	dataFlagMethod                         = "method"
	dataFlagMimeTypes                      = "mime-types"
	dataFlagStart                          = "start"
	dataFlagEnd                            = "end"
	dataFlagParallelDownloads              = "parallel"
	dataFlagTags                           = "tags"
	dataFlagBboxLabels                     = "bbox-labels"
	dataFlagOrgID                          = "org-id"
	dataFlagDeleteTabularDataOlderThanDays = "delete-older-than-days"

	boardFlagName    = "name"
	boardFlagPath    = "path"
	boardFlagVersion = "version"
)

var app = &cli.App{
	Name:            "viam",
	Usage:           "interact with your Viam robots",
	HideHelpCommand: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   baseURLFlag,
			Hidden: true,
			Usage:  "base URL of app",
		},
		&cli.StringFlag{
			Name:    configFlag,
			Aliases: []string{"c"},
			Usage:   "load configuration from `FILE`",
		},
		&cli.BoolFlag{
			Name:    debugFlag,
			Aliases: []string{"vvv"},
			Usage:   "enable debug logging",
		},
	},
	Commands: []*cli.Command{
		{
			Name: "login",
			// NOTE(benjirewis): maintain `auth` as an alias for backward compatibility.
			Aliases:         []string{"auth"},
			Usage:           "login to app.viam.com",
			HideHelpCommand: true,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  loginFlagDisableBrowser,
					Usage: "prevent opening the default browser during login",
				},
			},
			Action: LoginAction,
			Subcommands: []*cli.Command{
				{
					Name:   "print-access-token",
					Usage:  "print the access token associated with current credentials",
					Action: PrintAccessTokenAction,
				},
				{
					Name:  "api-key",
					Usage: "authenticate with an api key",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     loginFlagKeyID,
							Required: true,
							Usage:    "id of the key to authenticate with",
						},
						&cli.StringFlag{
							Name:     loginFlagKey,
							Required: true,
							Usage:    "key to authenticate with",
						},
					},
					Action: LoginWithAPIKeyAction,
				},
			},
		},
		{
			Name:   "logout",
			Usage:  "logout from current session",
			Action: LogoutAction,
		},
		{
			Name:   "whoami",
			Usage:  "get currently logged-in user",
			Action: WhoAmIAction,
		},
		{
			Name:            "organizations",
			Aliases:         []string{"organization", "org"},
			Usage:           "work with organizations",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:   "list",
					Usage:  "list organizations for the current user",
					Action: ListOrganizationsAction,
				},
				{
					Name:  "api-key",
					Usage: "work with an organization's api keys",
					Subcommands: []*cli.Command{
						{
							Name:  "create",
							Usage: "create an api key for your organization",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     apiKeyCreateFlagOrgID,
									Required: true,
									Usage:    "the org to create an api key for",
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
							},
							Action: OrganizationsAPIKeyCreateAction,
						},
					},
				},
			},
		},
		{
			Name:            "locations",
			Aliases:         []string{"location"},
			Usage:           "work with locations",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "list",
					Usage:     "list locations for the current user",
					ArgsUsage: "[organization]",
					Action:    ListLocationsAction,
				},
				{
					Name:  "api-key",
					Usage: "work with an api-key for your location",
					Subcommands: []*cli.Command{
						{
							Name:  "create",
							Usage: "create an api key for your location",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     apiKeyFlagLocationID,
									Required: true,
									Usage:    "the location to create an api-key for",
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name: apiKeyCreateFlagOrgID,
									Usage: "the org-id to attach the key to" +
										"If not provided, will attempt to attach itself to the org of the location if only one org is attached to the location",
								},
							},
							Action: LocationAPIKeyCreateAction,
						},
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
					Name:  "export",
					Usage: "download data from Viam cloud",
					UsageText: fmt.Sprintf("viam data export <%s> <%s> [other options]",
						dataFlagDestination, dataFlagDataType),
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
					Action: DataExportAction,
				},
				{
					Name:      "delete",
					Usage:     "delete binary data from Viam cloud",
					UsageText: fmt.Sprintf("viam data delete <%s> [other options]", dataFlagDataType),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     dataFlagDataType,
							Required: true,
							Usage:    "data type to be deleted. should only be binary. if tabular, use delete-tabular instead.",
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
					Action: DataDeleteBinaryAction,
				},
				{
					Name:      "delete-tabular",
					Usage:     "delete tabular data from Viam cloud",
					UsageText: "viam data delete-tabular [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     dataFlagOrgID,
							Usage:    "org",
							Required: true,
						},
						&cli.IntFlag{
							Name:     dataFlagDeleteTabularDataOlderThanDays,
							Usage:    "delete any tabular data that is older than X calendar days before now. 0 deletes all data.",
							Required: true,
						},
					},
					Action: DataDeleteTabularAction,
				},
				{
					Name:      "dataset",
					Usage:     "add or remove data from datasets",
					UsageText: "viam data dataset [other options]",
					Subcommands: []*cli.Command{
						{
							Name:  "add",
							Usage: "adds binary data with file IDs in a single org and location to dataset",
							UsageText: fmt.Sprintf("viam data dataset add <%s> <%s> <%s> <%s> [other options]",
								datasetFlagDatasetID, dataFlagOrgID, dataFlagLocationID, dataFlagFileIDs),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     datasetFlagDatasetID,
									Usage:    "dataset ID to which data will be added",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagOrgID,
									Usage:    "org ID to which data belongs",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagLocationID,
									Usage:    "location ID to which data belongs",
									Required: true,
								},
								&cli.StringSliceFlag{
									Name:     dataFlagFileIDs,
									Usage:    "file IDs of data belonging to specified org and location",
									Required: true,
								},
							},
							Action: DataAddToDataset,
						},
						{
							Name:  "remove",
							Usage: "removes binary data with file IDs in a single org and location from dataset",
							UsageText: fmt.Sprintf("viam data dataset remove <%s> <%s> <%s> <%s> [other options]",
								datasetFlagDatasetID, dataFlagOrgID, dataFlagLocationID, dataFlagFileIDs),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     datasetFlagDatasetID,
									Usage:    "dataset ID from which data will be removed",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagOrgID,
									Usage:    "org ID to which data belongs",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagLocationID,
									Usage:    "location ID to which data belongs",
									Required: true,
								},
								&cli.StringSliceFlag{
									Name:     dataFlagFileIDs,
									Usage:    "file IDs of data belonging to specified org and location",
									Required: true,
								},
							},
							Action: DataRemoveFromDataset,
						},
					},
				},
			},
		},
		{
			Name:            "dataset",
			Usage:           "work with datasets",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "create",
					Usage: "create a new dataset",
					UsageText: fmt.Sprintf("viam dataset create <%s> <%s>",
						dataFlagOrgID, datasetFlagName),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     dataFlagOrgID,
							Required: true,
							Usage:    "org ID for which dataset will be created",
						},
						&cli.StringFlag{
							Name:     datasetFlagName,
							Required: true,
							Usage:    "name of the new dataset",
						},
					},
					Action: DatasetCreateAction,
				},
				{
					Name:  "rename",
					Usage: "rename an existing dataset",
					UsageText: fmt.Sprintf("viam dataset rename <%s> <%s>",
						datasetFlagDatasetID, datasetFlagName),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "dataset ID of the dataset that will be renamed",
						},
						&cli.StringFlag{
							Name:     datasetFlagName,
							Required: true,
							Usage:    "new name for the dataset",
						},
					},
					Action: DatasetRenameAction,
				},
				{
					Name:  "list",
					Usage: "list dataset information from specified IDs or for an org ID",
					UsageText: fmt.Sprintf("viam dataset list [<%s> | <%s>]",
						datasetFlagDatasetIDs, dataFlagOrgID),
					Flags: []cli.Flag{
						&cli.StringSliceFlag{
							Name:  datasetFlagDatasetIDs,
							Usage: "dataset IDs of datasets to be listed",
						},
						&cli.StringFlag{
							Name:  dataFlagOrgID,
							Usage: "org ID for which datasets will be listed",
						},
					},
					Action: DatasetListAction,
				},
				{
					Name:  "delete",
					Usage: "delete a dataset",
					UsageText: fmt.Sprintf("viam dataset delete <%s>",
						datasetFlagDatasetID),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "ID of the dataset to be deleted",
						},
					},
					Action: DatasetCreateAction,
				},
			},
		},
		{
			Name:      "train",
			Usage:     "train on data",
			UsageText: "viam train [other options]",
			Subcommands: []*cli.Command{
				{
					Name:  "submit",
					Usage: "submits training job on data in Viam cloud",
					UsageText: fmt.Sprintf("viam train submit <%s> [other options]",
						dataFlagOrgIDs),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagModelOrgID,
							Usage:    "org ID to train and save ML model in",
							Required: true,
						},
						&cli.StringFlag{
							Name:     trainFlagModelName,
							Usage:    "name of ML model",
							Required: true,
						},
						&cli.StringFlag{
							Name: trainFlagModelType,
							Usage: "type of model to train. can be one of " +
								"[single_label_classification, multi_label_classification, or object_detection]",
							Required: true,
						},
						&cli.StringSliceFlag{
							Name: trainFlagModelLabels,
							Usage: "labels to train on. " +
								"this will either be classification or object detection labels",
							Required: true,
						},
						&cli.StringFlag{
							Name:  trainFlagModelVersion,
							Usage: "version of ML model. defaults to current timestamp if unspecified.",
						},
						&cli.StringFlag{
							Name:  datasetFlagDatasetID,
							Usage: "dataset ID",
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
								"accepts tagged for all tagged data, untagged for all untagged data, " +
								"or a list of tags for all data matching any of the tags",
						},
						&cli.StringSliceFlag{
							Name: dataFlagBboxLabels,
							Usage: "bbox labels filter. " +
								"accepts string labels corresponding to bounding boxes within images",
						},
					},
					Action: DataSubmitTrainingJob,
				},
				{
					Name:      "get",
					Usage:     "gets training job from Viam cloud based on training job ID",
					UsageText: fmt.Sprintf("viam train get <%s>", trainFlagJobID),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: DataGetTrainingJob,
				},
				{
					Name:      "cancel",
					Usage:     "cancels training job in Viam cloud based on training job ID",
					UsageText: fmt.Sprintf("viam train cancel <%s>", trainFlagJobID),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: DataCancelTrainingJob,
				},
				{
					Name:      "list",
					Usage:     "list training jobs in Viam cloud based on organization ID",
					UsageText: fmt.Sprintf("viam train list <%s> <%s>", dataFlagOrgID, trainFlagJobStatus),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     dataFlagOrgID,
							Usage:    "org ID",
							Required: true,
						},
						&cli.StringFlag{
							Name: trainFlagJobStatus,
							Usage: "training status to filter for. can be one of " +
								"[unspecified, pending, in_progress, completed, failed, canceled, canceling]",
						},
					},
					Action: DataListTrainingJobs,
				},
			},
		},
		{
			Name:            "robots",
			Aliases:         []string{"robot"},
			Usage:           "work with robots",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "list robots in an organization and location",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        organizationFlag,
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        locationFlag,
							DefaultText: "first location alphabetically",
						},
					},
					Action: ListRobotsAction,
				},
				{
					Name:  "api-key",
					Usage: "work with a robot's api keys",
					Subcommands: []*cli.Command{
						{
							Name:  "create",
							Usage: "create an api-key for your robot",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     apiKeyFlagRobotID,
									Required: true,
									Usage:    "the robot to create an api-key for",
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name: apiKeyCreateFlagOrgID,
									Usage: "the org-id to attach this api-key to. If not provided," +
										"we will attempt to use the org attached to the robot if only one exists",
								},
							},
							Action: RobotAPIKeyCreateAction,
						},
					},
				},
				{
					Name:      "status",
					Usage:     "display robot status",
					UsageText: "viam robots status <robot> [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        organizationFlag,
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        locationFlag,
							DefaultText: "first location alphabetically",
						},
						&cli.StringFlag{
							Name:     robotFlag,
							Required: true,
						},
					},
					Action: RobotsStatusAction,
				},
				{
					Name:      "logs",
					Usage:     "display robot logs",
					UsageText: "viam robots logs <robot> [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        organizationFlag,
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        locationFlag,
							DefaultText: "first location alphabetically",
						},
						&cli.StringFlag{
							Name:     robotFlag,
							Required: true,
						},
						&cli.BoolFlag{
							Name:  logsFlagErrors,
							Usage: "show only errors",
						},
					},
					Action: RobotsLogsAction,
				},
				{
					Name:            "part",
					Usage:           "work with a robot part",
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "status",
							Usage:     "display part status",
							UsageText: "viam robots part status <robot> <part> [other options]",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:        organizationFlag,
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        locationFlag,
									DefaultText: "first location alphabetically",
								},
								&cli.StringFlag{
									Name:     robotFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     partFlag,
									Required: true,
								},
							},
							Action: RobotsPartStatusAction,
						},
						{
							Name:      "logs",
							Usage:     "display part logs",
							UsageText: "viam robots part logs <robot> <part> [other options]",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:        organizationFlag,
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        locationFlag,
									DefaultText: "first location alphabetically",
								},
								&cli.StringFlag{
									Name:     robotFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     partFlag,
									Required: true,
								},
								&cli.BoolFlag{
									Name:  logsFlagErrors,
									Usage: "show only errors",
								},
								&cli.BoolFlag{
									Name:    logsFlagTail,
									Aliases: []string{"f"},
									Usage:   "follow logs",
								},
							},
							Action: RobotsPartLogsAction,
						},
						{
							Name:      "run",
							Usage:     "run a command on a robot part",
							UsageText: "viam robots part run <organization> <location> <robot> <part> [other options] <service.method>",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     organizationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     locationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     robotFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     partFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:    runFlagData,
									Aliases: []string{"d"},
								},
								&cli.DurationFlag{
									Name:    runFlagStream,
									Aliases: []string{"s"},
								},
							},
							Action: RobotsPartRunAction,
						},
						{
							Name:        "shell",
							Usage:       "start a shell on a robot part",
							Description: `In order to use the shell command, the robot must have a valid shell type service.`,
							UsageText:   "viam robots part shell <organization> <location> <robot> <part>",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     organizationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     locationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     robotFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     partFlag,
									Required: true,
								},
							},
							Action: RobotsPartShellAction,
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
							Name:     moduleFlagName,
							Usage:    "name of your module (cannot be changed once set)",
							Required: true,
						},
						&cli.StringFlag{
							Name:  moduleFlagPublicNamespace,
							Usage: "the public namespace where the module will reside (alternative way of specifying the org id)",
						},
						&cli.StringFlag{
							Name:  moduleFlagOrgID,
							Usage: "id of the organization that will host the module",
						},
					},
					Action: CreateModuleAction,
				},
				{
					Name:  "update",
					Usage: "update a module's metadata on app.viam.com",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        moduleFlagPath,
							Usage:       "path to meta.json",
							DefaultText: "./meta.json",
							TakesFile:   true,
						},
						&cli.StringFlag{
							Name:  moduleFlagPublicNamespace,
							Usage: "the public namespace where the module resides (alternative way of specifying the org id)",
						},
						&cli.StringFlag{
							Name:  moduleFlagOrgID,
							Usage: "id of the organization that hosts the module",
						},
					},
					Action: UpdateModuleAction,
				},
				{
					Name:  "upload",
					Usage: "upload a new version of your module",
					Description: `Upload an archive containing your module's file(s) for a specified platform

Example for linux/amd64:
tar -czf packaged-module.tar.gz my-binary   # the meta.json entrypoint is relative to the root of the archive, so it should be "./my-binary"
viam module upload --version "0.1.0" --platform "linux/amd64" packaged-module.tar.gz
                      `,
					UsageText: "viam module upload <version> <platform> [other options] <packaged-module.tar.gz>",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        moduleFlagPath,
							Usage:       "path to meta.json",
							DefaultText: "./meta.json",
							TakesFile:   true,
						},
						&cli.StringFlag{
							Name:  moduleFlagPublicNamespace,
							Usage: "the public namespace where the module resides (alternative way of specifying the org id)",
						},
						&cli.StringFlag{
							Name:  moduleFlagOrgID,
							Usage: "id of the organization that hosts the module",
						},
						&cli.StringFlag{
							Name:  moduleFlagName,
							Usage: "name of the module (used if you don't have a meta.json)",
						},
						&cli.StringFlag{
							Name:     moduleFlagVersion,
							Usage:    "version of the module to upload (semver2.0) ex: \"0.1.0\"",
							Required: true,
						},
						&cli.StringFlag{
							Name: moduleFlagPlatform,
							Usage: `platform of the binary you are uploading. Must be one of:
                      linux/amd64
                      linux/arm64
                      darwin/amd64 (for intel macs)
                      darwin/arm64 (for non-intel macs)`,
							Required: true,
						},
						&cli.BoolFlag{
							Name:  moduleFlagForce,
							Usage: "skip validation (may result in non-functional versions)",
						},
					},
					Action: UploadModuleAction,
				},
			},
		},
		{
			Name:   "version",
			Usage:  "print version info for this program",
			Action: VersionAction,
		},
		{
			Name:            "board",
			Usage:           "manage your board definition files",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "upload",
					Usage: "upload a board definition file",
					Description: `Upload a json board definition file for linux boards.
Example:
viam board upload --name=orin --organization="my org" --version=1.0.0 file.json`,
					UsageText: "viam board upload <name> <organization> <version> [other options] <file.json>",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     boardFlagName,
							Usage:    "name of your board definition file (cannot be changed once set)",
							Required: true,
						},
						&cli.StringFlag{
							Name:     organizationFlag,
							Usage:    "organization that will host the board definitions file. This can be the org's ID or name",
							Required: true,
						},
						&cli.StringFlag{
							Name:     boardFlagVersion,
							Usage:    "version of the file to upload (semver2.0) ex: \"0.1.0\"",
							Required: true,
						},
					},
					Action: UploadBoardDefsAction,
				},
				{
					Name:  "download",
					Usage: "download a board definitions package",
					Description: `download a json board definitions file for generic linux boards.
Example:
viam board download --name=test --organization="my org" --version=1.0.0`,
					UsageText: "viam board download <name> <organization> <version> [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     boardFlagName,
							Usage:    "name of the board definitions file to download",
							Required: true,
						},
						&cli.StringFlag{
							Name:     organizationFlag,
							Usage:    "organization that hosts the board definitions file",
							Required: true,
						},
						&cli.StringFlag{
							Name:  boardFlagVersion,
							Usage: "version of the file to download. defaults to latest if not set.",
						},
					},
					Action: DownloadBoardDefsAction,
				},
				{
					Name:  "list",
					Usage: "list all board defintions packages",
					Description: `list the board defintions packages available from an organization.
Example:
viam board list --organization="my org"`,
					UsageText: "viam board list <organization>[other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     organizationFlag,
							Usage:    "organization that hosts the board definitions files",
							Required: true,
						},
					},
					Action: ListBoardDefsAction,
				},
			},
		},
	},
}

// NewApp returns a new app with the CLI API, Writer set to out, and ErrWriter
// set to errOut.
func NewApp(out, errOut io.Writer) *cli.App {
	app.Writer = out
	app.ErrWriter = errOut
	return app
}
