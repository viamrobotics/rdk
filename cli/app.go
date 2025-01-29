package cli

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

// CLI flags.
const (
	baseURLFlag         = "base-url"
	configFlag          = "config"
	debugFlag           = "debug"
	profileFlag         = "profile"
	disableProfilesFlag = "disable-profiles"
	profileFlagName     = "profile-name"

	// TODO: RSDK-6683.
	quietFlag = "quiet"

	logsFlagFormat     = "format"
	logsFlagOutputFile = "output"
	logsFlagKeyword    = "keyword"
	logsFlagLevels     = "levels"
	logsFlagErrors     = "errors"
	logsFlagTail       = "tail"

	runFlagData   = "data"
	runFlagStream = "stream"

	loginFlagDisableBrowser = "disable-browser-open"
	loginFlagKeyID          = "key-id"
	loginFlagKey            = "key"

	// Flags shared by logs, api-key, module, ml-training, package, and data subcommands.
	generalFlagOrganization      = "organization"
	generalFlagAliasOrg          = "org"
	generalFlagAliasOrgName      = "org-name"
	generalFlagOrgID             = "org-id"
	generalFlagLocation          = "location"
	generalFlagAliasLocationName = "location-name"
	generalFlagLocationID        = "location-id"
	generalFlagMachine           = "machine"
	generalFlagMachineName       = "machine-name"
	generalFlagMachineID         = "machine-id"
	generalFlagAliasRobot        = "robot"
	generalFlagAliasRobotID      = "robot-id"
	generalFlagPart              = "part"
	generalFlagPartName          = "part-name"
	generalFlagPartID            = "part-id"
	generalFlagName              = "name"
	generalFlagMethod            = "method"
	generalFlagDestination       = "destination"
	generalFlagVersion           = "version"
	generalFlagCount             = "count"
	generalFlagPath              = "path"
	generalFlagType              = "type"
	generalFlagResourceSubtype   = "resource-subtype"
	generalFlagTags              = "tags"
	generalFlagStart             = "start"
	generalFlagEnd               = "end"

	moduleFlagLanguage        = "language"
	moduleFlagPublicNamespace = "public-namespace"
	moduleFlagPath            = "module"
	moduleFlagPlatform        = "platform"
	moduleFlagForce           = "force"
	moduleFlagBinary          = "binary"
	moduleFlagLocal           = "local"
	moduleFlagHomeDir         = "home"
	moduleCreateLocalOnly     = "local-only"
	moduleFlagID              = "id"
	moduleFlagIsPublic        = "public"
	moduleFlagResourceType    = "resource-type"
	moduleFlagModelName       = "model-name"
	moduleFlagEnableCloud     = "enable-cloud"
	moduleFlagRegister        = "register"
	moduleFlagDryRun          = "dry-run"
	moduleFlagUpload          = "upload"

	moduleBuildFlagRef       = "ref"
	moduleBuildFlagWait      = "wait"
	moduleBuildFlagToken     = "token"
	moduleBuildFlagWorkdir   = "workdir"
	moduleBuildFlagPlatforms = "platforms"
	moduleBuildFlagGroupLogs = "group-logs"
	moduleBuildRestartOnly   = "restart-only"
	moduleBuildFlagNoBuild   = "no-build"
	moduleBuildFlagOAuthLink = "oauth-link"
	moduleBuildFlagRepo      = "repo"

	mlTrainingFlagName        = "script-name"
	mlTrainingFlagFramework   = "framework"
	mlTrainingFlagDraft       = "draft"
	mlTrainingFlagVisibility  = "visibility"
	mlTrainingFlagDescription = "description"
	mlTrainingFlagURL         = "url"
	mlTrainingFlagArgs        = "args"

	dataFlagDataType                       = "data-type"
	dataFlagOrgIDs                         = "org-ids"
	dataFlagLocationIDs                    = "location-ids"
	dataFlagAliasRobotName                 = "robot-name"
	dataFlagComponentType                  = "component-type"
	dataFlagComponentName                  = "component-name"
	dataFlagResourceName                   = "resource-name"
	dataFlagMimeTypes                      = "mime-types"
	dataFlagParallelDownloads              = "parallel"
	dataFlagBboxLabels                     = "bbox-labels"
	dataFlagDeleteTabularDataOlderThanDays = "delete-older-than-days"
	dataFlagDatabasePassword               = "password"
	dataFlagFilterTags                     = "filter-tags"
	dataFlagTimeout                        = "timeout"

	packageFlagFramework = "model-framework"

	oauthAppFlagClientID             = "client-id"
	oauthAppFlagClientName           = "client-name"
	oauthAppFlagClientAuthentication = "client-authentication"
	oauthAppFlagPKCE                 = "pkce"
	oauthAppFlagEnabledGrants        = "enabled-grants"
	oauthAppFlagURLValidation        = "url-validation"
	oauthAppFlagOriginURIs           = "origin-uris"
	oauthAppFlagRedirectURIs         = "redirect-uris"
	oauthAppFlagLogoutURI            = "logout-uri"
	unspecified                      = "unspecified"

	cpFlagRecursive = "recursive"
	cpFlagPreserve  = "preserve"

	organizationFlagSupportEmail = "support-email"
	organizationBillingAddress   = "address"
	organizationFlagLogoPath     = "logo-path"
)

// matches all uppercase characters that follow lowercase chars and aren't at the [0] index of a string.
// This is useful for converting camel case into kabob case when getting values out of a CLI Context
// based on a flag name, and putting them into a struct with a camel cased field name.
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

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
		Name:  generalFlagPartID,
		Usage: "part id filter",
	},
	&AliasStringFlag{
		cli.StringFlag{
			Name:    generalFlagMachineName,
			Aliases: []string{dataFlagAliasRobotName},
			Usage:   "machine name filter",
		},
	},
	&cli.StringFlag{
		Name:  generalFlagPartName,
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
		Name:  generalFlagMethod,
		Usage: "method filter",
	},
	&cli.StringSliceFlag{
		Name:  dataFlagMimeTypes,
		Usage: "mime types filter",
	},
	&cli.StringFlag{
		Name:  generalFlagStart,
		Usage: "ISO-8601 timestamp in RFC3339 format indicating the start of the interval filter",
	},
	&cli.StringFlag{
		Name:  generalFlagEnd,
		Usage: "ISO-8601 timestamp in RFC3339 format indicating the end of the interval filter",
	},
	&cli.StringSliceFlag{
		Name:  dataFlagBboxLabels,
		Usage: "bbox labels filter. accepts string labels corresponding to bounding boxes within images",
	},
}

var dataTagByIDsFlags = []cli.Flag{
	&cli.StringSliceFlag{
		Name:     generalFlagTags,
		Usage:    "comma separated tags to add/remove to the data",
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
		Usage:    "comma separated file IDs of data belonging to specified org and location",
		Required: true,
	},
}

var dataTagByFilterFlags = append([]cli.Flag{
	&cli.StringSliceFlag{
		Name:     generalFlagTags,
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

type emptyArgs struct{}

type globalArgs struct {
	BaseURL         string
	Config          string
	Debug           bool
	Quiet           bool
	Profile         string
	DisableProfiles bool
}

func getValFromContext(name string, ctx *cli.Context) any {
	// some fuzzy searching is required here, because flags are typically in kebab case, but
	// params are typically in snake or camel case
	replacer := strings.NewReplacer("_", "-")
	dashFormattedName := replacer.Replace(strings.ToLower(name))

	value := ctx.Value(dashFormattedName)
	if value != nil {
		return value
	}

	camelFormattedName := matchAllCap.ReplaceAllString(name, "${1}-${2}")
	camelFormattedName = strings.ToLower(camelFormattedName)

	return ctx.Value(camelFormattedName)
}

// (erodkin) We don't support pointers in structs here. The problem is that when getting a value
// from a context for a supported flag, the context will default to populating with the zero value.
// When getting a value from the context, though, we currently have no way of know if that's going
// to a concrete value, going to a pointer and should be a nil value, or going to a pointer but should
// be a pointer to that default value.
func parseStructFromCtx[T any](ctx *cli.Context) T {
	var t T
	var s cli.StringSlice
	s.Value()
	tValue := reflect.ValueOf(&t).Elem()
	tType := tValue.Type()
	for i := 0; i < tType.NumField(); i++ {
		field := tType.Field(i)
		if value := getValFromContext(field.Name, ctx); value != nil {
			reflectVal := reflect.ValueOf(&value)
			// (erodkin) Unfortunately, the value we get out of the context when dealing with a
			// slice is not, e.g., a `[]string`, but rather a `cli.StringSlice` that has a
			// `Value` method that returns a `[]string`. Some short attempts to use reflection
			// to access that `Value` method proved unproductive, so instead we match on all
			// currently existing `cli.FooSlice` types. This should be relatively stable
			// (currently we only use a `StringSlice` in the CLI), but in theory it would be
			// sad if urfave introduced a new slice type and someone tried to use it in our
			// CLI. The default warning message should hopefully provide some clarity if
			// such a case should ever arise.
			if field.Type.Kind() == reflect.Slice {
				switch v := value.(type) {
				case cli.StringSlice:
					tValue.Field(i).Set(reflect.ValueOf(v.Value()))
				case cli.IntSlice:
					tValue.Field(i).Set(reflect.ValueOf(v.Value()))
				case cli.Int64Slice:
					tValue.Field(i).Set(reflect.ValueOf(v.Value()))
				case cli.Float64Slice:
					tValue.Field(i).Set(reflect.ValueOf(v.Value()))
				default:
					warningf(ctx.App.Writer,
						"Attempted to set flag with unsupported slice type %s, this value may not be set correctly. consider filing a ticket to add support",
						reflectVal.Type().Name())
				}
			} else {
				tValue.Field(i).Set(reflect.ValueOf(value))
			}
		}
	}

	return t
}

func getGlobalArgs(ctx *cli.Context) (*globalArgs, error) {
	gArgs := parseStructFromCtx[globalArgs](ctx)
	// TODO(RSDK-9361) - currently nothing prevents a developer from creating globalArgs directly
	// and thereby bypassing this check. We should find a way to prevent direct creation and thereby
	// programmatically enforce compliance here.
	if gArgs.DisableProfiles && gArgs.Profile != "" {
		return nil, errors.New("profile specified with disable-profiles flag set")
	}

	return &gArgs, nil
}

func createCommandWithT[T any](f func(*cli.Context, T) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		t := parseStructFromCtx[T](ctx)
		return f(ctx, t)
	}
}

// createUsageText is a helper for formatting UsageTexts. The created UsageText
// contains "viam", the command, requiredFlags, "[other options]" if unrequiredOptions
// is true, "<command> [command options]" if subcommand is true, and all passed-in
// arguments in that order.
func createUsageText(command string, requiredFlags []string, unrequiredOptions, subcommand bool, arguments ...string) string {
	formatted := []string{"viam", command}
	for _, flag := range requiredFlags {
		formatted = append(formatted, fmt.Sprintf("--%s=<%s>", flag, flag))
	}
	if unrequiredOptions {
		if len(requiredFlags) == 0 {
			formatted = append(formatted, "[options]")
		} else {
			formatted = append(formatted, "[other options]")
		}
	}
	if subcommand {
		formatted = append(formatted, "<command> [command options]")
	}
	formatted = append(formatted, arguments...)
	return strings.Join(formatted, " ")
}

// formatAcceptedValues is a helper for formatting the usage text for flags that only accept certain values.
func formatAcceptedValues(description string, values ...string) string {
	joined := strings.Join(values, ", ")
	return fmt.Sprintf("%s. value(s) can be: [%s]", description, joined)
}

var app = &cli.App{
	Name:            "viam",
	Usage:           "interact with your Viam machines",
	UsageText:       "viam [global options] <command> [command options]",
	HideHelpCommand: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   baseURLFlag,
			Hidden: true,
			Usage:  "base URL of app",
		},
		// TODO(RSDK-9287) - this flag isn't used anywhere. Confirm that we actually need it,
		// get rid of it if we don't.
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
		&cli.StringFlag{
			Name:  profileFlag,
			Usage: "specify a particular profile for the current command",
		},
		&cli.BoolFlag{
			Name:    disableProfilesFlag,
			Aliases: []string{"disable-profile"}, // for ease of use; not backwards compatibility related
			Usage:   "disable usage of profiles, falling back to default behavior",
		},
	},
	Commands: []*cli.Command{
		{
			Name: "login",
			// NOTE(benjirewis): maintain `auth` as an alias for backward compatibility.
			Aliases:         []string{"auth"},
			Usage:           "login to app.viam.com",
			UsageText:       "viam login [options] [command] [command options]",
			HideHelpCommand: true,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    loginFlagDisableBrowser,
					Aliases: []string{"no-browser"}, // ease of use alias, not related to backwards compatibility
					Usage:   "prevent opening the default browser during login",
				},
			},
			Action: createCommandWithT[loginActionArgs](LoginAction),
			After:  createCommandWithT[emptyArgs](CheckUpdateAction),
			Subcommands: []*cli.Command{
				{
					Name:      "print-access-token",
					Usage:     "print the access token associated with current credentials",
					UsageText: createUsageText("login print-access-token", nil, false, false),
					Action:    createCommandWithT[emptyArgs](PrintAccessTokenAction),
				},
				{
					Name:      "api-key",
					Usage:     "authenticate with an api key",
					UsageText: createUsageText("login api-key", []string{loginFlagKeyID, loginFlagKey}, false, false),
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
					Action: createCommandWithT[loginWithAPIKeyArgs](LoginWithAPIKeyAction),
				},
			},
		},
		{
			Name:      "logout",
			Usage:     "logout from current session",
			UsageText: createUsageText("logout", nil, false, false),
			Action:    createCommandWithT[emptyArgs](LogoutAction),
		},
		{
			Name:      "whoami",
			Usage:     "get currently logged-in user",
			UsageText: createUsageText("whoami", nil, false, false),
			Action:    createCommandWithT[emptyArgs](WhoAmIAction),
		},
		{
			Name:            "organizations",
			Aliases:         []string{"organization", "org"},
			Usage:           "work with organizations",
			UsageText:       createUsageText("organizations", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:            "auth-service",
					Usage:           "manage auth-service",
					UsageText:       createUsageText("organizations auth-service", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "enable",
							Usage:     "enable auth-service for OAuth applications",
							UsageText: createUsageText("organizations auth-service enable", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "organization ID tied to OAuth applications",
								},
							},
							Action: createCommandWithT[enableAuthServiceArgs](EnableAuthServiceAction),
						},
						{
							Name:      "disable",
							Usage:     "disable auth-service for OAuth applications",
							UsageText: createUsageText("organizations auth-service disable", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "organization ID tied to OAuth applications",
								},
							},
							Before: createCommandWithT[disableAuthServiceArgs](DisableAuthServiceConfirmation),
							Action: createCommandWithT[disableAuthServiceArgs](DisableAuthServiceAction),
						},
						{
							Name:            "oauth-app",
							Usage:           "manage the OAuth applications for an organization",
							UsageText:       createUsageText("organizations auth-service oauth-app", nil, false, true),
							HideHelpCommand: true,
							Subcommands: []*cli.Command{
								{
									Name:  "delete",
									Usage: "delete an OAuth application",
									UsageText: createUsageText(
										"organizations auth-service oauth-app delete", []string{generalFlagOrgID, oauthAppFlagClientID}, false, false,
									),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Required: true,
											Usage:    "organization ID tied to the OAuth application",
										},
										&cli.StringFlag{
											Name:     oauthAppFlagClientID,
											Required: true,
											Usage:    "client ID of the OAuth application to delete",
										},
									},
									Before: createCommandWithT[deleteOAuthAppArgs](DeleteOAuthAppConfirmation),
									Action: createCommandWithT[deleteOAuthAppArgs](DeleteOAuthAppAction),
								},
								{
									Name:      "list",
									Usage:     "list oauth applications for an organization",
									UsageText: createUsageText("organizations auth-service oauth-app list", []string{generalFlagOrgID}, false, false),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Required: true,
											Usage:    "the org to get applications for",
										},
									},
									Action: createCommandWithT[listOAuthAppsArgs](ListOAuthAppsAction),
								},
								{
									Name:  "read",
									Usage: "read the OAuth configuration details",
									UsageText: createUsageText(
										"organizations auth-service oauth-app read", []string{generalFlagOrgID, oauthAppFlagClientID}, false, false,
									),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Required: true,
											Usage:    "organization ID that is tied to the OAuth application",
										},
										&cli.StringFlag{
											Name:     oauthAppFlagClientID,
											Usage:    "id for the OAuth application",
											Required: true,
										},
									},
									Action: createCommandWithT[readOAuthAppArgs](ReadOAuthAppAction),
								},
								{
									Name:  "update",
									Usage: "update an OAuth application",
									UsageText: createUsageText(
										"organizations auth-service oauth-app update", []string{generalFlagOrgID, oauthAppFlagClientID}, true, false,
									),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Required: true,
											Usage:    "organization ID that is tied to the OAuth application",
										},
										&cli.StringFlag{
											Name:     oauthAppFlagClientID,
											Usage:    "id for the OAuth application to be updated",
											Required: true,
										},
										&cli.StringFlag{
											Name:  oauthAppFlagClientName,
											Usage: "updated name for the OAuth application",
										},
										&cli.StringFlag{
											Name: oauthAppFlagClientAuthentication,
											Usage: formatAcceptedValues(
												"updated client authentication policy for the OAuth application",
												string(ClientAuthenticationUnspecified), string(ClientAuthenticationRequired),
												string(ClientAuthenticationNotRequired), string(ClientAuthenticationNotRequiredWhenUsingPKCE),
											),
											Value: unspecified,
										},
										&cli.StringFlag{
											Name: oauthAppFlagURLValidation,
											Usage: formatAcceptedValues(
												"updated url validation for the OAuth application",
												string(URLValidationUnspecified), string(URLValidationExactMatch), string(URLValidationAllowWildcards),
											),
											Value: unspecified,
										},
										&cli.StringFlag{
											Name: oauthAppFlagPKCE,
											Usage: formatAcceptedValues(
												"updated pkce for the OAuth application",
												string(PKCEUnspecified), string(PKCERequired), string(PKCENotRequired), string(PKCENotRequiredWhenUsingClientAuthentication),
											),
											Value: unspecified,
										},
										&cli.StringSliceFlag{
											Name:  oauthAppFlagOriginURIs,
											Usage: "updated comma-separated origin uris for the OAuth application",
										},
										&cli.StringSliceFlag{
											Name:  oauthAppFlagRedirectURIs,
											Usage: "updated comma separated redirect uris for the OAuth application",
										},
										&cli.StringFlag{
											Name:  oauthAppFlagLogoutURI,
											Usage: "updated logout uri for the OAuth application",
										},
										&cli.StringSliceFlag{
											Name: oauthAppFlagEnabledGrants,
											Usage: formatAcceptedValues(
												"updated comma separated enabled grants for the OAuth application",
												string(EnabledGrantUnspecified), string(EnabledGrantRefreshToken), string(EnabledGrantPassword),
												string(EnabledGrantImplicit), string(EnabledGrantDeviceCode), string(EnabledGrantAuthorizationCode),
											),
										},
									},
									Action: createCommandWithT[updateOAuthAppArgs](UpdateOAuthAppAction),
								},
								{
									Name:  "create",
									Usage: "create an OAuth application",
									UsageText: createUsageText("organizations auth-serice oauth-app create",
										[]string{
											generalFlagOrgID, oauthAppFlagClientAuthentication, oauthAppFlagURLValidation, oauthAppFlagPKCE,
											oauthAppFlagRedirectURIs, oauthAppFlagLogoutURI, oauthAppFlagEnabledGrants,
										},
										true, false),
									Flags: []cli.Flag{
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Usage:    "organization ID that is tied to the OAuth application",
											Required: true,
										},
										&cli.StringFlag{
											Name:  oauthAppFlagClientName,
											Usage: "name for the OAuth application",
										},
										&cli.StringFlag{
											Name: oauthAppFlagClientAuthentication,
											Usage: formatAcceptedValues(
												"client authentication policy for the OAuth application",
												string(ClientAuthenticationUnspecified), string(ClientAuthenticationRequired),
												string(ClientAuthenticationNotRequired), string(ClientAuthenticationNotRequiredWhenUsingPKCE),
											),
											Required: true,
										},
										&cli.StringFlag{
											Name: oauthAppFlagURLValidation,
											Usage: formatAcceptedValues(
												"url validation for the OAuth application",
												string(URLValidationUnspecified), string(URLValidationExactMatch), string(URLValidationAllowWildcards),
											),
											Required: true,
										},
										&cli.StringFlag{
											Name: oauthAppFlagPKCE,
											Usage: formatAcceptedValues(
												"pkce for the OAuth application",
												string(PKCEUnspecified), string(PKCERequired), string(PKCENotRequired), string(PKCENotRequiredWhenUsingClientAuthentication),
											),
											Required: true,
										},
										&cli.StringSliceFlag{
											Name:  oauthAppFlagOriginURIs,
											Usage: "comma-separated origin uris for the OAuth application",
										},
										&cli.StringSliceFlag{
											Name:     oauthAppFlagRedirectURIs,
											Usage:    "comma-separated redirect uris for the OAuth application, requires at least one.",
											Required: true,
										},
										&cli.StringFlag{
											Name:     oauthAppFlagLogoutURI,
											Usage:    "logout uri for the OAuth application",
											Required: true,
										},
										&cli.StringSliceFlag{
											Name: oauthAppFlagEnabledGrants,
											Usage: formatAcceptedValues(
												"comma-separated enabled grants for the OAuth application",
												string(EnabledGrantUnspecified), string(EnabledGrantRefreshToken), string(EnabledGrantPassword),
												string(EnabledGrantImplicit), string(EnabledGrantDeviceCode), string(EnabledGrantAuthorizationCode),
											),
											Required: true,
										},
									},
									Action: createCommandWithT[createOAuthAppArgs](CreateOAuthAppAction),
								},
							},
						},
					},
				},
				{
					Name:      "list",
					Usage:     "list organizations for the current user",
					UsageText: createUsageText("organizations list", nil, false, false),
					Action:    createCommandWithT[emptyArgs](ListOrganizationsAction),
				},
				{
					Name:            "logo",
					Usage:           "manage the logo for an organization",
					UsageText:       createUsageText("organizations logo", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "set",
							Usage:     "set the logo for an organization from a local file",
							UsageText: createUsageText("organizations logo set", []string{generalFlagOrgID, organizationFlagLogoPath}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to set the logo for",
								},
								&cli.StringFlag{
									Name:     organizationFlagLogoPath,
									Required: true,
									Usage:    "the file path of the logo to set for the organization. This must be a png file.",
								},
							},
							Action: createCommandWithT[organizationsLogoSetArgs](OrganizationLogoSetAction),
						},
						{
							Name:      "get",
							Usage:     "get the logo for an organization",
							UsageText: createUsageText("organizations logo get", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to get the logo for",
								},
							},
							Action: createCommandWithT[organizationsLogoGetArgs](OrganizationsLogoGetAction),
						},
					},
				},
				{
					Name:            "support-email",
					Usage:           "manage the support email for an organization",
					UsageText:       createUsageText("organizations support-email", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:  "set",
							Usage: "set the support email for an organization",
							UsageText: createUsageText(
								"organizations support-email set", []string{generalFlagOrgID, organizationFlagSupportEmail}, false, false,
							),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to set the support email for",
								},
								&cli.StringFlag{
									Name:     organizationFlagSupportEmail,
									Required: true,
									Usage:    "the support email to set for the organization",
								},
							},
							Action: createCommandWithT[organizationsSupportEmailSetArgs](OrganizationsSupportEmailSetAction),
						},
						{
							Name:      "get",
							Usage:     "get the support email for an organization",
							UsageText: createUsageText("organizations support-email get", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to get the support email for",
								},
							},
							Action: createCommandWithT[organizationsSupportEmailGetArgs](OrganizationsSupportEmailGetAction),
						},
					},
				},
				{
					Name:            "billing-service",
					Usage:           "manage the organizations billing service",
					UsageText:       createUsageText("organizations billing-service", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "get-config",
							Usage:     "get the billing service config for an organization",
							UsageText: createUsageText("organizations billing-service get-config", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to get the billing config for",
								},
							},
							Action: createCommandWithT[getBillingConfigArgs](GetBillingConfigAction),
						},
						{
							Name:      "disable",
							Usage:     "disable the billing service for an organization",
							UsageText: createUsageText("organizations billing-service disable", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to disable the billing service for",
								},
							},
							Action: createCommandWithT[organizationDisableBillingServiceArgs](OrganizationDisableBillingServiceAction),
						},
						{
							Name:  "update",
							Usage: "update the billing service update for an organization",
							UsageText: createUsageText(
								"organizations billing-service update", []string{generalFlagOrgID, organizationBillingAddress}, false, false,
							),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to update the billing service for",
								},
								&cli.StringFlag{
									Name:     organizationBillingAddress,
									Required: true,
									Usage:    "the stringified address that follows the pattern: line1, line2 (optional), city, state, zipcode",
								},
							},
							Action: createCommandWithT[updateBillingServiceArgs](UpdateBillingServiceAction),
						},
						{
							Name:  "enable",
							Usage: "enable the billing service for an organization",
							UsageText: createUsageText(
								"organizations billing-service enable", []string{generalFlagOrgID, organizationBillingAddress}, false, false,
							),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to enable the billing service for",
								},
								&cli.StringFlag{
									Name:     organizationBillingAddress,
									Required: true,
									Usage:    "the stringified address that follows the pattern: line1, line2 (optional), city, state, zipcode",
								},
							},
							Action: createCommandWithT[organizationEnableBillingServiceArgs](OrganizationEnableBillingServiceAction),
						},
					},
				},
				{
					Name:            "api-key",
					Usage:           "work with an organization's api keys",
					UsageText:       createUsageText("organizations api-key", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "create",
							Usage:     "create an api key for your organization",
							UsageText: createUsageText("organizations api-key create", []string{generalFlagOrgID}, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Required: true,
									Usage:    "the org to create an api key for",
								},
								&cli.StringFlag{
									Name:        generalFlagName,
									Usage:       "the name of the key",
									DefaultText: "login info with current time",
								},
							},
							Action: createCommandWithT[organizationsAPIKeyCreateArgs](OrganizationsAPIKeyCreateAction),
						},
					},
				},
			},
		},
		{
			Name:            "locations",
			Aliases:         []string{"location"},
			Usage:           "work with locations",
			UsageText:       createUsageText("locations", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "list locations for the current user",
					// use custom usage text to show default organization flag usage even if it isn't required
					UsageText: "viam locations list [--organization=<organization>]",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        generalFlagOrganization,
							Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
							DefaultText: "first organization alphabetically",
						},
					},
					Action: createCommandWithT[listLocationsArgs](ListLocationsAction),
				},
				{
					Name:      "api-key",
					Usage:     "work with an api-key for your location",
					UsageText: createUsageText("locations api-key", nil, false, true),
					Subcommands: []*cli.Command{
						{
							Name:      "create",
							Usage:     "create an api key for your location",
							UsageText: createUsageText("locations api-key create", []string{generalFlagLocationID}, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagLocationID,
									Required: true,
									Usage:    "id of the location to create an api-key for",
								},
								&cli.StringFlag{
									Name:  generalFlagName,
									Usage: "the name of the key (defaults to your login info with the current time)",
								},
								&cli.StringFlag{
									Name:        generalFlagOrgID,
									Usage:       "the org-id to attach the key to",
									DefaultText: "will attempt to attach key to the org of the location if only one org is attached to the location",
								},
							},
							Action: createCommandWithT[locationAPIKeyCreateArgs](LocationAPIKeyCreateAction),
						},
					},
				},
			},
		},
		{
			Name:            "profiles",
			Usage:           "work with CLI profiles",
			UsageText:       createUsageText("profiles", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "update",
					Usage:     "update an existing profile for authentication, or add it if it doesn't exist",
					UsageText: createUsageText("profiles update", []string{profileFlagName, loginFlagKeyID, loginFlagKey}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     profileFlagName,
							Required: true,
							Usage:    "name of the profile to update",
						},
						&cli.StringFlag{
							Name:     loginFlagKeyID,
							Required: true,
							Usage:    "id of the profile's API key",
						},
						&cli.StringFlag{
							Name:     loginFlagKey,
							Required: true,
							Usage:    "the profile's API key",
						},
					},
					Action: createCommandWithT[addOrUpdateProfileArgs](UpdateProfileAction),
				},
				{
					Name:      "add",
					Usage:     "add a new profile for authentication (errors if the profile already exists)",
					UsageText: createUsageText("profiles add", []string{profileFlagName, loginFlagKeyID, loginFlagKey}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     profileFlagName,
							Required: true,
							Usage:    "name of the profile to add",
						},
						&cli.StringFlag{
							Name:     loginFlagKeyID,
							Required: true,
							Usage:    "id of the profile's API key",
						},
						&cli.StringFlag{
							Name:     loginFlagKey,
							Required: true,
							Usage:    "the profile's API key",
						},
					},
					Action: createCommandWithT[addOrUpdateProfileArgs](AddProfileAction),
				},
				{
					Name:      "list",
					Usage:     "list all existing profiles by name",
					UsageText: createUsageText("profiles list", nil, false, false),
					Action:    createCommandWithT[emptyArgs](ListProfilesAction),
				},
				{
					Name:      "remove",
					Usage:     "remove an authentication profile",
					UsageText: createUsageText("profiles remove", []string{profileFlagName}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     profileFlagName,
							Required: true,
							Usage:    "name of the profile to remove",
						},
					},
					Action: createCommandWithT[removeProfileArgs](RemoveProfileAction),
				},
			},
		},
		{
			Name:            "data",
			Usage:           "work with data",
			UsageText:       createUsageText("data", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:            "export",
					Usage:           "download data from Viam cloud",
					UsageText:       createUsageText("data export", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "binary",
							Usage:     "download binary data",
							UsageText: createUsageText("data export binary", []string{generalFlagDestination}, true, false),
							Flags: append([]cli.Flag{
								&cli.PathFlag{
									Name:     generalFlagDestination,
									Required: true,
									Usage:    "output directory for downloaded data",
								},
								&cli.UintFlag{
									Name:  dataFlagParallelDownloads,
									Usage: "number of download requests to make in parallel",
									Value: 100,
								},
								&cli.UintFlag{
									Name:  dataFlagTimeout,
									Usage: "number of seconds to wait for large file downloads",
									Value: 30,
								},
								&cli.StringSliceFlag{
									Name:  generalFlagTags,
									Usage: "tags filter. accepts tagged for all tagged data, untagged for all untagged data, or a list of tags",
								},
							}, commonFilterFlags...),
							Action: createCommandWithT[dataExportBinaryArgs](DataExportBinaryAction),
						},
						{
							Name:  "tabular",
							Usage: "download tabular data",
							UsageText: createUsageText("data export tabular", []string{
								generalFlagDestination,
								generalFlagPartID,
								dataFlagResourceName,
								generalFlagResourceSubtype,
								generalFlagMethod,
							}, true, false),
							Flags: []cli.Flag{
								&cli.PathFlag{
									Name:     generalFlagDestination,
									Required: true,
									Usage:    "output directory for downloaded data",
								},
								&cli.StringFlag{
									Name:     generalFlagPartID,
									Required: true,
									Usage:    "part id",
								},
								&cli.StringFlag{
									Name:     dataFlagResourceName,
									Required: true,
									Usage:    "resource name (sometimes called 'component name')",
								},
								&cli.StringFlag{
									Name:     generalFlagResourceSubtype,
									Required: true,
									Usage:    "resource subtype (sometimes called 'component type')",
								},
								&cli.StringFlag{
									Name:     generalFlagMethod,
									Required: true,
									Usage:    "method name",
								},
								&cli.StringFlag{
									Name:  "start",
									Usage: "ISO-8601 timestamp in RFC3339 format indicating the start of the interval",
								},
								&cli.StringFlag{
									Name:  "end",
									Usage: "ISO-8601 timestamp in RFC3339 format indicating the end of the interval",
								},
							},
							Action: createCommandWithT[dataExportTabularArgs](DataExportTabularAction),
						},
					},
				},
				{
					Name:            "delete",
					Usage:           "delete data from Viam cloud",
					UsageText:       createUsageText("data delete", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "binary",
							Usage:     "delete binary data from Viam cloud",
							UsageText: createUsageText("data delete binary", []string{dataFlagOrgIDs, generalFlagStart, generalFlagEnd}, true, false),
							Flags: []cli.Flag{
								&cli.StringSliceFlag{
									Name:     dataFlagOrgIDs,
									Required: true,
									Usage:    "orgs filter",
								},
								&cli.StringFlag{
									Name:     generalFlagStart,
									Required: true,
									Usage:    "ISO-8601 timestamp in RFC3339 format indicating the start of the interval filter",
								},
								&cli.StringFlag{
									Name:     generalFlagEnd,
									Required: true,
									Usage:    "ISO-8601 timestamp in RFC3339 format indicating the end of the interval filter",
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
									Name:  generalFlagPartID,
									Usage: "part id filter",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachineName,
										Aliases: []string{dataFlagAliasRobotName},
										Usage:   "machine name filter",
									},
								},
								&cli.StringFlag{
									Name:  generalFlagPartName,
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
									Name:  generalFlagMethod,
									Usage: "method filter",
								},
								&cli.StringSliceFlag{
									Name:  dataFlagMimeTypes,
									Usage: "mime types filter",
								},
								&cli.StringSliceFlag{
									Name: dataFlagBboxLabels,
									Usage: "bbox labels filter. " +
										"accepts string labels corresponding to bounding boxes within images",
								},
							},
							Action: createCommandWithT[emptyArgs](DataDeleteBinaryAction),
						},
						{
							Name:      "tabular",
							Usage:     "delete tabular data from Viam cloud",
							UsageText: createUsageText("data delete tabular", []string{generalFlagOrgID, dataFlagDeleteTabularDataOlderThanDays}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Usage:    "org id",
									Required: true,
								},
								&cli.IntFlag{
									Name:     dataFlagDeleteTabularDataOlderThanDays,
									Usage:    "delete any tabular data that is older than X calendar days before now. 0 deletes all data.",
									Required: true,
								},
							},
							Action: createCommandWithT[dataDeleteTabularArgs](DataDeleteTabularAction),
						},
					},
				},
				{
					Name:            "database",
					Usage:           "interact with a MongoDB Atlas Data Federation instance",
					UsageText:       createUsageText("data database", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "configure",
							Usage:     "configures a database user for the Viam org's MongoDB Atlas Data Federation instance",
							UsageText: createUsageText("data database configure", []string{generalFlagOrgID, dataFlagDatabasePassword}, false, false),
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
							Before: createCommandWithT[dataConfigureDatabaseUserArgs](DataConfigureDatabaseUserConfirmation),
							Action: createCommandWithT[dataConfigureDatabaseUserArgs](DataConfigureDatabaseUser),
						},
						{
							Name:      "hostname",
							Usage:     "gets the hostname to access a MongoDB Atlas Data Federation Instance",
							UsageText: createUsageText("data database hostname", []string{generalFlagOrgID}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagOrgID,
									Usage:    "org ID for the database user",
									Required: true,
								},
							},
							Action: createCommandWithT[dataGetDatabaseConnectionArgs](DataGetDatabaseConnection),
						},
					},
				},
				{
					Name:            "tag",
					Usage:           "tag binary data by filter or ids",
					UsageText:       createUsageText("data tag", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:            "ids",
							Usage:           "adds or removes tags from binary data by file ids for a given org and location",
							UsageText:       createUsageText("data tag ids", nil, true, false),
							HideHelpCommand: true,
							Subcommands: []*cli.Command{
								{
									Name:  "add",
									Usage: "adds tags to binary data by file ids for a given org and location",
									UsageText: createUsageText(
										"data tag ids add", []string{generalFlagTags, generalFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false, false,
									),
									Flags:  dataTagByIDsFlags,
									Action: createCommandWithT[dataTagByIDsArgs](DataTagActionByIds),
								},
								{
									Name:  "remove",
									Usage: "removes tags from binary data by file ids for a given org and location",
									UsageText: createUsageText(
										"data tag ids remove", []string{generalFlagTags, generalFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false, false,
									),
									Flags:  dataTagByIDsFlags,
									Action: createCommandWithT[dataTagByIDsArgs](DataTagActionByIds),
								},
							},
						},
						{
							Name:            "filter",
							Usage:           "adds or removes tags from binary data by filter",
							UsageText:       createUsageText("data tag filter", nil, false, true),
							HideHelpCommand: true,
							Subcommands: []*cli.Command{
								{
									Name:      "add",
									Usage:     "adds tags to binary data by filter",
									UsageText: createUsageText("data tag filter add", []string{generalFlagTags}, false, false),
									Flags:     dataTagByFilterFlags,
									Action:    createCommandWithT[dataTagByFilterArgs](DataTagActionByFilter),
								},
								{
									Name:      "remove",
									Usage:     "removes tags from binary data by filter",
									UsageText: createUsageText("data tag filter remove", []string{generalFlagTags}, false, false),
									Flags:     dataTagByFilterFlags,
									Action:    createCommandWithT[dataTagByFilterArgs](DataTagActionByFilter),
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
			UsageText:       createUsageText("dataset", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "create",
					Usage:     "create a new dataset",
					UsageText: createUsageText("dataset create", []string{generalFlagOrgID, datasetFlagName}, false, false),
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
					Action: createCommandWithT[datasetCreateArgs](DatasetCreateAction),
				},
				{
					Name:  "rename",
					Usage: "rename an existing dataset",
					UsageText: createUsageText("dataset rename",
						[]string{datasetFlagDatasetID, datasetFlagName}, false, false),
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
					Action: createCommandWithT[datasetRenameArgs](DatasetRenameAction),
				},
				{
					Name:  "list",
					Usage: "list dataset information from specified IDs or for an org ID",
					UsageText: fmt.Sprintf("viam dataset list [--%s=<%s> | --%s=<%s>]",
						datasetFlagDatasetIDs, datasetFlagDatasetIDs, generalFlagOrgID, generalFlagOrgID),
					Description: "In order to list datasets, an org ID or a list of dataset IDs is required",
					Flags: []cli.Flag{
						&cli.StringSliceFlag{
							Name:  datasetFlagDatasetIDs,
							Usage: fmt.Sprintf("dataset IDs of datasets to be listed, required if '%s' is not given", generalFlagOrgID),
						},
						&cli.StringFlag{
							Name:  generalFlagOrgID,
							Usage: fmt.Sprintf("org ID for which datasets will be listed, required if '%s' is not given", datasetFlagDatasetIDs),
						},
					},
					Action: createCommandWithT[datasetListArgs](DatasetListAction),
				},
				{
					Name:      "delete",
					Usage:     "delete a dataset",
					UsageText: createUsageText("dataset delete", []string{datasetFlagDatasetID}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "ID of the dataset to be deleted",
						},
					},
					Action: createCommandWithT[datasetDeleteArgs](DatasetDeleteAction),
				},
				{
					Name:  "export",
					Usage: "download data from a dataset",
					UsageText: createUsageText("dataset export",
						[]string{generalFlagDestination, datasetFlagDatasetID}, true, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:     generalFlagDestination,
							Required: true,
							Usage:    "output directory for downloaded data",
						},
						&cli.StringFlag{
							Name:     datasetFlagDatasetID,
							Required: true,
							Usage:    "dataset ID of the dataset to be downloaded",
						},
						&cli.BoolFlag{
							Name:  datasetFlagIncludeJSONLines,
							Usage: "option to include JSON Lines files for local testing",
						},
						&cli.UintFlag{
							Name:  dataFlagParallelDownloads,
							Usage: "number of download requests to make in parallel",
							Value: 100,
						},
						&cli.UintFlag{
							Name:  dataFlagTimeout,
							Usage: "number of seconds to wait for large file downloads",
							Value: 30,
						},
					},
					Action: createCommandWithT[datasetDownloadArgs](DatasetDownloadAction),
				},
				{
					Name:            "data",
					Usage:           "add or remove data from datasets",
					UsageText:       createUsageText("dataset data", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:            "add",
							Usage:           "adds binary data either by IDs or filter to dataset",
							UsageText:       createUsageText("dataset data add", nil, false, true),
							HideHelpCommand: true,
							Subcommands: []*cli.Command{
								{
									Name:  "ids",
									Usage: "adds binary data with file IDs in a single org and location to dataset",
									UsageText: createUsageText(
										"dataset data add ids", []string{datasetFlagDatasetID, generalFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false, false,
									),
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
									Action: createCommandWithT[dataAddToDatasetByIDsArgs](DataAddToDatasetByIDs),
								},
								{
									Name:      "filter",
									Usage:     "adds binary data from the specified filter to dataset",
									UsageText: createUsageText("dataset data add filter", []string{datasetFlagDatasetID}, true, false),
									Flags: append([]cli.Flag{
										&cli.StringFlag{
											Name:     datasetFlagDatasetID,
											Usage:    "dataset ID to which data will be added",
											Required: true,
										},
										&cli.StringSliceFlag{
											Name: generalFlagTags,
											Usage: "tags filter. " +
												"accepts tagged for all tagged data, untagged for all untagged data, or a list of tags for all data matching any of the tags",
										},
									},
										commonFilterFlags...),
									Action: createCommandWithT[dataAddToDatasetByFilterArgs](DataAddToDatasetByFilter),
								},
							},
						},
						{
							Name:  "remove",
							Usage: "removes binary data with file IDs in a single org and location from dataset",
							UsageText: createUsageText(
								"dataset data remove", []string{datasetFlagDatasetID, generalFlagOrgID, dataFlagLocationID, dataFlagFileIDs}, false, false,
							),
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
							Action: createCommandWithT[dataRemoveFromDatasetArgs](DataRemoveFromDataset),
						},
					},
				},
			},
		},
		{
			Name:            "train",
			Usage:           "train on data",
			UsageText:       createUsageText("train", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "submit",
					Usage:     "submits training job on data in Viam cloud",
					UsageText: createUsageText("train submit", nil, false, true),
					Subcommands: []*cli.Command{
						{
							Name:  "managed",
							Usage: "submits training job on data in Viam cloud with a Viam-managed training script",
							UsageText: createUsageText("train submit managed",
								[]string{datasetFlagDatasetID, trainFlagModelOrgID, trainFlagModelName, trainFlagModelType, trainFlagModelLabels},
								true, false,
							),
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
									Usage: formatAcceptedValues(
										"type of model to train",
										"single_label_classification", "multi_label_classification", "object_detection",
									),
									Required: true,
								},
								&cli.StringSliceFlag{
									Name:     trainFlagModelLabels,
									Usage:    "labels to train on. these will either be classification or object detection labels",
									Required: true,
								},
								&cli.StringFlag{
									Name:        trainFlagModelVersion,
									Usage:       "version of ML model",
									DefaultText: "current timestamp",
								},
							},
							Action: createCommandWithT[mlSubmitTrainingJobArgs](MLSubmitTrainingJob),
						},
						{
							Name:            "custom",
							Usage:           "submits custom training job on data in Viam cloud",
							UsageText:       createUsageText("train submit custom", nil, false, true),
							HideHelpCommand: true,
							Subcommands: []*cli.Command{
								{
									Name:  "from-registry",
									Usage: "submits custom training job with an existing training script in the registry on data in Viam cloud",
									UsageText: createUsageText("train submit custom from-registry",
										[]string{datasetFlagDatasetID, generalFlagOrgID, trainFlagModelName, mlTrainingFlagName, generalFlagVersion},
										true, false,
									),
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
												"should be formatted as prefix:itemname where prefix is either the org ID or the namespace.",
											Required: true,
										},
										&cli.StringFlag{
											Name:     generalFlagVersion,
											Usage:    "version of the ML training script to use for training.",
											Required: true,
										},
										&cli.StringSliceFlag{
											Name:  mlTrainingFlagArgs,
											Usage: "command line arguments to run the training script with. should be formatted as option1=value1,option2=value2",
										},
									},
									Action: createCommandWithT[mlSubmitCustomTrainingJobArgs](MLSubmitCustomTrainingJob),
								},
								{
									Name:  "with-upload",
									Usage: "submits custom training job with an upload training script on data in Viam cloud",
									UsageText: createUsageText("train submit custom with-upload",
										[]string{generalFlagOrgID, datasetFlagDatasetID, trainFlagModelOrgID, trainFlagModelName, generalFlagPath, mlTrainingFlagName},
										true, false,
									),
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
											Name:  mlTrainingFlagURL,
											Usage: "url of Github repository associated with the training scripts",
										},
										&cli.StringFlag{
											Name:     generalFlagPath,
											Usage:    "path to ML training scripts for upload",
											Required: true,
										},
										&cli.StringFlag{
											Name:     generalFlagOrgID,
											Usage:    "org ID to save the custom training script in",
											Required: true,
										},
										&cli.StringFlag{
											Name:     trainFlagModelOrgID,
											Required: true,
											Usage:    "org ID to upload and run training job",
										},
										&cli.StringFlag{
											Name:     mlTrainingFlagName,
											Usage:    "script name of the ML training script to upload",
											Required: true,
										},
										&cli.StringFlag{
											Name:        generalFlagVersion,
											Usage:       "version of the ML training script to upload",
											DefaultText: "current timestamp",
										},
										&cli.StringFlag{
											Name:  mlTrainingFlagFramework,
											Usage: formatAcceptedValues("framework of the ML training script to upload", modelFrameworks...),
										},
										&cli.StringFlag{
											Name:  trainFlagModelType,
											Usage: formatAcceptedValues("task type of the ML training script to upload", modelTypes...),
										},
										&cli.StringSliceFlag{
											Name:  mlTrainingFlagArgs,
											Usage: "command line arguments to run the training script with. should be formatted as option1=value1,option2=value2",
										},
									},
									Action: createCommandWithT[mlSubmitCustomTrainingJobWithUploadArgs](MLSubmitCustomTrainingJobWithUpload),
								},
							},
						},
					},
				},
				{
					Name:      "get",
					Usage:     "gets training job from Viam cloud based on training job ID",
					UsageText: createUsageText("train get", []string{trainFlagJobID}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: createCommandWithT[dataGetTrainingJobArgs](DataGetTrainingJob),
				},
				{
					Name:      "logs",
					Usage:     "gets training job logs from Viam cloud based on training job ID",
					UsageText: createUsageText("train logs", []string{trainFlagJobID}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: createCommandWithT[mlGetTrainingJobLogsArgs](MLGetTrainingJobLogs),
				},
				{
					Name:      "cancel",
					Usage:     "cancels training job in Viam cloud based on training job ID",
					UsageText: createUsageText("train cancel", []string{trainFlagJobID}, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     trainFlagJobID,
							Usage:    "training job ID",
							Required: true,
						},
					},
					Action: createCommandWithT[dataCancelTrainingJobArgs](DataCancelTrainingJob),
				},
				{
					Name:      "list",
					Usage:     "list training jobs in Viam cloud based on organization ID",
					UsageText: createUsageText("train list", []string{generalFlagOrgID}, true, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Usage:    "org ID",
							Required: true,
						},
						&cli.StringFlag{
							Name:  trainFlagJobStatus,
							Usage: formatAcceptedValues("training status to filter for", allTrainingStatusValues()...),
							Value: defaultTrainingStatus(),
						},
					},
					Action: createCommandWithT[dataListTrainingJobsArgs](DataListTrainingJobs),
				},
			},
		},
		{
			Name:            "machines",
			Aliases:         []string{"machine", "robots", "robot"},
			Usage:           "work with machines",
			UsageText:       createUsageText("machines", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "list",
					Usage:     "list machines in an organization and location",
					UsageText: createUsageText("machines list", nil, true, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        generalFlagOrganization,
							Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        generalFlagLocation,
							Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
							DefaultText: "first location alphabetically",
						},
					},
					Action: createCommandWithT[listRobotsActionArgs](ListRobotsAction),
				},
				{
					Name:            "api-key",
					Usage:           "work with a machine's api keys",
					UsageText:       createUsageText("machines api-key", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "create",
							Usage:     "create an api-key for your machine",
							UsageText: createUsageText("machines api-key create", []string{generalFlagMachineID}, true, false),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagMachineID,
										Aliases:  []string{generalFlagAliasRobotID},
										Required: true,
										Usage:    "id of the machine to create an api-key for",
									},
								},
								&cli.StringFlag{
									Name:        generalFlagName,
									Usage:       "the name of the key",
									DefaultText: "login info with the current time",
								},
								&cli.StringFlag{
									Name:        generalFlagOrgID,
									Usage:       "the org-id to attach this api-key to.",
									DefaultText: "the org attached to the machine if only one exists",
								},
							},
							Action: createCommandWithT[robotAPIKeyCreateArgs](RobotAPIKeyCreateAction),
						},
					},
				},
				{
					Name:      "status",
					Usage:     "display machine status",
					UsageText: createUsageText("machines status", []string{generalFlagMachine}, true, false),
					Flags: []cli.Flag{
						&AliasStringFlag{
							cli.StringFlag{
								Name:     generalFlagMachine,
								Aliases:  []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
								Required: true,
							},
						},
						&cli.StringFlag{
							Name:        generalFlagOrganization,
							Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        generalFlagLocation,
							Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
							DefaultText: "first location alphabetically",
						},
					},
					Action: createCommandWithT[robotsStatusArgs](RobotsStatusAction),
				},
				{
					Name:      "logs",
					Aliases:   []string{"log"},
					Usage:     "display machine logs",
					UsageText: createUsageText("machines logs", []string{generalFlagMachine}, true, false),
					Flags: []cli.Flag{
						&AliasStringFlag{
							cli.StringFlag{
								Name:     generalFlagMachine,
								Aliases:  []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
								Required: true,
							},
						},
						&cli.StringFlag{
							Name:        generalFlagOrganization,
							Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
							DefaultText: "first organization alphabetically",
						},
						&cli.StringFlag{
							Name:        generalFlagLocation,
							Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
							DefaultText: "first location alphabetically",
						},
						&cli.StringFlag{
							Name:  logsFlagOutputFile,
							Usage: "path to output file",
						},
						&cli.StringFlag{
							Name:  logsFlagFormat,
							Usage: "file format (text or json)",
						},
						&cli.StringFlag{
							Name:  logsFlagKeyword,
							Usage: "filter logs by keyword",
						},
						&cli.StringSliceFlag{
							Name:  logsFlagLevels,
							Usage: "filter logs by levels (e.g., info, warn, error)",
						},
						&cli.StringFlag{
							Name:  generalFlagStart,
							Usage: "ISO-8601 timestamp in RFC3339 format indicating the start of the interval filter (e.g., 2025-01-15T14:00:00Z)",
						},
						&cli.StringFlag{
							Name:  generalFlagEnd,
							Usage: "ISO-8601 timestamp in RFC3339 format indicating the end of the interval filter (e.g., 2025-01-15T15:00:00Z)",
						},
						&cli.IntFlag{
							Name:        generalFlagCount,
							Usage:       fmt.Sprintf("number of logs to fetch (max %v)", maxNumLogs),
							DefaultText: fmt.Sprintf("%v", defaultNumLogs),
						},
					},
					Action: createCommandWithT[robotsLogsArgs](RobotsLogsAction),
				},
				{
					Name:            "part",
					Usage:           "work with a machine part",
					UsageText:       createUsageText("machines part", nil, false, true),
					HideHelpCommand: true,
					Subcommands: []*cli.Command{
						{
							Name:      "status",
							Usage:     "display part status",
							UsageText: createUsageText("machines part status", []string{generalFlagPart}, true, false),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagPart,
										Aliases:  []string{generalFlagPartID, generalFlagPartName},
										Required: true,
									},
								},
								&cli.StringFlag{
									Name:        generalFlagOrganization,
									Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        generalFlagLocation,
									Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
									DefaultText: "first location alphabetically",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
								},
							},
							Action: createCommandWithT[robotsPartStatusArgs](RobotsPartStatusAction),
						},
						{
							Name:      "logs",
							Aliases:   []string{"log"},
							Usage:     "display part logs",
							UsageText: createUsageText("machines part logs", []string{generalFlagPart}, true, false),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagPart,
										Aliases:  []string{generalFlagPartID, generalFlagPartName},
										Required: true,
									},
								},
								&cli.StringFlag{
									Name:        generalFlagOrganization,
									Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        generalFlagLocation,
									Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
									DefaultText: "first location alphabetically",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
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
									Name:        generalFlagCount,
									Usage:       fmt.Sprintf("number of logs to fetch (max %v)", maxNumLogs),
									DefaultText: fmt.Sprintf("%v", defaultNumLogs),
								},
							},
							Action: createCommandWithT[robotsPartLogsArgs](RobotsPartLogsAction),
						},
						{
							Name:      "restart",
							Usage:     "request part restart",
							UsageText: createUsageText("machines part restart", []string{generalFlagPart}, true, false),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagPart,
										Aliases:  []string{generalFlagPartID, generalFlagPartName},
										Required: true,
									},
								},
								&cli.StringFlag{
									Name:        generalFlagOrganization,
									Aliases:     []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
									DefaultText: "first organization alphabetically",
								},
								&cli.StringFlag{
									Name:        generalFlagLocation,
									Aliases:     []string{generalFlagLocationID, generalFlagAliasLocationName},
									DefaultText: "first location alphabetically",
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
								},
							},
							Action: createCommandWithT[robotsPartRestartArgs](RobotsPartRestartAction),
						},
						{
							Name:      "run",
							Usage:     "run a command on a machine part",
							UsageText: createUsageText("machines part run", []string{generalFlagPart, generalFlagMethod}, true, false),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagPart,
										Aliases:  []string{generalFlagPartID, generalFlagPartName},
										Required: true,
									},
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagOrganization,
										Aliases: []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
									},
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagLocation,
										Aliases: []string{generalFlagLocationID, generalFlagAliasLocationName},
									},
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
								},
								&cli.StringFlag{
									Name:    runFlagData,
									Aliases: []string{"d"},
								},
								&cli.DurationFlag{
									Name:    runFlagStream,
									Aliases: []string{"s"},
								},
								&cli.StringFlag{
									Name:     generalFlagMethod,
									Usage:    "service method formatted as: '<service>.<method>' or '<service>/<method>'",
									Required: false, // should be required but set as false to ensure backwards capability
								},
							},
							Action: createCommandWithT[machinesPartRunArgs](MachinesPartRunAction),
						},
						{
							Name:  "shell",
							Usage: "start a shell on a machine part",
							Description: `
In order to use the shell command, the machine must have a valid shell type service.
Organization and location are required flags if the machine/part name are not unique across your account.
`,
							UsageText: createUsageText("machines part shell", []string{generalFlagPart}, false, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     generalFlagPart,
									Aliases:  []string{generalFlagPartID, generalFlagPartName},
									Required: true,
								},
								&cli.StringFlag{
									Name:    generalFlagOrganization,
									Aliases: []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
								},
								&cli.StringFlag{
									Name:    generalFlagLocation,
									Aliases: []string{generalFlagLocationID, generalFlagAliasLocationName},
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
								},
							},
							Action: createCommandWithT[robotsPartShellArgs](RobotsPartShellAction),
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
'viam machine part cp --part "m1-main" -r my_dir machine:/home/user/'

Copy multiple files to the machine with recursion and keep original permissions and metadata:
'viam machine part cp --part "m1-main" -r -p my_dir my_file machine:/home/user/some/existing/dir/'

Copy a single file from the machine to a local destination:
'viam machine part cp --part "m1-main" machine:my_file ~/Downloads/'

Recursively copy a directory from the machine to a local destination with the same name:
'viam machine part cp --part "m1-main" -r machine:my_dir ~/Downloads/'

Copy multiple files from the machine to a local destination with recursion and keep original permissions and metadata:
'viam machine part cp --part "m1-main" -r -p machine:my_dir machine:my_file ~/some/existing/dir/'
`,
							UsageText: createUsageText(
								"machines part cp",
								[]string{generalFlagPart},
								true, false,
								"<source i.e. [machine:]files>... <target i.e. [machine:]files>"),
							Flags: []cli.Flag{
								&AliasStringFlag{
									cli.StringFlag{
										Name:     generalFlagPart,
										Aliases:  []string{generalFlagPartID, generalFlagPartName},
										Required: true,
									},
								},
								&cli.StringFlag{
									Name:    generalFlagOrganization,
									Aliases: []string{generalFlagAliasOrg, generalFlagOrgID, generalFlagAliasOrgName},
								},
								&cli.StringFlag{
									Name:    generalFlagLocation,
									Aliases: []string{generalFlagLocationID, generalFlagAliasLocationName},
								},
								&AliasStringFlag{
									cli.StringFlag{
										Name:    generalFlagMachine,
										Aliases: []string{generalFlagAliasRobot, generalFlagMachineID, generalFlagMachineName},
									},
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
							Action: createCommandWithT[machinesPartCopyFilesArgs](MachinesPartCopyFilesAction),
						},
					},
				},
			},
		},
		{
			Name:            "module",
			Usage:           "manage your modules in Viam's registry",
			UsageText:       createUsageText("module", nil, false, true),
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
					UsageText: createUsageText("module create", []string{generalFlagName}, true, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagName,
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
					Action: createCommandWithT[createModuleActionArgs](CreateModuleAction),
				},
				{
					Name:      "generate",
					Usage:     "generate a new modular resource via prompts",
					UsageText: createUsageText("module generate", nil, true, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  generalFlagName,
							Usage: "name to use for module. for example, a module that contains sensor implementations might be named 'sensors'",
						},
						&cli.StringFlag{
							Name:  moduleFlagLanguage,
							Usage: formatAcceptedValues("language to use for module", supportedModuleGenLanguages...),
						},
						&cli.BoolFlag{
							Name:  moduleFlagIsPublic,
							Usage: "set module to public",
						},
						&cli.StringFlag{
							Name: moduleFlagPublicNamespace,
							Usage: "namespace or organization ID of module. " +
								"must be either a valid organization ID, or a namespace that exists within a user organization",
						},
						&cli.StringFlag{
							Name: generalFlagResourceSubtype,
							Usage: "resource subtype to use in module, for example arm, camera, or motion. see " +
								"https://docs.viam.com/dev/reference/glossary/#term-subtype for more details",
						},
						// This is unnecessary and creates a gotcha for users. Kept here
						// because it's technically breaking to remove it, but it's hidden
						// and serves no purpose.
						&cli.StringFlag{
							Name:   moduleFlagResourceType,
							Hidden: true,
						},
						&cli.StringFlag{
							Name: moduleFlagModelName,
							Usage: "name for the particular resource subtype implementation." +
								" for example, a sensor model that detects moisture might be named 'moisture'",
						},
						&cli.BoolFlag{
							Name:  moduleFlagEnableCloud,
							Usage: "generate Github workflows to build module",
						},
						&cli.BoolFlag{
							Name:  moduleFlagRegister,
							Usage: "register module with Viam to associate with your organization",
						},
						&cli.BoolFlag{
							Name:   moduleFlagDryRun,
							Usage:  "indicate a dry test run, so skip regular checks",
							Hidden: true,
						},
					},
					Action: createCommandWithT[generateModuleArgs](GenerateModuleAction),
				},
				{
					Name:      "update",
					Usage:     "update a module's metadata on app.viam.com",
					UsageText: createUsageText("module update", nil, false, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:      moduleFlagPath,
							Usage:     "path to meta.json",
							Value:     "./meta.json",
							TakesFile: true,
						},
					},
					Action: createCommandWithT[updateModuleArgs](UpdateModuleAction),
				},
				{
					Name:      "update-models",
					Usage:     "update a module's metadata file based on models it provides",
					UsageText: createUsageText("module update-models", []string{moduleFlagBinary}, true, false),
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
					Action: createCommandWithT[updateModelsArgs](UpdateModelsAction),
				},
				{
					Name:  "upload",
					Usage: "upload a new version of your module",
					Description: `Upload an archive containing your module's file(s) for a specified platform
Example uploading a single file:
viam module upload --version "0.1.0" --platform "linux/amd64" --upload "./bin/my-module"
(this example requires the entrypoint in the meta.json to be "./bin/my-module")

Example uploading a whole directory:
viam module upload --version "0.1.0" --platform "linux/amd64" --upload "./bin"
(this example requires the entrypoint in the meta.json to be inside the bin directory like "./bin/[your path here]")

Example uploading a custom tarball of your module:
tar -czf packaged-module.tar.gz ./src requirements.txt run.sh
viam module upload --version "0.1.0" --platform "linux/amd64" --upload "packaged-module.tar.gz"
                      `,
					UsageText: createUsageText("module upload", []string{generalFlagVersion, moduleFlagPlatform, moduleFlagUpload}, true, false),
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
							Name:  generalFlagName,
							Usage: "name of the module (used if you don't have a meta.json)",
						},
						&cli.StringFlag{
							Name:     generalFlagVersion,
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
						&cli.StringSliceFlag{
							Name: generalFlagTags,
							Usage: `extra fields for constraining the platforms to which this binary
                             is deployed. Examples: distro:debian, distro:ubuntu, os_version:22.04,
                             os_codename:jammy. For a machine to use an upload, all tags must be
                             satisified as well as the --platform field.`,
						},
						&cli.BoolFlag{
							Name:  moduleFlagForce,
							Usage: "skip validation (may result in non-functional versions)",
						},
						&cli.StringFlag{
							Name:     moduleFlagUpload,
							Usage:    "the path to the upload",
							Required: false, // should be true, but set to false to allow for backwards compatibility
						},
					},
					Action: createCommandWithT[uploadModuleArgs](UploadModuleAction),
				},
				{
					Name:            "build",
					Usage:           "build your module for different architectures using cloud runners",
					UsageText:       createUsageText("module build", nil, false, true),
					HideHelpCommand: true,
					Description: `Build your module on different operating systems and cpu architectures via cloud runners.
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
							Name:      "local",
							Usage:     "run your meta.json build command locally",
							UsageText: createUsageText("module build local", nil, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
							},
							Action: createCommandWithT[moduleBuildLocalArgs](ModuleBuildLocalAction),
						},
						{
							Name:      "start",
							Usage:     "start a remote build",
							UsageText: createUsageText("module build start", []string{generalFlagVersion}, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
								&cli.StringFlag{
									Name:     generalFlagVersion,
									Usage:    "version of the module to upload (semver2.0) ex: \"0.1.0\"",
									Required: true,
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagRef,
									Usage: "git ref to clone when building your module. This can be a branch name or a commit hash",
									Value: "main",
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagToken,
									Usage: "checkout token for private repos, not necessary for public repos",
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagWorkdir,
									Usage: "use this to indicate that your meta.json is in a subdirectory of your repo. --module flag should be relative to this",
									Value: ".",
								},
								&cli.StringSliceFlag{
									Name: moduleBuildFlagPlatforms,
									// would use 'DefaultText' key, but defaults don't show for slice flags
									Usage: "list of platforms to build, e.g. linux/amd64,linux/arm64 (default: build.arch in meta.json)",
								},
							},
							Action: createCommandWithT[moduleBuildStartArgs](ModuleBuildStartAction),
						},
						{
							Name:      "list",
							Usage:     "check on the status of your cloud builds",
							UsageText: createUsageText("module build list", nil, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:      moduleFlagPath,
									Usage:     "path to meta.json",
									Value:     "./meta.json",
									TakesFile: true,
								},
								&cli.IntFlag{
									Name:        generalFlagCount,
									Usage:       "number of builds to list",
									Aliases:     []string{"c"},
									DefaultText: "all",
								},
								&cli.StringFlag{
									Name:  moduleFlagID,
									Usage: "restrict output to just return builds that match this id",
								},
							},
							Action: createCommandWithT[moduleBuildListArgs](ModuleBuildListAction),
						},
						{
							Name:      "logs",
							Aliases:   []string{"log"},
							Usage:     "get the logs from one of your cloud builds",
							UsageText: createUsageText("module build logs", []string{moduleFlagID}, true, false),
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:     moduleFlagID,
									Usage:    "build that you want to get the logs for",
									Required: true,
								},
								&cli.StringFlag{
									Name:        moduleFlagPlatform,
									Usage:       "build platform to get the logs for. Ex: linux/arm64.",
									DefaultText: "all platforms",
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
							Action: createCommandWithT[moduleBuildLogsArgs](ModuleBuildLogsAction),
						},
						{
							Name:      "link-repo",
							Usage:     "link a GitHub repository to your module",
							UsageText: createUsageText("module build link-repo", nil, true, false),
							Description: `
This command connects a Viam module to a GitHub repository so that repo actions can trigger builds and releases of your module.

This won't work unless you have an existing installation of our GitHub app on your GitHub org. (Details to follow).`,
							// TODO(APP-3604): unhide when this is shipped externally
							Hidden: true,
							Flags: []cli.Flag{
								&cli.StringFlag{
									Name:  moduleBuildFlagOAuthLink,
									Usage: "ID of the oauth link between your GitHub org and Viam. Only required if you have more than one link",
								},
								&cli.StringFlag{
									Name:        moduleFlagPath,
									Usage:       "your module's ID in org-id:name or public-namespace:name format",
									DefaultText: "meta.json file in current directory",
								},
								&cli.StringFlag{
									Name:  moduleBuildFlagRepo,
									Usage: "your github repository in account/repository form (e.g. viamrobotics/rdk, not github.com/viamrobotics/rdk)",
								},
							},
							Action: createCommandWithT[moduleBuildLinkRepoArgs](ModuleBuildLinkRepoAction),
						},
					},
				},
				{
					Name:      "reload",
					Usage:     "build a module locally and run it on a target device. rebuild & restart if already running",
					UsageText: createUsageText("module reload", nil, true, false),
					Description: `Example invocations:

	# A full reload command. This will build your module, send the tarball to the machine with given part ID,
	# and configure or restart it.
	# The GOARCH env in this case would get passed to an underlying go build (assuming you're targeting an arm device).
	# Note that you'll still need to add the components for your models after your module is installed.
	GOARCH=arm64 viam module reload --part-id UUID

	# Restart a module running on your local viam server, by name, without building or reconfiguring.
	viam module reload --restart-only --id viam:python-example-module

	# Build and configure a module on your local machine without shipping a tarball.
	viam module reload --local`,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:        generalFlagPartID,
							Usage:       "part ID of machine. get from 'Live/Offline' dropdown in the web app",
							DefaultText: "/etc/viam.json",
						},
						&cli.StringFlag{
							Name:  moduleFlagPath,
							Usage: "path to a meta.json. used for module ID. can be overridden with --id or --name",
							Value: "meta.json",
						},
						&cli.StringFlag{
							Name:  generalFlagName,
							Usage: "name of module to restart. pass at most one of --name, --id",
						},
						&cli.StringFlag{
							Name:  moduleFlagID,
							Usage: "ID of module to restart, for example viam:wifi-sensor. pass at most one of --name, --id",
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
					Action: createCommandWithT[reloadModuleArgs](ReloadModuleAction),
				},
				{
					Name:      "download",
					Usage:     "download a module package from the registry",
					UsageText: createUsageText("module download", []string{}, true, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:  generalFlagDestination,
							Usage: "output directory for downloaded package",
							Value: ".",
						},
						&cli.StringFlag{
							Name:        moduleFlagID,
							Usage:       "module ID as org-id:name or namespace:name",
							DefaultText: "will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:  generalFlagVersion,
							Usage: "version of the requested package. can be `latest` to get the most recent version",
							Value: "latest",
						},
						&cli.StringFlag{
							Name:        moduleFlagPlatform,
							Usage:       "platform like 'linux/amd64'",
							DefaultText: "platform of the CLI binary",
						},
					},
					Action: createCommandWithT[downloadModuleFlags](DownloadModuleAction),
				},
			},
		},
		{
			Name:            "packages",
			Usage:           "work with packages",
			UsageText:       createUsageText("packages", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "export",
					Usage:     "download a package from Viam cloud",
					UsageText: createUsageText("packages export", []string{generalFlagType}, false, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:  generalFlagDestination,
							Usage: "output directory for downloaded package",
							Value: ".",
						},
						&cli.StringFlag{
							Name:        generalFlagOrgID,
							Usage:       "organization ID or namespace of the requested package",
							DefaultText: "will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:        generalFlagName,
							Usage:       "name of the requested package",
							DefaultText: "will try to read from meta.json",
						},
						&cli.StringFlag{
							Name:  generalFlagVersion,
							Usage: "version of the requested package. can be `latest` to get the most recent version",
							Value: "latest",
						},
						&cli.StringFlag{
							Name:     generalFlagType,
							Required: true,
							Usage:    formatAcceptedValues("type of the requested package", packageTypes...),
						},
					},
					Action: createCommandWithT[packageExportArgs](PackageExportAction),
				},
				{
					Name:  "upload",
					Usage: "upload a package to Viam cloud",
					UsageText: createUsageText("packages upload",
						[]string{generalFlagPath, generalFlagOrgID, generalFlagName, generalFlagVersion, generalFlagType},
						false, false),
					Flags: []cli.Flag{
						&cli.PathFlag{
							Name:     generalFlagPath,
							Required: true,
							Usage:    "path to package for upload",
						},
						&cli.StringFlag{
							Name:     generalFlagOrgID,
							Required: true,
							Usage:    "organization ID of the requested package",
						},
						&cli.StringFlag{
							Name:     generalFlagName,
							Required: true,
							Usage:    "name of the requested package",
						},
						&cli.StringFlag{
							Name:     generalFlagVersion,
							Required: true,
							Usage:    "version of the requested package. can be `latest` to get the most recent version",
						},
						&cli.StringFlag{
							Name:     generalFlagType,
							Required: true,
							Usage:    formatAcceptedValues("type of the requested package", packageTypes...),
						},
						&cli.StringFlag{
							Name: packageFlagFramework,
							Usage: formatAcceptedValues(
								"framework for an ml_model being uploaded. Required if packages is of type 'ml_model'", modelFrameworks...,
							),
						},
					},
					Action: createCommandWithT[packageUploadArgs](PackageUploadAction),
				},
			},
		},
		{
			Name:            "training-script",
			Usage:           "manage training scripts for custom ML training",
			UsageText:       createUsageText("training-script", nil, false, true),
			HideHelpCommand: true,
			Subcommands: []*cli.Command{
				{
					Name:      "upload",
					Usage:     "upload ML training scripts for custom ML training",
					UsageText: createUsageText("training-script upload", []string{generalFlagOrgID, generalFlagPath, mlTrainingFlagName}, true, false),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     generalFlagPath,
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
							Name:  generalFlagVersion,
							Usage: "version of the ML training script to upload",
						},
						&cli.StringFlag{
							Name:  mlTrainingFlagFramework,
							Usage: formatAcceptedValues("framework of the ML training script to upload", modelFrameworks...),
						},
						&cli.StringFlag{
							Name:  generalFlagType,
							Usage: formatAcceptedValues("task type of the ML training script to upload", modelTypes...),
						},
						&cli.BoolFlag{
							Name:  mlTrainingFlagDraft,
							Usage: "indicate draft mode, drafts will not be viewable in the registry",
						},
						&cli.StringFlag{
							Name:  mlTrainingFlagURL,
							Usage: "url of Github repository associated with the training scripts",
						},
					},
					Action: createCommandWithT[mlTrainingUploadArgs](MLTrainingUploadAction),
				},
				{
					Name:  "update",
					Usage: "update ML training scripts for custom ML training",
					UsageText: createUsageText(
						"training-script update", []string{generalFlagOrgID, mlTrainingFlagName, mlTrainingFlagVisibility}, true, false,
					),
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
							Usage:    formatAcceptedValues("visibility of the registry item", "public", "private"),
							Required: true,
						},
						&cli.StringFlag{
							Name:  mlTrainingFlagDescription,
							Usage: "description of the ML training script",
						},
						&cli.StringFlag{
							Name:  mlTrainingFlagURL,
							Usage: "url of Github repository associated with the training scripts",
						},
					},
					Action: createCommandWithT[mlTrainingUpdateArgs](MLTrainingUpdateAction),
				},
			},
		},
		{
			Name:      "version",
			Usage:     "print version info for this program",
			UsageText: createUsageText("version", nil, false, false),
			Action:    createCommandWithT[emptyArgs](VersionAction),
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
