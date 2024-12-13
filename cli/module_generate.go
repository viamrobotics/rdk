package cli

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.viam.com/utils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"go.viam.com/rdk/cli/module_generate/common"
	gen "go.viam.com/rdk/cli/module_generate/scripts"
)

//go:embed module_generate/scripts
var scripts embed.FS

//go:embed all:module_generate/_templates/*
var templates embed.FS

const (
	version        = "0.1.0"
	basePath       = "module_generate"
	templatePrefix = "tmpl-"
	python         = "python"
	golang         = "go"
)

var (
	scriptsPath   = filepath.Join(basePath, "scripts")
	templatesPath = filepath.Join(basePath, "_templates")
)

type generateModuleArgs struct {
	Language        string
	ResourceType    string
	ResourceSubtype string
}

// GenerateModuleAction runs the module generate cli and generates necessary module templates based on user input.
func GenerateModuleAction(cCtx *cli.Context, args generateModuleArgs) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.generateModuleAction(cCtx, args)
}

func (c *viamClient) generateModuleAction(cCtx *cli.Context, args generateModuleArgs) error {
	var newModule *common.ModuleInputs
	var err error
	resourceType := args.ResourceType
	resourceSubtype := args.ResourceSubtype
	if resourceSubtype != "" && resourceType != "" {
		newModule = &common.ModuleInputs{
			ModuleName:       "my-module",
			IsPublic:         false,
			Namespace:        "my-org",
			Language:         args.Language,
			Resource:         resourceSubtype + " " + resourceType,
			ResourceType:     resourceType,
			ResourceSubtype:  resourceSubtype,
			ModelName:        "my-model",
			EnableCloudBuild: false,
			RegisterOnApp:    false,
			GeneratorVersion: "0.1.0",
			GeneratedOn:      time.Now().UTC(),

			ModulePascal:          "MyModule",
			API:                   fmt.Sprintf("rdk:%s:%s", resourceType, resourceSubtype),
			ResourceSubtypePascal: strings.ToUpper(string(resourceSubtype[0])) + resourceSubtype[1:],
			ModelPascal:           "MyModel",
			ModelTriple:           "my-org:my-module:my-model",

			SDKVersion: "0.0.0",
		}
		populateAdditionalInfo(newModule)
	} else {
		newModule, err = promptUser()
		if err != nil {
			return err
		}
		populateAdditionalInfo(newModule)
		if err := wrapResolveOrg(cCtx, c, newModule); err != nil {
			return err
		}
	}

	s := spinner.New()
	var fatalError error
	nonFatalError := false
	globalArgs := parseStructFromCtx[globalArgs](cCtx)
	action := func() {
		s.Title("Getting latest release...")
		version, err := getLatestSDKTag(cCtx, newModule.Language, globalArgs)
		if err != nil {
			fatalError = err
			return
		}
		newModule.SDKVersion = version[1:]

		s.Title("Setting up module directory...")
		if err = setupDirectories(cCtx, newModule.ModuleName, globalArgs); err != nil {
			fatalError = err
			return
		}

		s.Title("Creating module and generating manifest...")
		if err = createModuleAndManifest(cCtx, c, *newModule, globalArgs); err != nil {
			fatalError = err
			return
		}

		s.Title("Rendering common files...")
		if err = renderCommonFiles(cCtx, *newModule, globalArgs); err != nil {
			fatalError = err
			return
		}

		s.Title(fmt.Sprintf("Copying %s files...", newModule.Language))
		if err = copyLanguageTemplate(cCtx, newModule.Language, newModule.ModuleName, globalArgs); err != nil {
			fatalError = err
			return
		}

		s.Title("Rendering template...")
		if err = renderTemplate(cCtx, *newModule, globalArgs); err != nil {
			fatalError = err
			return
		}

		s.Title(fmt.Sprintf("Generating %s stubs...", newModule.Language))
		if err = generateStubs(cCtx, *newModule, globalArgs); err != nil {
			warningf(cCtx.App.ErrWriter, err.Error())
			nonFatalError = true
		}

		s.Title("Generating cloud build requirements...")
		if err = generateCloudBuild(cCtx, *newModule, globalArgs); err != nil {
			warningf(cCtx.App.ErrWriter, err.Error())
			nonFatalError = true
		}
	}

	if globalArgs.Debug {
		action()
	} else {
		s.Action(action)
		err := s.Run()
		if err != nil {
			return err
		}
	}

	if fatalError != nil {
		err := os.RemoveAll(newModule.ModuleName)
		if err != nil {
			return errors.Wrap(fatalError, fmt.Sprintf("some steps of module generation failed, "+
				"incomplete module located at %s", newModule.ModuleName))
		}
		return errors.Wrap(fatalError, "unable to generate module")
	}

	if nonFatalError {
		return fmt.Errorf("some steps of module generation failed, incomplete module located at %s", newModule.ModuleName)
	}

	printf(cCtx.App.Writer, "Module successfully generated at %s", newModule.ModuleName)
	return nil
}

// Prompt the user for information regarding the module they want to create
// returns the common.ModuleInputs struct that contains the information the user entered.
func promptUser() (*common.ModuleInputs, error) {
	var newModule common.ModuleInputs
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Generate a new modular resource").
				Description("For more details about modular resources, view the documentation at \nhttps://docs.viam.com/registry/"),
			huh.NewInput().
				Title("Set a module name:").
				Description("The module name can contain only alphanumeric characters, dashes, and underscores.").
				Value(&newModule.ModuleName).
				Placeholder("my-module").
				Suggestions([]string{"my-module"}).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("module name must not be empty")
					}
					match, err := regexp.MatchString("^[a-z0-9]+(?:[_-][a-z0-9]+)*$", s)
					if !match || err != nil {
						return errors.New("module names can only contain alphanumeric characters, dashes, and underscores")
					}
					if _, err := os.Stat(s); err == nil {
						return errors.New("this module directory already exists")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Specify the language for the module:").
				Options(
					huh.NewOption("Python", python),
					huh.NewOption("Go", golang),
				).
				Value(&newModule.Language),
			huh.NewConfirm().
				Title("Visibility").
				Affirmative("Public").
				Negative("Private").
				Value(&newModule.IsPublic),
			huh.NewInput().
				Title("Namespace/Organization ID").
				Value(&newModule.Namespace).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("namespace or org ID must not be empty")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Select a resource to be added to the module:").
				Options(
					huh.NewOption("Arm Component", "arm component"),
					huh.NewOption("Audio Input Component", "audio_input component"),
					huh.NewOption("Base Component", "base component"),
					huh.NewOption("Board Component", "board component"),
					huh.NewOption("Camera Component", "camera component"),
					huh.NewOption("Encoder Component", "encoder component"),
					huh.NewOption("Gantry Component", "gantry component"),
					huh.NewOption("Generic Component", "generic component"),
					huh.NewOption("Gripper Component", "gripper component"),
					huh.NewOption("Input Component", "input component"),
					huh.NewOption("Motor Component", "motor component"),
					huh.NewOption("Movement Sensor Component", "movement_sensor component"),
					huh.NewOption("Pose Tracker Component", "pose_tracker component"),
					huh.NewOption("Power Sensor Component", "power_sensor component"),
					huh.NewOption("Sensor Component", "sensor component"),
					huh.NewOption("Servo Component", "servo component"),
					huh.NewOption("Generic Service", "generic service"),
					huh.NewOption("MLModel Service", "mlmodel service"),
					huh.NewOption("Motion Service", "motion service"),
					huh.NewOption("Navigation Service", "navigation service"),
					huh.NewOption("SLAM Service", "slam service"),
					huh.NewOption("Vision Service", "vision service"),
				).
				Value(&newModule.Resource).WithHeight(25),
			huh.NewInput().
				Title("Set a model name of the resource:").
				Description("This is the name of the new resource model that your module will provide.\n"+
					"The model name can contain only alphanumeric characters, dashes, and underscores.").
				Placeholder("my-model").
				Value(&newModule.ModelName).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("model name must not be empty")
					}
					match, err := regexp.MatchString("^[a-zA-Z0-9]+(?:[_-][a-zA-Z0-9]+)*$", s)
					if !match || err != nil {
						return errors.New("module names can only contain alphanumeric characters, dashes, and underscores")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Enable cloud build").
				Description("If enabled, this will generate GitHub workflows to build your module.").
				Value(&newModule.EnableCloudBuild),
			huh.NewConfirm().
				Title("Register module").
				Description("Register this module with Viam.\nIf selected, "+
					"this will associate the module with your organization.\nOtherwise, this will be a local-only module.").
				Value(&newModule.RegisterOnApp),
		),
	).WithHeight(25).WithWidth(88)
	err := form.Run()
	if err != nil {
		return nil, errors.Wrap(err, "encountered an error generating module")
	}

	return &newModule, nil
}

func wrapResolveOrg(cCtx *cli.Context, c *viamClient, newModule *common.ModuleInputs) error {
	match, err := regexp.MatchString("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", newModule.Namespace)
	if !match || err != nil {
		// If newModule.Namespace is NOT a UUID
		org, err := resolveOrg(c, newModule.Namespace, "")
		if err != nil {
			return catchResolveOrgErr(cCtx, c, newModule, err)
		}
		newModule.OrgID = org.GetId()
	} else {
		// If newModule.Namespace is a UUID/OrgID
		org, err := resolveOrg(c, "", newModule.Namespace)
		if err != nil {
			return catchResolveOrgErr(cCtx, c, newModule, err)
		}
		newModule.OrgID = newModule.Namespace
		newModule.Namespace = org.GetPublicNamespace()
	}
	return nil
}

func catchResolveOrgErr(cCtx *cli.Context, c *viamClient, newModule *common.ModuleInputs, caughtErr error) error {
	if strings.Contains(caughtErr.Error(), "not logged in") || strings.Contains(caughtErr.Error(), "error while refreshing token") {
		originalWriter := cCtx.App.Writer
		cCtx.App.Writer = io.Discard
		err := c.loginAction(cCtx)
		cCtx.App.Writer = originalWriter
		if err != nil {
			return err
		}
		return wrapResolveOrg(cCtx, c, newModule)
	}
	if strings.Contains(caughtErr.Error(), "none of your organizations have a public namespace") ||
		strings.Contains(caughtErr.Error(), "no organization found for") {
		return errors.Wrapf(caughtErr, "cannot create module for an organization of which you are not a member")
	}
	return caughtErr
}

// populateAdditionalInfo fills in additional info in newModule.
func populateAdditionalInfo(newModule *common.ModuleInputs) {
	newModule.GeneratedOn = time.Now().UTC()
	newModule.GeneratorVersion = version
	newModule.ResourceSubtype = strings.Split(newModule.Resource, " ")[0]
	newModule.ResourceType = strings.Split(newModule.Resource, " ")[1]

	titleCaser := cases.Title(language.Und)
	replacer := strings.NewReplacer("_", " ", "-", " ")
	spaceReplacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	newModule.ModulePascal = spaceReplacer.Replace(titleCaser.String(replacer.Replace(newModule.ModuleName)))
	newModule.ModuleCamel = strings.ToLower(string(newModule.ModulePascal[0])) + newModule.ModulePascal[1:]
	newModule.ModuleLowercase = strings.ToLower(newModule.ModulePascal)
	newModule.API = fmt.Sprintf("rdk:%s:%s", newModule.ResourceType, newModule.ResourceSubtype)
	newModule.ResourceSubtypePascal = spaceReplacer.Replace(titleCaser.String(replacer.Replace(newModule.ResourceSubtype)))
	if newModule.Language == golang {
		newModule.ResourceSubtype = spaceReplacer.Replace(newModule.ResourceSubtype)
	}
	newModule.ResourceTypePascal = spaceReplacer.Replace(titleCaser.String(replacer.Replace(newModule.ResourceType)))
	newModule.ModelPascal = spaceReplacer.Replace(titleCaser.String(replacer.Replace(newModule.ModelName)))
	newModule.ModelTriple = fmt.Sprintf("%s:%s:%s", newModule.Namespace, newModule.ModuleName, newModule.ModelName)
	newModule.ModelCamel = strings.ToLower(string(newModule.ModelPascal[0])) + newModule.ModelPascal[1:]
	newModule.ModelLowercase = strings.ToLower(newModule.ModelPascal)
}

// Creates a new directory with moduleName.
func setupDirectories(c *cli.Context, moduleName string, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Setting up directories")
	err := os.Mkdir(moduleName, 0o750)
	if err != nil {
		return err
	}
	return nil
}

func renderCommonFiles(c *cli.Context, module common.ModuleInputs, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, module.ResourceSubtypePascal)
	debugf(c.App.Writer, globalArgs.Debug, "Rendering common files")

	// Render .viam-gen-info
	infoBytes, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return err
	}

	infoFilePath := filepath.Join(module.ModuleName, ".viam-gen-info")
	//nolint:gosec
	infoFile, err := os.Create(infoFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", infoFilePath)
	}
	defer utils.UncheckedErrorFunc(infoFile.Close)

	if _, err := infoFile.Write(infoBytes); err != nil {
		return errors.Wrapf(err, "failed to write generator info to %s", infoFilePath)
	}

	// Render workflows for cloud build
	if module.EnableCloudBuild {
		debugf(c.App.Writer, globalArgs.Debug, "\tCreating cloud build workflow")
		destWorkflowPath := filepath.Join(module.ModuleName, ".github")
		if err = os.Mkdir(destWorkflowPath, 0o750); err != nil {
			return errors.Wrap(err, "failed to create cloud build workflow")
		}

		workflowPath := filepath.Join(templatesPath, ".github")
		workflowFS, err := fs.Sub(templates, workflowPath)
		if err != nil {
			return errors.Wrap(err, "failed to create cloud build workflow")
		}

		err = fs.WalkDir(workflowFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() != ".github" {
					debugf(c.App.Writer, globalArgs.Debug, "\t\tCopying %s directory", d.Name())
					err = os.Mkdir(filepath.Join(destWorkflowPath, path), 0o750)
					if err != nil {
						return err
					}
				}
			} else if !strings.HasPrefix(d.Name(), templatePrefix) {
				debugf(c.App.Writer, globalArgs.Debug, "\t\tCopying file %s", path)
				srcFile, err := templates.Open(filepath.Join(workflowPath, path))
				if err != nil {
					return errors.Wrapf(err, "error opening file %s", srcFile)
				}
				defer utils.UncheckedErrorFunc(srcFile.Close)

				destPath := filepath.Join(destWorkflowPath, path)
				//nolint:gosec
				destFile, err := os.Create(destPath)
				if err != nil {
					return errors.Wrapf(err, "failed to create file %s", destPath)
				}
				defer utils.UncheckedErrorFunc(destFile.Close)

				_, err = io.Copy(destFile, srcFile)
				if err != nil {
					return errors.Wrapf(err, "error executing template for %s", destPath)
				}
			}
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to render all common files")
		}
		return nil
	}

	return nil
}

// copyLanguageTemplate copies the files from templates/language directory into the moduleName root directory.
func copyLanguageTemplate(c *cli.Context, language, moduleName string, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Creating %s template files", language)
	languagePath := filepath.Join(templatesPath, language)
	tempDir, err := fs.Sub(templates, languagePath)
	if err != nil {
		return err
	}
	err = fs.WalkDir(tempDir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() != language {
				debugf(c.App.Writer, globalArgs.Debug, "\tCopying %s directory", d.Name())
				err = os.Mkdir(filepath.Join(moduleName, path), 0o750)
				if err != nil {
					return err
				}
			}
		} else if !strings.HasPrefix(d.Name(), templatePrefix) {
			debugf(c.App.Writer, globalArgs.Debug, "\tCopying file %s", path)
			srcFile, err := templates.Open(filepath.Join(languagePath, path))
			if err != nil {
				return errors.Wrapf(err, "error opening file %s", srcFile)
			}
			defer utils.UncheckedErrorFunc(srcFile.Close)

			destPath := filepath.Join(moduleName, path)
			//nolint:gosec
			destFile, err := os.Create(destPath)
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", destPath)
			}
			defer utils.UncheckedErrorFunc(destFile.Close)

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return errors.Wrapf(err, "error executing template for %s", destPath)
			}
			if filepath.Ext(destPath) == ".sh" {
				//nolint:gosec
				err = os.Chmod(destPath, 0o750)
				if err != nil {
					return errors.Wrapf(err, "error making file executable for %s", destPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to render all %s files", language)
	}
	return nil
}

// Render all the files in the new directory.
func renderTemplate(c *cli.Context, module common.ModuleInputs, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Rendering template files")
	languagePath := filepath.Join(templatesPath, module.Language)
	tempDir, err := fs.Sub(templates, languagePath)
	if err != nil {
		return err
	}
	err = fs.WalkDir(tempDir, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasPrefix(d.Name(), templatePrefix) {
			destPath := filepath.Join(module.ModuleName, strings.ReplaceAll(path, templatePrefix, ""))
			debugf(c.App.Writer, globalArgs.Debug, "\tRendering file %s", destPath)

			tFile, err := templates.Open(filepath.Join(languagePath, path))
			if err != nil {
				return err
			}
			defer utils.UncheckedErrorFunc(tFile.Close)
			tBytes, err := io.ReadAll(tFile)
			if err != nil {
				return err
			}

			tmpl, err := template.New(path).Parse(string(tBytes))
			if err != nil {
				return err
			}

			//nolint:gosec
			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer utils.UncheckedErrorFunc(destFile.Close)

			err = tmpl.Execute(destFile, module)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// Generate stubs for the resource.
func generateStubs(c *cli.Context, module common.ModuleInputs, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Generating %s stubs", module.Language)
	switch module.Language {
	case python:
		return generatePythonStubs(module)
	case golang:
		return generateGolangStubs(module)
	default:
		return errors.Errorf("cannot generate stubs for language %s", module.Language)
	}
}

func generateGolangStubs(module common.ModuleInputs) error {
	out, err := gen.RenderGoTemplates(module)
	if err != nil {
		return errors.Wrap(err, "cannot generate go stubs -- generator script encountered an error")
	}
	modulePath := filepath.Join(module.ModuleName, "models", "module.go")
	//nolint:gosec
	moduleFile, err := os.Create(modulePath)
	if err != nil {
		return errors.Wrap(err, "cannot generate go stubs -- unable to open file")
	}
	defer utils.UncheckedErrorFunc(moduleFile.Close)
	_, err = moduleFile.Write(out)
	if err != nil {
		return errors.Wrap(err, "cannot generate go stubs -- unable to write to file")
	}

	// run goimports on module file out here
	err = runGoImports(moduleFile)
	if err != nil {
		return errors.Wrap(err, "cannot generate go stubs -- unable to sort imports")
	}

	return nil
}

// run goimports to remove unused imports and add necessary imports.
func runGoImports(moduleFile *os.File) error {
	// check if the gopath is set
	goPath, err := checkGoPath()
	if err != nil {
		return err
	}

	// check if goimports exists in the bin directory
	goImportsPath := fmt.Sprintf("%s/bin/goimports", goPath)
	if _, err := os.Stat(goImportsPath); os.IsNotExist(err) {
		// installing goimports
		installCmd := exec.Command("go", "install", "golang.org/x/tools/cmd/goimports@latest")
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install goimports: %w", err)
		}
	}

	// goimports is installed. Run goimport on the module file
	//nolint:gosec
	formatCmd := exec.Command(goImportsPath, "-w", moduleFile.Name())
	_, err = formatCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run goimports: %w", err)
	}
	return err
}

func checkGoPath() (string, error) {
	goPathCmd := exec.Command("go", "env", "GOPATH")
	goPathBytes, err := goPathCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GOPATH: %w", err)
	}
	goPath := strings.TrimSpace(string(goPathBytes))

	return goPath, err
}

func generatePythonStubs(module common.ModuleInputs) error {
	venvName := ".venv"
	cmd := exec.Command("python3", "--version")
	_, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- python runtime not found")
	}
	cmd = exec.Command("python3", "-m", "venv", venvName)
	_, err = cmd.Output()
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to create python virtual environment")
	}
	defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(venvName) })

	script, err := scripts.ReadFile(filepath.Join(scriptsPath, "generate_stubs.py"))
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to open generator script")
	}
	//nolint:gosec
	cmd = exec.Command(filepath.Join(venvName, "bin", "python3"), "-c", string(script), module.ResourceType,
		module.ResourceSubtype, module.Namespace, module.ModuleName, module.ModelName)
	out, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- generator script encountered an error")
	}

	mainPath := filepath.Join(module.ModuleName, "src", "main.py")
	//nolint:gosec
	mainFile, err := os.Create(mainPath)
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to open file")
	}
	defer utils.UncheckedErrorFunc(mainFile.Close)
	_, err = mainFile.Write(out)
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to write to file")
	}

	return nil
}

func getLatestSDKTag(c *cli.Context, language string, globalArgs globalArgs) (string, error) {
	var repo string
	switch language {
	case python:
		repo = "viam-python-sdk"
	case golang:
		repo = "rdk"
	default:
		return "", errors.New("cannot produce template -- unexpected language was selected")
	}
	debugf(c.App.Writer, globalArgs.Debug, "Getting the latest release tag for %s", repo)
	url := fmt.Sprintf("https://api.github.com/repos/viamrobotics/%s/releases", repo)

	req, err := http.NewRequestWithContext(c.Context, http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get latest %s release", repo)
	}
	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get latest %s release", repo)
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected http GET status: %s", resp.Status)
	}
	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", errors.Wrap(err, "could not decode json")
	}
	releases := result.([]interface{})
	if len(releases) == 0 {
		return "", errors.Errorf("could not get latest %s release", repo)
	}
	latest := releases[0]
	version := latest.(map[string]interface{})["tag_name"].(string)
	debugf(c.App.Writer, globalArgs.Debug, "\tLatest release for %s: %s", repo, version)
	return version, nil
}

func generateCloudBuild(c *cli.Context, module common.ModuleInputs, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Setting cloud build functionality to %s", module.EnableCloudBuild)
	switch module.Language {
	case python:
		if module.EnableCloudBuild {
			err := os.Remove(filepath.Join(module.ModuleName, "run.sh"))
			if err != nil {
				return err
			}
		} else {
			err := os.Remove(filepath.Join(module.ModuleName, "build.sh"))
			if err != nil {
				return err
			}
		}
	case golang:
		if module.EnableCloudBuild {
			err := os.Remove(filepath.Join(module.ModuleName, "run.sh"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createModuleAndManifest(cCtx *cli.Context, c *viamClient, module common.ModuleInputs, globalArgs globalArgs) error {
	var moduleID moduleID
	if module.RegisterOnApp {
		debugf(cCtx.App.Writer, globalArgs.Debug, "Registering module with Viam")
		moduleResponse, err := c.createModule(module.ModuleName, module.OrgID)
		if err != nil {
			return errors.Wrap(err, "failed to register module")
		}
		moduleID, err = parseModuleID(moduleResponse.GetModuleId())
		if err != nil {
			return errors.Wrap(err, "failed to parse module identifier")
		}
	} else {
		debugf(cCtx.App.Writer, globalArgs.Debug, "Creating a local-only module")
		moduleID.name = module.ModuleName
		moduleID.prefix = module.Namespace
	}
	err := renderManifest(cCtx, moduleID.String(), module, globalArgs)
	if err != nil {
		return errors.Wrap(err, "failed to render manifest")
	}
	return nil
}

// Create the meta.json manifest.
func renderManifest(c *cli.Context, moduleID string, module common.ModuleInputs, globalArgs globalArgs) error {
	debugf(c.App.Writer, globalArgs.Debug, "Rendering module manifest")

	visibility := moduleVisibilityPrivate
	if module.IsPublic {
		visibility = moduleVisibilityPublic
	}

	manifest := moduleManifest{
		Schema:      "https://dl.viam.dev/module.schema.json",
		ModuleID:    moduleID,
		Visibility:  visibility,
		Description: fmt.Sprintf("Modular %s %s: %s", module.ResourceSubtype, module.ResourceType, module.ModelName),
		Models: []ModuleComponent{
			{API: module.API, Model: module.ModelTriple},
		},
	}
	switch module.Language {
	case python:
		if module.EnableCloudBuild {
			manifest.Build = &manifestBuildInfo{
				Setup: "./setup.sh",
				Build: "./build.sh",
				Path:  "dist/archive.tar.gz",
				Arch:  []string{"linux/amd64", "linux/arm64"},
			}
			manifest.Entrypoint = "dist/main"
		} else {
			manifest.Entrypoint = "./run.sh"
		}
	case golang:
		if module.EnableCloudBuild {
			manifest.Build = &manifestBuildInfo{
				Setup: "make setup",
				Build: "make module.tar.gz",
				Path:  "bin/module.tar.gz",
				Arch:  []string{"linux/amd64", "linux/arm64"},
			}
			manifest.Entrypoint = fmt.Sprintf("bin/%s", module.ModuleName)
		} else {
			manifest.Entrypoint = "./run.sh"
		}
	}

	if err := writeManifest(filepath.Join(module.ModuleName, defaultManifestFilename), manifest); err != nil {
		return err
	}

	return nil
}
