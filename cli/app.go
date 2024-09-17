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

	// TODO: RSDK-6683.
	quietFlag = "quiet"

	logsFlagErrors = "errors"
	logsFlagTail   = "tail"
	logsFlagCount  = "count"

	runFlagData   = "data"
	runFlagStream = "stream"

	loginFlagDisableBrowser = "disable-browser-open"
	loginFlagKeyID          = "key-id"
	loginFlagKey            = "key"

	// Flags shared by api-key, module, ml-training and data subcommands.
	generalFlagOrgID        = "org-id"
	generalFlagLocationID   = "location-id"
	generalFlagMachineID    = "machine-id"
	generalFlagAliasRobotID = "robot-id"

	apiKeyCreateFlagName = "name"

	moduleFlagName            = "name"
	moduleFlagPublicNamespace = "public-namespace"
	moduleFlagPath            = "module"
	moduleFlagVersion         = "version"
	moduleFlagPlatform        = "platform"
	moduleFlagForce           = "force"
	moduleFlagBinary          = "binary"
	moduleFlagLocal           = "local"
	moduleFlagHomeDir         = "home"
	moduleCreateLocalOnly     = "local-only"
	moduleFlagID              = "id"

	moduleBuildFlagPath      = "module"
	moduleBuildFlagRef       = "ref"
	moduleBuildFlagCount     = "count"
	moduleBuildFlagVersion   = "version"
	moduleBuildFlagBuildID   = "id"
	moduleBuildFlagPlatform  = "platform"
	moduleBuildFlagWait      = "wait"
	moduleBuildFlagGroupLogs = "group-logs"
	moduleBuildRestartOnly   = "restart-only"
	moduleBuildFlagNoBuild   = "no-build"
	moduleBuildFlagOAuthLink = "oauth-link"
	moduleBuildFlagRepo      = "repo"

	mlTrainingFlagPath        = "path"
	mlTrainingFlagName        = "script-name"
	mlTrainingFlagVersion     = "version"
	mlTrainingFlagFramework   = "framework"
	mlTrainingFlagType        = "type"
	mlTrainingFlagDraft       = "draft"
	mlTrainingFlagVisibility  = "visibility"
	mlTrainingFlagDescription = "description"
	mlTrainingFlagURL         = "url"

	dataFlagDestination                    = "destination"
	dataFlagDataType                       = "data-type"
	dataFlagOrgIDs                         = "org-ids"
	dataFlagLocationIDs                    = "location-ids"
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
	dataFlagChunkLimit                     = "chunk-limit"
	dataFlagParallelDownloads              = "parallel"
	dataFlagTags                           = "tags"
	dataFlagBboxLabels                     = "bbox-labels"
	dataFlagDeleteTabularDataOlderThanDays = "delete-older-than-days"
	dataFlagDatabasePassword               = "password"
	dataFlagFilterTags                     = "filter-tags"

	packageFlagName        = "name"
	packageFlagVersion     = "version"
	packageFlagType        = "type"
	packageFlagDestination = "destination"
	packageFlagPath        = "path"

	authApplicationFlagName          = "application-name"
	authApplicationFlagApplicationID = "application-id"
	authApplicationFlagOriginURIs    = "origin-uris"
	authApplicationFlagRedirectURIs  = "redirect-uris"
	authApplicationFlagLogoutURI     = "logout-uri"

	cpFlagRecursive = "recursive"
	cpFlagPreserve  = "preserve"
)

var commonFilterFlags = []cli.Flag{
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
			Name:    generalFlagMachineID,
			Aliases: []string{generalFlagAliasRobotID},
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
		Name: dataFlagBboxLabels,
		Usage: "bbox labels filter. " +
			"accepts string labels corresponding to bounding boxes within images",
	},
}

var dataTagByIDsFlags = []cli.Flag{
	&cli.StringSliceFlag{
		Name:     dataFlagTags,
		Required: true,
		Usage:    "comma separated tags to add/remove to the data",
	},
	&cli.StringFlag{
		Name:     generalFlagOrgID,
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
		Usage:    "comma separated file IDs of data belonging to specified org and location",
		Required: true,
	},
}

var dataTagByFilterFlags = append([]cli.Flag{
	&cli.StringSliceFlag{
		Name:     dataFlagTags,
		Required: true,
		Usage:    "comma separated tags to add/remove to the data",
	},
	&cli.StringSliceFlag{
		Name: dataFlagFilterTags,
		Usage: "tags filter. " +
			"accepts tagged for all tagged data, untagged for all untagged data, or a list of tags for all data matching any of the tags",
	},
},
	commonFilterFlags...)

// createUsageText is a helper for formatting UsageTexts. The created UsageText
// contains "viam", the command, requiredFlags, [other options] if otherOptions
// is true, and all passed-in arguments in that order.
func createUsageText(command string, requiredFlags []string, otherOptions bool, arguments ...string) string {
	formatted := []string{"viam", command}
	for _, flag := range requiredFlags {
		formatted = append(formatted, fmt.Sprintf("--%s=<%s>", flag, flag))
	}
	if otherOptions {
		formatted = append(formatted, "[other options]")
	}
	formatted = append(formatted, arguments...)
	return strings.Join(formatted, " ")
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
		&cli.BoolFlag{
			Name:    quietFlag,
			Value:   false,
			Aliases: []string{"q"},
			Usage:   "suppress warnings",
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
			After:  CheckUpdateAction,
			Subcommands: []*cli.Command{
				{
					Name:   "print-access-token",
					Usage:  "print the access token associated with current credentials",
					Action: PrintAccessTokenAction,
				},
				{
					Name:      "api-key",
					Usage:     "authenticate with an api key",
					UsageText: createUsageText("login api-key", []string{loginFlagKeyID, loginFlagKey}, false),
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
					Name:      "api-key",
					Usage:     "work with an organization's api keys",
					UsageText: createUsageText("organizations api-key", []string{generalFlagOrgID}, true),
					Subcommands: []*cli.Command{
						{
							Name:  "create",
							Usage: "create an api key for your organization",
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
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
							Name:      "create",
							Usage:     "create an api key for your location",
							UsageText: createUsageText("api-key create", []string{generalFlagOrgID}, true),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagLocationID,
									Required: true,
									Usage:    "the location to create an api-key for",
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name: generalFlagOrgID,
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
					UsageText: createUsageText("data export", []string{dataFlagDestination, dataFlagDataType}, true),
					Flags: append([]cli.Flag{
						&cli.PathFlag{
							Name:     dataFlagDestination,
							Required: true,
							Usage:    "output directory for downloaded data",
						},
						&cli.UintFlag{
							Name:  dataFlagChunkLimit,
							Usage: "maximum number of results per download request (tabular data only)",
							Value: 100000,
						},
						&cli.UintFlag{
							Name:  dataFlagParallelDownloads,
							Usage: "number of download requests to make in parallel (binary data only)",
							Value: 100,
						},
						&cli.StringSliceFlag{
							Name: dataFlagTags,
							Usage: "tags filter. " +
								"accepts tagged for all tagged data, untagged for all untagged data, or a list of tags for all data matching any of the tags",
						},
						&cli.StringFlag{
							Name:  dataFlagDataType,
							Usage: "type of data to download. can be binary or tabular",
						},
					},
						commonFilterFlags...),
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
							UsageText: createUsageText("data delete binary", nil, true),
							Flags: []cli.Flag{
								&cli.StringSliceFlag{
									Name:     dataFlagOrgIDs,
									Required: true,
									Usage:    "orgs filter",
								},
								&cli.StringFlag{
									Name:     dataFlagStart,
									Required: true,
									Usage:    "ISO-8601 timestamp indicating the start of the interval filter",
								},
								&cli.StringFlag{
									Name:     dataFlagEnd,
									Required: true,
									Usage:    "ISO-8601 timestamp indicating the end of the interval filter",
								},
								&cli.StringSliceFlag{
									Name:  dataFlagLocationIDs,
									Usage: "locations filter",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachineID,
										Aliases: []string{generalFlagAliasRobotID},
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
							},
							Action: DataDeleteBinaryAction,
						},
						{
							Name:      "tabular",
							Usage:     "delete tabular data from Viam cloud",
							UsageText: createUsageText("data delete tabular", []string{generalFlagOrgID, dataFlagDeleteTabularDataOlderThanDays}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
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
					UsageText: createUsageText("data database", nil, true),
					Subcommands: []*cli.Command{
						{
							Name:      "configure",
							Usage:     "configures a database user for the Viam org's MongoDB Atlas Data Federation instance",
							UsageText: createUsageText("data database configure", []string{generalFlagOrgID, dataFlagDatabasePassword}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Usage:    "org ID for the database user being configured",
									Required: true,
								},
								&cli.StringFlag{
									Name:     dataFlagDatabasePassword,
									Usage:    "password for the database user being configured",
									Required: true,
								},
							},
							Before: DataConfigureDatabaseUserConfirmation,
							Action: DataConfigureDatabaseUser,
						},
						{
							Name:      "hostname",
							Usage:     "gets the hostname to access a MongoDB Atlas Data Federation Instance",
							UsageText: createUsageText("data database hostname", []string{generalFlagOrgID}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Usage:    "org ID for the database user",
									Required: true,
								},
							},
							Action: DataGetDatabaseConnection,
						},
					},
				},
				{
					Name:      "tag",
					Usage:     "tag binary data by filter or ids",
					UsageText: createUsageText("data tag", nil, true),
					Subcommands: []*cli.Command{
						{
							Name:      "ids",
							Usage:     "adds or removes tags from binary data by file ids for a given org and location",
							UsageText: createUsageText("data tag ids", nil, true),
							Subcommands: []*cli.Command{
								{
									Name:  "add",
									Usage: "adds tags to binary data by file ids for a given org and location",
									UsageText: createUsageText("data tag ids add", []string{
										dataFlagTags, generalFlagOrgID,
										dataFlagLocationID, dataFlagFileIDs,
									}, false),
									Flags:  dataTagByIDsFlags,
									Action: DataTagActionByIds,
								},
								{
									Name:  "remove",
									Usage: "removes tags from binary data by file ids for a given org and location",
									UsageText: createUsageText("data tag ids remove", []string{
										dataFlagTags, generalFlagOrgID,
										dataFlagLocationID, dataFlagFileIDs,
									}, false),
									Flags:  dataTagByIDsFlags,
									Action: DataTagActionByIds,
								},
							},
						},
						{
							Name:      "filter",
							Usage:     "adds or removes tags from binary data by filter",
							UsageText: createUsageText("data tag filter", []string{dataFlagTags}, false),
							Subcommands: []*cli.Command{
								{
									Name:  "add",
									Usage: "adds tags to binary data by filter",
									UsageText: createUsageText("data tag filter add", []string{
										dataFlagTags,
									}, false),
									Flags:  dataTagByFilterFlags,
									Action: DataTagActionByFilter,
								},
								{
									Name:  "remove",
									Usage: "removes tags from binary data by filter",
									UsageText: createUsageText("data tag filter remove", []string{
										dataFlagTags,
									}, false),
									Flags:  dataTagByFilterFlags,
									Action: DataTagActionByFilter,
								},
							},
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
					UsageText: createUsageText("dataset create", []string{generalFlagOrgID, datasetFlagName}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
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
					UsageText: createUsageText("dataset rename",
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
						datasetFlagDatasetIDs, datasetFlagDatasetIDs, generalFlagOrgID, generalFlagOrgID),
					Flags: []cli.Flag{
						&cli.StringSliceFlag{
							Name:  datasetFlagDatasetIDs,
							Usage: "dataset IDs of datasets to be listed",
						},
						&cli.StringFlag{
							Name:  generalFlagOrgID,
							Usage: "org ID for which datasets will be listed",
						},
					},
					Action: DatasetListAction,
				},
				{
					Name:      "delete",
					Usage:     "delete a dataset",
					UsageText: createUsageText("dataset delete", []string{datasetFlagDatasetID}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "ID of the dataset to be deleted",
						},
					},
					Action: DatasetDeleteAction,
				},
				{
					Name:  "export",
					Usage: "download data from a dataset",
					UsageText: createUsageText("dataset export",
						[]string{dataFlagDestination, datasetFlagDatasetID}, true, datasetFlagIncludeJSONLines, dataFlagParallelDownloads),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:     dataFlagDestination,
							Required: true,
							Usage:    "output directory for downloaded data",
						},
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "dataset ID of the dataset to be downloaded",
						},
						&cli.BoolFlag{
							Name:     datasetFlagIncludeJSONLines,
							Required: false,
							Usage:    "option to include JSON Lines files for local testing",
							Value:    false,
						},
						&cli.UintFlag{
							Name:     dataFlagParallelDownloads,
							Required: false,
							Usage:    "number of download requests to make in parallel",
							Value:    100,
						},
					},
					Action: DatasetDownloadAction,
				},
				{
					Name:      "data",
					Usage:     "add or remove data from datasets",
					UsageText: createUsageText("dataset data", nil, true),
					Subcommands: []*cli.Command{
						{
							Name:  "add",
							Usage: "adds binary data either by IDs or filter to dataset",
							Subcommands: []*cli.Command{
								{
									Name:  "ids",
									Usage: "adds binary data with file IDs in a single org and location to dataset",
									UsageText: createUsageText("dataset data add ids", []string{
										datasetFlagDatasetID, generalFlagOrgID,
										dataFlagLocationID, dataFlagFileIDs,
									}, false),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     datasetFlagDatasetID,
											Usage:    "dataset ID to which data will be added",
											Required: true,
										},
										&cli.StringFlag{
											Name:     generalFlagOrgID,
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
									Usage:     "adds binary data from the specified filter to dataset",
									UsageText: createUsageText("dataset data add filter", []string{datasetFlagDatasetID}, true),
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
												Name:    generalFlagMachineID,
												Aliases: []string{generalFlagAliasRobotID},
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
							UsageText: createUsageText("dataset data remove",
								[]string{datasetFlagDatasetID, generalFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     datasetFlagDatasetID,
									Usage:    "dataset ID from which data will be removed",
									Required: true,
								},
								&cli.StringFlag{
									Name:     generalFlagOrgID,
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
			Name:      "train",
			Usage:     "train on data",
			UsageText: "viam train [other options]",
			Subcommands: []*cli.Command{
				{
					Name:  "submit",
					Usage: "submits training job on data in Viam cloud",
					Subcommands: []*cli.Command{
						{
							Name:  "managed",
							Usage: "submits training job on data in Viam cloud with a Viam-managed training script",
							UsageText: createUsageText("train submit managed",
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
							Action: MLSubmitTrainingJob,
						},
						{
							Name:  "custom",
							Usage: "submits custom training job on data in Viam cloud",
							Subcommands: []*cli.Command{
								{
									Name:  "from-registry",
									Usage: "submits custom training job with an existing training script in the registry on data in Viam cloud",

									UsageText: createUsageText("train submit custom from-registry",
										[]string{
											datasetFlagDatasetID, generalFlagOrgID, trainFlagModelName,
											mlTrainingFlagName, mlTrainingFlagVersion,
										}, true),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     datasetFlagDatasetID,
											Usage:    "dataset ID",
											Required: true,
										},
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Usage:    "org ID to train and save ML model in",
											Required: true,
										},
										&cli.StringFlag{
											Name:     trainFlagModelName,
											Usage:    "name of ML model",
											Required: true,
										},
										&cli.StringFlag{
											Name:  trainFlagModelVersion,
											Usage: "version of ML model. defaults to current timestamp if unspecified.",
										},
										&cli.StringFlag{
											Name: mlTrainingFlagName,
											Usage: "registry name of the ML training script to use for training, " +
												"which should be formatted as prefix:itemname where prefix is either the org ID or the namespace.",
											Required: true,
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagVersion,
											Usage:    "version of the ML training script to use for training.",
											Required: true,
										},
									},
									Action: MLSubmitCustomTrainingJob,
								},
								{
									Name:  "with-upload",
									Usage: "submits custom training job with an upload training script on data in Viam cloud",
									UsageText: createUsageText("train submit custom with-upload",
										[]string{
											datasetFlagDatasetID, generalFlagOrgID, trainFlagModelName,
											mlTrainingFlagPath, mlTrainingFlagName,
										}, true),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     datasetFlagDatasetID,
											Usage:    "dataset ID",
											Required: true,
										},
										&cli.StringFlag{
											Name:     trainFlagModelName,
											Usage:    "name of ML model",
											Required: true,
										},
										&cli.StringFlag{
											Name:  trainFlagModelVersion,
											Usage: "version of ML model. defaults to current timestamp if unspecified.",
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagPath,
											Usage:    "path to ML training scripts for upload",
											Required: true,
										},
										&cli.StringFlag{
											Name:     trainFlagModelOrgID,
											Required: true,
											Usage:    "organization ID to upload and run training job",
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagName,
											Usage:    "script name of the ML training script to upload",
											Required: true,
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagVersion,
											Usage:    "version of the ML training script to upload. defaults to current timestamp if unspecified.",
											Required: false,
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagFramework,
											Usage:    "framework of the ML training script to upload, can be: " + strings.Join(modelFrameworks, ", "),
											Required: false,
										},
										&cli.StringFlag{
											Name:     trainFlagModelType,
											Usage:    "task type of the ML training script to upload, can be: " + strings.Join(modelTypes, ", "),
											Required: false,
										},
									},
									Action: MLSubmitCustomTrainingJobWithUpload,
								},
							},
						},
					},
				},
				{
					Name:      "get",
					Usage:     "gets training job from Viam cloud based on training job ID",
					UsageText: createUsageText("train get", []string{trainFlagJobID}, false),
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
					Name:      "logs",
					Usage:     "gets training job logs from Viam cloud based on training job ID",
					UsageText: createUsageText("train logs", []string{trainFlagJobID}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: MLGetTrainingJobLogs,
				},
				{
					Name:      "cancel",
					Usage:     "cancels training job in Viam cloud based on training job ID",
					UsageText: createUsageText("train cancel", []string{trainFlagJobID}, false),
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
					UsageText: createUsageText("train list", []string{generalFlagOrgID}, true),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Usage:    "org ID",
							Required: true,
						},
						&cli.StringFlag{
							Name:     trainFlagJobStatus,
							Usage:    "training status to filter for. can be one of " + allTrainingStatusValues(),
							Required: false,
							Value:    defaultTrainingStatus(),
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
							Name:      "create",
							Usage:     "create an api-key for your machine",
							UsageText: createUsageText("machines api-key create", []string{generalFlagMachineID}, true),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagMachineID,
										Aliases:  []string{generalFlagAliasRobotID},
										Required: true,
										Usage:    "the machine to create an api-key for",
									},
								},
								&cli.StringFlag{
									Name:  apiKeyCreateFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name: generalFlagOrgID,
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
					UsageText: createUsageText("machines status", []string{machineFlag}, true),
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
					UsageText: createUsageText("machines logs", []string{machineFlag}, true),
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
						&cli.IntFlag{
							Name:        logsFlagCount,
							Usage:       fmt.Sprintf("number of logs to fetch (max %v)", maxNumLogs),
							DefaultText: fmt.Sprintf("%v", defaultNumLogs),
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
							UsageText: createUsageText("machines part status", []string{machineFlag, partFlag}, true),
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
							UsageText: createUsageText("machines part logs", []string{machineFlag, partFlag}, true),
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
								&cli.IntFlag{
									Name:        logsFlagCount,
									Usage:       fmt.Sprintf("number of logs to fetch (max %v)", maxNumLogs),
									DefaultText: fmt.Sprintf("%v", defaultNumLogs),
								},
							},
							Action: RobotsPartLogsAction,
						},
						{
							Name:      "restart",
							Aliases:   []string{},
							Usage:     "request part restart",
							UsageText: createUsageText("machines part restart", []string{machineFlag, partFlag}, true),
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
							Action: RobotsPartRestartAction,
						},
						{
							Name:  "run",
							Usage: "run a command on a machine part",
							UsageText: createUsageText("machines part run", []string{
								organizationFlag, locationFlag, machineFlag, partFlag,
							}, true, "<service.method>"),
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
							UsageText:   createUsageText("machines part shell", []string{organizationFlag, locationFlag, machineFlag, partFlag}, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name: organizationFlag,
								},
								&cli.StringFlag{
									Name: locationFlag,
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    machineFlag,
										Aliases: []string{aliasRobotFlag},
									},
								},
								&cli.StringFlag{
									Name: partFlag,
								},
							},
							Action: RobotsPartShellAction,
						},
						{
							Name:  "cp",
							Usage: "copy files to and from a machine part",

							Description: `
In order to use the cp command, the machine must have a valid shell type service.
Specifying ~ or a blank destination for the machine will use the home directory of the user
that is running the process (this may sometimes be root). Organization and location are
required flags if the machine/part name are not unique across your account.
Note: There is no progress meter while copying is in progress.

Copy a single file to the machine with a new name:
'viam machine part cp --organization "org" --location "location" --machine "m1" --part "m1-main" my_file machine:/home/user/'

Recursively copy a directory to the machine with the same name:
'viam machine part cp --machine "m1" --part "m1-main" -r my_dir machine:/home/user/'

Copy multiple files to the machine with recursion and keep original permissions and metadata:
'viam machine part cp --machine "m1" --part "m1-main" -r -p my_dir my_file machine:/home/user/some/existing/dir/'

Copy a single file from the machine to a local destination:
'viam machine part cp --machine "m1" --part "m1-main" machine:my_file ~/Downloads/'

Recursively copy a directory from the machine to a local destination with the same name:
'viam machine part cp --machine "m1" --part "m1-main" -r machine:my_dir ~/Downloads/'

Copy multiple files from the machine to a local destination with recursion and keep original permissions and metadata:
'viam machine part cp --machine "m1" --part "m1-main" -r -p machine:my_dir machine:my_file ~/some/existing/dir/'
`,
							UsageText: createUsageText(
								"machines part cp",
								[]string{organizationFlag, locationFlag, machineFlag, partFlag},
								true,
								"[-p] [-r] source ([machine:]files) ... target ([machine:]files"),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name: organizationFlag,
								},
								&cli.StringFlag{
									Name: locationFlag,
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
									Name:    cpFlagRecursive,
									Aliases: []string{"r"},
									Usage:   "recursively copy files",
								},
								&cli.BoolFlag{
									Name:    cpFlagPreserve,
									Aliases: []string{"p"},
									// Note(erd): maybe support access time in the future if needed
									Usage: "preserve modification times and file mode bits from the source files",
								},
							},
							Action: MachinesPartCopyFilesAction,
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
					UsageText: createUsageText("module create", []string{moduleFlagName}, true),
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
							Name:  generalFlagOrgID,
							Usage: "id of the organization that will host the module",
						},
						&cli.BoolFlag{
							Name:  moduleCreateLocalOnly,
							Usage: "create a meta.json file for local use, but don't create the module on the backend",
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
					Name:      "update-models",
					Usage:     "update a module's metadata file based on models it provides",
					UsageText: createUsageText("module update-models", []string{moduleFlagBinary}, true),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:      moduleFlagPath,
							Usage:     "path to meta.json",
							Value:     "./meta.json",
							TakesFile: true,
						},
						&cli.StringFlag{
							Name:     moduleFlagBinary,
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
					UsageText: createUsageText("module upload", []string{moduleFlagVersion, moduleFlagPlatform}, true, "<packaged-module.tag.gz>"),
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
							Name:  generalFlagOrgID,
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
					Name:  "build",
					Usage: "build your module for different architectures using cloud runners",
					UsageText: `Build your module on different operating systems and cpu architectures via cloud runners.
Make sure to add a "build" section to your meta.json.
Example:
{
  "module_id": ...,
  "build": {
    "setup": "./setup.sh",                  // optional - command to install your build dependencies
    "build": "make module.tar.gz",          // command that will build your module
    "path" : "module.tar.gz",               // optional - path to your built module
                                            // (passed to the 'viam module upload' command)
    "arch" : ["linux/amd64", "linux/arm64"] // architectures to build for
  }
}
`,
					Subcommands: []*cli.Command{
						{
							Name:  "local",
							Usage: "run your meta.json build command locally",
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
							Name:      "start",
							Usage:     "start a remote build",
							UsageText: createUsageText("module build start", []string{moduleBuildFlagVersion}, true),
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
							Name:      "logs",
							Aliases:   []string{"log"},
							Usage:     "get the logs from one of your cloud builds",
							UsageText: createUsageText("module build logs", []string{moduleBuildFlagBuildID}, true),
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
								&cli.BoolFlag{
									Name:  moduleBuildFlagGroupLogs,
									Usage: "write ::group:: commands so github action logs collapse",
								},
							},
							Action: ModuleBuildLogsAction,
						},
						{
							Name:  "link-repo",
							Usage: "link a GitHub repository to your module",
							//nolint:lll
							UsageText: `This command connects a Viam module to a GitHub repository so that repo actions can trigger builds and releases of your module.

This won't work unless you have an existing installation of our GitHub app on your GitHub org. (Details to follow).
`,
							// TODO(APP-3604): unhide when this is shipped externally
							Hidden: true,
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:  moduleBuildFlagOAuthLink,
									Usage: "ID of the oauth link between your GitHub org and Viam. Only required if you have more than one link",
								},
								&cli.StringFlag{
									Name: moduleFlagPath,
									Usage: "your module's ID in org-id:name or public-namespace:name format. " +
										"If missing, we will try to get this from meta.json file in current directory",
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagRepo,
									Usage: "your github repository in account/repository form (e.g. viamrobotics/rdk, not github.com/viamrobotics/rdk)",
								},
							},
							Action: ModuleBuildLinkRepoAction,
						},
					},
				},
				{
					Name:      "reload",
					Usage:     "build a module locally and run it on a target device. rebuild & restart if already running",
					UsageText: createUsageText("module reload", []string{}, true),
					Description: `Example invocations:

	# A full reload command. This will build your module, send the tarball to the machine with given part ID,
	# and configure or restart it.
	# The GOARCH env in this case would get passed to an underlying go build (assuming you're targeting an arm device).
	# Note that you'll still need to add the components for your models after your module is installed.
	GOARCH=arm64 viam module reload --part UUID

	# Restart a module running on your local viam server, by name, without building or reconfiguring.
	viam module reload --restart-only --id viam:python-example-module

	# Build and configure a module on your local machine without shipping a tarball.
	viam module reload --local`,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  partFlag,
							Usage: "part ID of machine. get from 'Live/Offline' dropdown in the web app, or leave it blank to use /etc/viam.json",
						},
						&cli.StringFlag{
							Name:  moduleFlagPath,
							Usage: "path to a meta.json. used for module ID. can be overridden with --id or --name",
							Value: "meta.json",
						},
						&cli.StringFlag{
							Name:  moduleFlagName,
							Usage: "name of module to restart. pass at most one of --name, --id",
						},
						&cli.StringFlag{
							Name:  moduleBuildFlagBuildID,
							Usage: "ID of module to restart, for example viam:wifi-sensor",
						},
						&cli.BoolFlag{
							Name:  moduleBuildRestartOnly,
							Usage: "just restart the module on the target system, don't do other reload steps",
						},
						&cli.BoolFlag{
							Name:  moduleBuildFlagNoBuild,
							Usage: "don't do build step",
						},
						&cli.BoolFlag{
							Name:  moduleFlagLocal,
							Usage: "if the target machine is localhost, run the entrypoint directly rather than transferring a bundle",
						},
						&cli.StringFlag{
							Name:  moduleFlagHomeDir,
							Usage: "remote user's home directory. only necessary if you're targeting a remote machine where $HOME is not /root",
							Value: "/root",
						},
					},
					Action: ReloadModuleAction,
				},
				{
					Name:      "download",
					Usage:     "download a module package from the registry",
					UsageText: createUsageText("module download", []string{}, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:  packageFlagDestination,
							Usage: "output directory for downloaded package",
							Value: ".",
						},
						&cli.StringFlag{
							Name:  moduleFlagID,
							Usage: "module ID as org-id:name or namespace:name. if missing, will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:  packageFlagVersion,
							Usage: "version of the requested package, can be `latest` to get the most recent version",
							Value: "latest",
						},
						&cli.StringFlag{
							Name:  moduleFlagPlatform,
							Usage: "platform like 'linux/amd64'. if missing, will use platform of the CLI binary",
						},
					},
					Action: DownloadModuleAction,
				},
			},
		},
		{
			Name:            "packages",
			Usage:           "work with packages",
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "export",
					Usage: "download a package from Viam cloud",
					UsageText: createUsageText("packages export",
						[]string{packageFlagType}, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:  packageFlagDestination,
							Usage: "output directory for downloaded package",
							Value: ".",
						},
						&cli.StringFlag{
							Name:  generalFlagOrgID,
							Usage: "organization ID or namespace of the requested package. if missing, will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:  packageFlagName,
							Usage: "name of the requested package. if missing, will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:  packageFlagVersion,
							Usage: "version of the requested package, can be `latest` to get the most recent version",
							Value: "latest",
						},
						&cli.StringFlag{
							Name:     packageFlagType,
							Required: true,
							Usage:    "type of the requested package, can be: " + strings.Join(packageTypes, ", "),
						},
					},
					Action: PackageExportAction,
				},
				{
					Name:  "upload",
					Usage: "upload a package to Viam cloud",
					UsageText: createUsageText("packages upload",
						[]string{
							packageFlagPath, generalFlagOrgID, packageFlagName,
							packageFlagVersion, packageFlagType,
						}, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:     packageFlagPath,
							Required: true,
							Usage:    "path to package for upload",
						},
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID of the requested package",
						},
						&cli.StringFlag{
							Name:     packageFlagName,
							Required: true,
							Usage:    "name of the requested package",
						},
						&cli.StringFlag{
							Name:     packageFlagVersion,
							Required: true,
							Usage:    "version of the requested package, can be `latest` to get the most recent version",
						},
						&cli.StringFlag{
							Name:     packageFlagType,
							Required: true,
							Usage:    "type of the requested package, can be: " + strings.Join(packageTypes, ", "),
						},
					},
					Action: PackageUploadAction,
				},
			},
		},
		{
			Name:  "training-script",
			Usage: "manage training scripts for custom ML training",
			Subcommands: []*cli.Command{
				{
					Name:      "upload",
					Usage:     "upload ML training scripts for custom ML training",
					UsageText: createUsageText("training-script upload", []string{mlTrainingFlagPath, mlTrainingFlagName}, true),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     mlTrainingFlagPath,
							Usage:    "path to ML training scripts for upload",
							Required: true,
						},
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID that will host the scripts",
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagName,
							Usage:    "name of the ML training script to upload",
							Required: true,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagVersion,
							Usage:    "version of the ML training script to upload",
							Required: false,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagFramework,
							Usage:    "framework of the ML training script to upload, can be: " + strings.Join(modelFrameworks, ", "),
							Required: false,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagType,
							Usage:    "task type of the ML training script to upload, can be: " + strings.Join(modelTypes, ", "),
							Required: false,
						},
						&cli.BoolFlag{
							Name:     mlTrainingFlagDraft,
							Usage:    "indicate draft mode, drafts will not be viewable in the registry",
							Required: false,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagURL,
							Usage:    "url of Github repository associated with the training scripts",
							Required: false,
						},
					},
					// Upload action
					Action: MLTrainingUploadAction,
				},
				{
					Name:      "update",
					Usage:     "update ML training scripts for custom ML training",
					UsageText: createUsageText("training-script update", []string{generalFlagOrgID, mlTrainingFlagName, mlTrainingFlagVisibility}, true),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID that hosts the scripts",
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagName,
							Usage:    "name of the ML training script to update",
							Required: true,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagVisibility,
							Usage:    "visibility of the registry item, can be: `public` or `private`",
							Required: true,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagDescription,
							Usage:    "description of the ML training script",
							Required: false,
						},
						&cli.StringFlag{
							Name:     mlTrainingFlagURL,
							Usage:    "url of Github repository associated with the training scripts",
							Required: false,
						},
					},
					Action: MLTrainingUpdateAction,
				},
			},
		},
		{
			Name:  "auth-app",
			Usage: "manage third party auth applications",
			Subcommands: []*cli.Command{
				{
					Name:  "register",
					Usage: "register a third party auth application",
					UsageText: createUsageText("auth-app register", []string{
						generalFlagOrgID,
						authApplicationFlagName, authApplicationFlagOriginURIs, authApplicationFlagRedirectURIs,
						authApplicationFlagLogoutURI,
					}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Usage:    "organization ID that will be tied to auth application",
							Required: true,
						},
						&cli.StringFlag{
							Name:     authApplicationFlagName,
							Usage:    "name for the auth application",
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     authApplicationFlagOriginURIs,
							Usage:    "origin uris for the auth application",
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     authApplicationFlagRedirectURIs,
							Usage:    "redirect uris for the auth application",
							Required: true,
						},
						&cli.StringFlag{
							Name:     authApplicationFlagLogoutURI,
							Usage:    "logout uri for the auth application",
							Required: true,
						},
					},
					Action: RegisterAuthApplicationAction,
				},
				{
					Name:  "update",
					Usage: "update a third party auth application",
					UsageText: createUsageText("auth-app update", []string{
						generalFlagOrgID,
						authApplicationFlagApplicationID, authApplicationFlagName, authApplicationFlagOriginURIs,
						authApplicationFlagRedirectURIs, authApplicationFlagLogoutURI,
					}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID that will be tied to auth application",
						},
						&cli.StringFlag{
							Name:     authApplicationFlagApplicationID,
							Usage:    "id for the auth application",
							Required: true,
						},
						&cli.StringFlag{
							Name:     authApplicationFlagName,
							Usage:    "updated name for the auth application",
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     authApplicationFlagOriginURIs,
							Usage:    "updated origin uris for the auth application",
							Required: false,
						},
						&cli.StringSliceFlag{
							Name:     authApplicationFlagRedirectURIs,
							Usage:    "updated redirect uris for the auth application",
							Required: false,
						},
						&cli.StringFlag{
							Name:     authApplicationFlagLogoutURI,
							Usage:    "updated logout uri for the auth application",
							Required: false,
						},
					},
					Action: UpdateAuthApplicationAction,
				},
				{
					Name:  "get",
					Usage: "get configuration for a third party auth application",
					UsageText: createUsageText("auth-app get", []string{
						generalFlagOrgID,
						authApplicationFlagApplicationID,
					}, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID that will be tied to auth application",
						},
						&cli.StringFlag{
							Name:     authApplicationFlagApplicationID,
							Usage:    "id for the auth application",
							Required: true,
						},
					},
					Action: GetAuthApplicationAction,
				},
			},
		},
		{
			Name:   "version",
			Usage:  "print version info for this program",
			Action: VersionAction,
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
