package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"
)

// CLI flags.
const (
	baseURLFlag      = "base-url"
	configFlag       = "config"
	debugFlag        = "debug"
	organizationFlag = "organization"
	locationFlag     = "location"
	machineFlag      = "machine"
	aliasRobotFlag   = "robot"
	partFlag         = "part"

	logsFlagErrors = "errors"
	logsFlagTail   = "tail"

	runFlagData   = "data"
	runFlagStream = "stream"

	apiKeyCreateFlagOrgID  = "org-id"
	apiKeyCreateFlagName   = "name"
	apiKeyFlagMachineID    = "machine-id"
	apiKeyFlagAliasRobotID = "robot-id"
	apiKeyFlagLocationID   = "location-id"

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

	moduleBuildFlagPath     = "module"
	moduleBuildFlagRef      = "ref"
	moduleBuildFlagCount    = "count"
	moduleBuildFlagVersion  = "version"
	moduleBuildFlagBuildID  = "id"
	moduleBuildFlagPlatform = "platform"
	moduleBuildFlagWait     = "wait"

	dataFlagDestination                    = "destination"
	dataFlagDataType                       = "data-type"
	dataFlagOrgIDs                         = "org-ids"
	dataFlagLocationIDs                    = "location-ids"
	dataFlagMachineID                      = "machine-id"
	dataFlagAliasRobotID                   = "robot-id"
	dataFlagPartID                         = "part-id"
	dataFlagMachineName                    = "machine-name"
	dataFlagAliasRobotName                 = "robot-name"
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
	dataFlagDatabasePassword               = "password"

	boardFlagName    = "name"
	boardFlagPath    = "path"
	boardFlagVersion = "version"
)

// createUsageText is a helper for formatting the flags, if otherOptions is set to true
// then [other options] is appended to the end of the text
func createUsageText(command string, flags []string, otherOptions bool) string {
	formattedFlags := make([]string, len(flags)+1)
	for i, flag := range flags {
		formattedFlags[i] = fmt.Sprintf("--%s=<%s>", flag, flag)
	}
	if otherOptions {
		lastIdx := len(flags) - 1
		if len(flags) == 0 {
			lastIdx = 0
		}
		formattedFlags[lastIdx] = "[other options]"
	}
	return fmt.Sprintf("%s %s", command, strings.Join(formattedFlags, " "))
}

var app = &cli.App{
	Name:            "viam",
	Usage:           "interact with your Viam machines",
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
					Name:      "export",
					Usage:     "download data from Viam cloud",
					UsageText: createUsageText("viam data export", []string{dataFlagDestination, dataFlagDataType}, true),
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
						&AliasStringFlag{
							cli.StringFlag{
								Name:    dataFlagMachineID,
								Aliases: []string{dataFlagAliasRobotID},
								Usage:   "machine id filter",
							},
						},
						&cli.StringFlag{
							Name:  dataFlagPartID,
							Usage: "part id filter",
						},
						&AliasStringFlag{
							cli.StringFlag{
								Name:    dataFlagMachineName,
								Aliases: []string{dataFlagAliasRobotName},
								Usage:   "machine name filter",
							},
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
							Name:  dataFlagParallelDownloads,
							Usage: "number of download requests to make in parallel",
							Value: 100,
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
					Name:            "delete",
					Usage:           "delete data from Viam cloud",
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "binary",
							Usage:     "delete binary data from Viam cloud",
							UsageText: createUsageText("viam data delete binary", nil, true),
							Flags: []cli.Flag{
								&cli.StringSliceFlag{
									Name:  dataFlagOrgIDs,
									Usage: "orgs filter",
								},
								&cli.StringSliceFlag{
									Name:  dataFlagLocationIDs,
									Usage: "locations filter",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    dataFlagMachineID,
										Aliases: []string{dataFlagAliasRobotID},
										Usage:   "machine id filter",
									},
								},
								&cli.StringFlag{
									Name:  dataFlagPartID,
									Usage: "part id filter",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    dataFlagMachineName,
										Aliases: []string{dataFlagAliasRobotName},
										Usage:   "machine name filter",
									},
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
							Name:      "tabular",
							Usage:     "delete tabular data from Viam cloud",
							UsageText: createUsageText("viam data delete tabular", nil, true),
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
					},
				},
				{
					Name:      "database",
					Usage:     "interact with a MongoDB Atlas Data Federation instance",
					UsageText: "viam data database [other options]",
					Subcommands: []*cli.Command{
						{
							Name:      "configure",
							Usage:     "configures a database user for the Viam org's MongoDB Atlas Data Federation instance",
							UsageText: createUsageText("viam data database configure", []string{dataFlagOrgID, dataFlagDatabasePassword}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     dataFlagOrgID,
									Usage:    "org ID for the database user being configured",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagDatabasePassword,
									Usage:    "password for the database user being configured",
									Required: true,
								},
							},
							Action: DataConfigureDatabaseUser,
						},
						{
							Name:      "hostname",
							Usage:     "gets the hostname to access a MongoDB Atlas Data Federation Instance",
							UsageText: createUsageText("viam data database hostname", []string{dataFlagOrgID}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     dataFlagOrgID,
									Usage:    "org ID for the database user",
									Required: true,
								},
							},
							Action: DataGetDatabaseConnection,
						},
					},
				},
				{
					Name:      "dataset",
					Usage:     "add or remove data from datasets",
					UsageText: createUsageText("viam data dataset", nil, true),
					Subcommands: []*cli.Command{
						{
							Name:  "add",
							Usage: "adds binary data either by IDs or filter to dataset",
							Subcommands: []*cli.Command{
								{
									Name:  "ids",
									Usage: "adds binary data with file IDs in a single org and location to dataset",
									UsageText: createUsageText("viam data dataset add ids", []string{
										datasetFlagDatasetID, dataFlagOrgID,
										dataFlagLocationID, dataFlagFileIDs,
									}, false),
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
									Action: DataAddToDatasetByIDs,
								},

								{
									Name:      "filter",
									UsageText: createUsageText("viam data dataset add filter", nil, true),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     datasetFlagDatasetID,
											Usage:    "dataset ID to which data will be added",
											Required: true,
										},
										&cli.StringSliceFlag{
											Name:  dataFlagOrgIDs,
											Usage: "orgs filter",
										},
										&cli.StringSliceFlag{
											Name:  dataFlagLocationIDs,
											Usage: "locations filter",
										},
										&AliasStringFlag{
											cli.StringFlag{
												Name:    dataFlagMachineID,
												Aliases: []string{dataFlagAliasRobotID},
												Usage:   "machine id filter",
											},
										},
										&cli.StringFlag{
											Name:  dataFlagPartID,
											Usage: "part id filter",
										},
										&AliasStringFlag{
											cli.StringFlag{
												Name:    dataFlagMachineName,
												Aliases: []string{dataFlagAliasRobotName},
												Usage:   "machine name filter",
											},
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
												"accepts tagged for all tagged data, untagged for all untagged data, or a list of tags for all data matching any of the tags",
										},
										&cli.StringSliceFlag{
											Name: dataFlagBboxLabels,
											Usage: "bbox labels filter. " +
												"accepts string labels corresponding to bounding boxes within images",
										},
									},
									Action: DataAddToDatasetByFilter,
								},
							},
						},
						{
							Name:  "remove",
							Usage: "removes binary data with file IDs in a single org and location from dataset",
							UsageText: createUsageText("viam data dataset remove",
								[]string{datasetFlagDatasetID, dataFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false),
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
					Name:      "create",
					Usage:     "create a new dataset",
					UsageText: createUsageText("viam dataset create", []string{dataFlagOrgID, datasetFlagName}, false),
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
					UsageText: createUsageText("viam dataset rename",
						[]string{datasetFlagDatasetID, datasetFlagName}, false),
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
					UsageText: fmt.Sprintf("viam dataset list [--%s=<%s> | --%s=<%s>]",
						datasetFlagDatasetIDs, datasetFlagDatasetIDs, dataFlagOrgID, dataFlagOrgID),
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
					Name:      "delete",
					Usage:     "delete a dataset",
					UsageText: createUsageText("viam dataset delete", []string{datasetFlagDatasetID}, false),
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
					UsageText: createUsageText("viam train submit",
						[]string{datasetFlagDatasetID, trainFlagModelOrgID, trainFlagModelName, trainFlagModelType, trainFlagModelLabels}, true),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Usage:    "dataset ID",
							Required: true,
						},
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
					},
					Action: DataSubmitTrainingJob,
				},
				{
					Name:      "get",
					Usage:     "gets training job from Viam cloud based on training job ID",
					UsageText: createUsageText("viam train get", []string{trainFlagJobID}, false),
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
					UsageText: createUsageText("viam train cancel", []string{trainFlagJobID}, false),
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
					UsageText: createUsageText("viam train list", []string{dataFlagOrgID, trainFlagJobStatus}, false),
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
			Name:            "machines",
			Aliases:         []string{"machine", "robots", "robot"},
			Usage:           "work with machines",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "list machines in an organization and location",
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
					Usage: "work with a machine's api keys",
					Subcommands: []*cli.Command{
						{
							Name:  "create",
							Usage: "create an api-key for your machine",
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     apiKeyFlagMachineID,
										Aliases:  []string{apiKeyFlagAliasRobotID},
										Required: true,
										Usage:    "the machine to create an api-key for",
									},
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name: apiKeyCreateFlagOrgID,
									Usage: "the org-id to attach this api-key to. If not provided," +
										"we will attempt to use the org attached to the machine if only one exists",
								},
							},
							Action: RobotAPIKeyCreateAction,
						},
					},
				},
				{
					Name:      "status",
					Usage:     "display machine status",
					UsageText: "viam machines status <machine> [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        organizationFlag,
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        locationFlag,
							DefaultText: "first location alphabetically",
						},
						&AliasStringFlag{
							cli.StringFlag{
								Name:     machineFlag,
								Aliases:  []string{aliasRobotFlag},
								Required: true,
							},
						},
					},
					Action: RobotsStatusAction,
				},
				{
					Name:      "logs",
					Aliases:   []string{"log"},
					Usage:     "display machine logs",
					UsageText: "viam machines logs <machine> [other options]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        organizationFlag,
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        locationFlag,
							DefaultText: "first location alphabetically",
						},
						&AliasStringFlag{
							cli.StringFlag{
								Name:     machineFlag,
								Aliases:  []string{aliasRobotFlag},
								Required: true,
							},
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
					Usage:           "work with a machine part",
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "status",
							Usage:     "display part status",
							UsageText: "viam machines part status <machine> <part> [other options]",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:        organizationFlag,
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        locationFlag,
									DefaultText: "first location alphabetically",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:     machineFlag,
										Aliases:  []string{aliasRobotFlag},
										Required: true,
									},
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
							Aliases:   []string{"log"},
							Usage:     "display part logs",
							UsageText: "viam machines part logs <machine> <part> [other options]",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:        organizationFlag,
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        locationFlag,
									DefaultText: "first location alphabetically",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:     machineFlag,
										Aliases:  []string{aliasRobotFlag},
										Required: true,
									},
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
							Usage:     "run a command on a machine part",
							UsageText: "viam machines part run <organization> <location> <machine> <part> [other options] <service.method>",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     organizationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     locationFlag,
									Required: true,
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:     machineFlag,
										Aliases:  []string{aliasRobotFlag},
										Required: true,
									},
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
							Usage:       "start a shell on a machine part",
							Description: `In order to use the shell command, the machine must have a valid shell type service.`,
							UsageText:   "viam machines part shell <organization> <location> <machine> <part>",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     organizationFlag,
									Required: true,
								},
								&cli.StringFlag{
									Name:     locationFlag,
									Required: true,
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:     machineFlag,
										Aliases:  []string{aliasRobotFlag},
										Required: true,
									},
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
							Name:      moduleFlagPath,
							Usage:     "path to meta.json",
							Value:     "./meta.json",
							TakesFile: true,
						},
					},
					Action: UpdateModuleAction,
				},
				{
					Name:  "update-models",
					Usage: "update a module's metadata file based on models it provides",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:      moduleFlagPath,
							Usage:     "path to meta.json",
							Value:     "./meta.json",
							TakesFile: true,
						},
						&cli.StringFlag{
							Name:     "binary",
							Usage:    "binary for the module to run (has to work on this os/processor)",
							Required: true,
						},
					},
					Action: UpdateModelsAction,
				},
				{
					Name:  "upload",
					Usage: "upload a new version of your module",
					Description: `Upload an archive containing your module's file(s) for a specified platform
Example uploading a single file:
viam module upload --version "0.1.0" --platform "linux/amd64" ./bin/my-module
(this example requires the entrypoint in the meta.json to be "./bin/my-module")

Example uploading a whole directory:
viam module upload --version "0.1.0" --platform "linux/amd64" ./bin
(this example requires the entrypoint in the meta.json to be inside the bin directory like "./bin/[your path here]")

Example uploading a custom tarball of your module:
tar -czf packaged-module.tar.gz ./src requirements.txt run.sh
viam module upload --version "0.1.0" --platform "linux/amd64" packaged-module.tar.gz
                      `,
					UsageText: "viam module upload <version> <platform> [other options] <packaged-module.tar.gz>",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:      moduleFlagPath,
							Usage:     "path to meta.json",
							Value:     "./meta.json",
							TakesFile: true,
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
                      any           (most Python modules)
                      any/amd64     (most Docker-based modules)
                      any/arm64
                      linux/any     (Python modules that also require OS support)
                      darwin/any
                      linux/amd64
                      linux/arm64
                      linux/arm32v7
                      linux/arm32v6
                      darwin/amd64  (Intel macs)
                      darwin/arm64  (Apple silicon macs)`,
							Required: true,
						},
						&cli.BoolFlag{
							Name:  moduleFlagForce,
							Usage: "skip validation (may result in non-functional versions)",
						},
					},
					Action: UploadModuleAction,
				},
				{
					Name:   "build",
					Hidden: true,
					Usage: `build your module on different operating systems and cpu architectures via cloud runners.
Uses the "build" section of your meta.json.
Example:
"build": {
   "setup": "setup.sh",                    // optional - command to install your build dependencies
   "build": "make module.tar.gz",          // command that will build your module
   "path" : "module.tar.gz",               // optional - path to your built module
                                           // (passed to the 'viam module upload' command)
   "arch" : ["linux/amd64", "linux/arm64"] // architectures to build for
}`,
					Subcommands: []*cli.Command{
						{
							Name:  "local",
							Usage: "run your module's build commands locally",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
							},
							Action: ModuleBuildLocalAction,
						},
						{
							Name:        "start",
							Description: "start a remote build",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleBuildFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
								&cli.StringFlag{
									Name:     moduleBuildFlagVersion,
									Usage:    "version of the module to upload (semver2.0) ex: \"0.1.0\"",
									Required: true,
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagRef,
									Usage: "git ref to clone when building your module. This can be a branch name or a commit hash",
									Value: "main",
								},
							},
							Action: ModuleBuildStartAction,
						},
						{
							Name:  "list",
							Usage: "check on the status of your cloud builds",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
								&cli.IntFlag{
									Name:        moduleBuildFlagCount,
									Usage:       "number of builds to list",
									Aliases:     []string{"c"},
									DefaultText: "all",
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagBuildID,
									Usage: "restrict output to just return builds that match this id",
								},
							},
							Action: ModuleBuildListAction,
						},
						{
							Name:    "logs",
							Aliases: []string{"log"},
							Usage:   "get the logs from one of your cloud builds",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     moduleBuildFlagBuildID,
									Usage:    "build that you want to get the logs for",
									Required: true,
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagPlatform,
									Usage: "build platform to get the logs for. Ex: linux/arm64. If a platform is not provided, it returns logs for all platforms",
								},
								&cli.BoolFlag{
									Name:  moduleBuildFlagWait,
									Usage: "wait for the build to finish before outputting any logs",
								},
							},
							Action: ModuleBuildLogsAction,
						},
					},
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
