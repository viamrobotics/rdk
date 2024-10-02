package cli

import (
	"embed"
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
)

//go:embed module_generate/scripts
var scripts embed.FS

//go:embed all:module_generate/templates/*
var templates embed.FS

const (
	version        = "0.1.0"
	basePath       = "module_generate"
	templatePrefix = "tmpl-"
)

var (
	scriptsPath   = filepath.Join(basePath, "scripts")
	templatesPath = filepath.Join(basePath, "templates")
)

// module contains the necessary information to fill out template files
type moduleInputs struct {
	ModuleName       string    `json:"module_name"`
	IsPublic         bool      `json:"-"`
	Namespace        string    `json:"namespace"`
	Language         string    `json:"language"`
	Resource         string    `json:"-"`
	ResourceType     string    `json:"resource_type"`
	ResourceSubtype  string    `json:"resource_subtype"`
	ModelName        string    `json:"model_name"`
	EnableCloudBuild bool      `json:"enable_cloud_build"`
	InitializeGit    bool      `json:"initialize_git"`
	RegisterOnApp    bool      `json:"-"`
	GeneratorVersion string    `json:"generator_version"`
	GeneratedOn      time.Time `json:"generated_on"`

	ModulePascal          string `json:"-"`
	API                   string `json:"-"`
	ResourceSubtypePascal string `json:"-"`
	ModelPascal           string `json:"-"`
	ModelTriple           string `json:"-"`

	SDKVersion string `json:"-"`
}

func GenerateModuleAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.generateModuleAction(cCtx)
}

func (c *viamClient) generateModuleAction(cCtx *cli.Context) error {
	var newModule *moduleInputs
	var err error
	resource := cCtx.String(moduleFlagResource)
	if resource != "" {
		resourceSubtype := strings.Fields(resource)[0]
		resourceType := strings.Fields(resource)[1]
		newModule = &moduleInputs{
			ModuleName:       "my-module",
			IsPublic:         false,
			Namespace:        "my-org",
			Language:         "python",
			Resource:         resource,
			ResourceType:     resourceType,
			ResourceSubtype:  resourceSubtype,
			ModelName:        "my-model",
			EnableCloudBuild: false,
			InitializeGit:    false,
			GeneratorVersion: "0.1.0",
			GeneratedOn:      time.Now().UTC(),
	
			ModulePascal:          "MyModule",
			API:                   fmt.Sprintf("rdk:%s:ss", resourceType, resourceSubtype),
			ResourceSubtypePascal: strings.ToUpper(string(resourceSubtype[0])) + resourceSubtype[1:],
			ModelPascal:           "MyModel",
			ModelTriple:           "my-org:my-module:my-model",
	
			SDKVersion: "0.0.0",
		}
	} else {
		newModule, err = promptUser()
	}
	if err != nil {
		return err
	}

	s := spinner.New()
	var fatalError error
	nonFatalError := false
	action := func() {
		s.Title("Getting latest release...")
		version, err := getLatestSDKTag(cCtx, newModule.Language)
		if err != nil {
			fatalError = err
			return
		}
		newModule.SDKVersion = version[1:]

		s.Title("Setting up module directory...")
		if err = setupDirectories(cCtx, newModule.ModuleName); err != nil {
			fatalError = err
			return
		}

		s.Title("Creating module and generating manifest...")
		if err = createModuleAndManifest(cCtx, c, *newModule); err != nil {
			fatalError = err
			return
		}

		s.Title("Rendering common files...")
		if err = renderCommonFiles(cCtx, *newModule); err != nil {
			fatalError = err
			return
		}

		s.Title(fmt.Sprintf("Copying %s files...", newModule.Language))
		if err = copyLanguageTemplate(cCtx, newModule.Language, newModule.ModuleName); err != nil {
			fatalError = err
			return
		}

		s.Title("Rendering template...")
		if err = renderTemplate(cCtx, *newModule); err != nil {
			fatalError = err
			return
		}

		s.Title(fmt.Sprintf("Generating %s stubs...", newModule.Language))
		if err = generateStubs(cCtx, *newModule); err != nil {
			warningf(cCtx.App.ErrWriter, err.Error())
			nonFatalError = true
		}

		s.Title("Generating cloud build requirements...")
		if err = generateCloudBuild(cCtx, *newModule); err != nil {
			warningf(cCtx.App.ErrWriter, err.Error())
			nonFatalError = true
		}
	}

	if cCtx.Bool(debugFlag) {
		action()
	} else {
		s.Action(action)
		s.Run()
	}

	if fatalError != nil {
		os.RemoveAll(newModule.ModuleName)
		return errors.Wrap(fatalError, "unable to generate module")
	}

	if nonFatalError {
		return errors.New(fmt.Sprintf("some steps of module generation failed, incomplete module located at %s", newModule.ModuleName))
	}

	printf(cCtx.App.Writer, "Module successfully generated at %s", newModule.ModuleName)
	return nil
}

// Prompt the user for information regarding the module they want to create
// returns the moduleInputs struct that contains the information the user entered
func promptUser() (*moduleInputs, error) {
	var newModule moduleInputs
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
						return errors.New("Module name must not be empty!")
					}
					match, err := regexp.MatchString("^[a-z0-9]+(?:[_-][a-z0-9]+)*$", s)
					if !match || err != nil {
						return errors.New("Module names can only contain alphanumeric characters, dashes, and underscores!")
					}
					if _, err := os.Stat(s); err == nil {
						return errors.New("This module directory already exists!")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Specify the language for the module:").
				Options(
					huh.NewOption("Python", "python"),
					// huh.NewOption("Go", "go"),
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
						return errors.New("Namespace or org ID must not be empty!")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Select a resource to be added to the module:").
				Options(
					huh.NewOption("Arm Component", "arm component"),
					huh.NewOption("Audio Input Component", "audio_input component"),
					huh.NewOption("Base Component", "base component"),
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
					huh.NewOption("Sensors Service", "sensors service"),
					huh.NewOption("SLAM Service", "slam service"),
					huh.NewOption("Vision Service", "vision service"),
				).
				Value(&newModule.Resource).WithHeight(25),
			huh.NewInput().
				Title("Set a model name of the resource:").
				Description("This is the name of the new resource model that your module will provide.\nThe model name can contain only alphanumeric characters, dashes, and underscores.").
				Placeholder("my-model").
				Value(&newModule.ModelName).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("Model name must not be empty!")
					}
					match, err := regexp.MatchString("^[a-z0-9]+(?:[_-][a-z0-9]+)*$", s)
					if !match || err != nil {
						return errors.New("Module names can only contain alphanumeric characters, dashes, and underscores!")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Enable cloud build").
				Description("If enabled, this will generate GitHub workflows to build your module.").
				Value(&newModule.EnableCloudBuild),
			huh.NewConfirm().
				Title("Register module").
				Description("Register this module with Viam.\nIf selected, this will associate the module with your organization.\nOtherwise, this will be a local-only module.").
				Value(&newModule.RegisterOnApp),
		),
	).WithHeight(25).WithWidth(88)
	err := form.Run()
	if err != nil {
		return nil, errors.Wrap(err, "encountered an error generating module")
	}

	// Fill in additional info
	newModule.GeneratedOn = time.Now().UTC()
	newModule.GeneratorVersion = version
	newModule.ResourceSubtype = strings.Split(newModule.Resource, " ")[0]
	newModule.ResourceType = strings.Split(newModule.Resource, " ")[1]

	titleCaser := cases.Title(language.Und)
	replacer := strings.NewReplacer("_", "", "-", "")
	newModule.ModulePascal = replacer.Replace(titleCaser.String(newModule.ModuleName))
	newModule.API = fmt.Sprintf("rdk:%s:%s", newModule.ResourceType, newModule.ResourceSubtype)
	newModule.ResourceSubtypePascal = replacer.Replace(titleCaser.String(newModule.ResourceSubtype))
	newModule.ModelPascal = replacer.Replace(titleCaser.String(newModule.ModelName))
	newModule.ModelTriple = fmt.Sprintf("%s:%s:%s", newModule.Namespace, newModule.ModuleName, newModule.ModelName)

	return &newModule, nil
}

// Creates a new directory with moduleName
func setupDirectories(c *cli.Context, moduleName string) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Setting up directories")
	err := os.Mkdir(moduleName, 0755)
	if err != nil {
		return err
	}
	return nil
}

func renderCommonFiles(c *cli.Context, module moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Rendering common files")

	// Render .viam-gen-info
	infoBytes, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return err
	}

	infoFilePath := filepath.Join(module.ModuleName, ".viam-gen-info")
	infoFile, err := os.Create(infoFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", infoFilePath)
	}
	defer infoFile.Close()

	if _, err := infoFile.Write(infoBytes); err != nil {
		return errors.Wrapf(err, "failed to write generator info to %s", infoFilePath)
	}

	// Render workflows for cloud build
	if module.EnableCloudBuild {
		debugf(c.App.Writer, c.Bool(debugFlag), "\tCreating cloud build workflow")
		destWorkflowPath := filepath.Join(module.ModuleName, ".github")
		if err = os.Mkdir(destWorkflowPath, 0755); err != nil {
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
					debugf(c.App.Writer, c.Bool(debugFlag), "\t\tCopying %s directory", d.Name())
					err = os.Mkdir(filepath.Join(destWorkflowPath, path), 0755)
					if err != nil {
						return err
					}
				}
			} else if !strings.HasPrefix(d.Name(), templatePrefix) {
				debugf(c.App.Writer, c.Bool(debugFlag), "\t\tCopying file %s", path)
				srcFile, err := templates.Open(filepath.Join(workflowPath, path))
				if err != nil {
					return errors.Wrapf(err, "error opening file %s", srcFile)
				}
				defer srcFile.Close()

				destPath := filepath.Join(destWorkflowPath, path)
				destFile, err := os.Create(destPath)
				if err != nil {
					return errors.Wrapf(err, "failed to create file %s", destPath)
				}
				defer destFile.Close()

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

// copyLanguageTemplate copies the files from templates/language directory into the moduleName root directory
func copyLanguageTemplate(c *cli.Context, language string, moduleName string) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Creating %s template files", language)
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
				debugf(c.App.Writer, c.Bool(debugFlag), "\tCopying %s directory", d.Name())
				err = os.Mkdir(filepath.Join(moduleName, path), 0755)
				if err != nil {
					return err
				}
			}
		} else if !strings.HasPrefix(d.Name(), templatePrefix) {
			debugf(c.App.Writer, c.Bool(debugFlag), "\tCopying file %s", path)
			srcFile, err := templates.Open(filepath.Join(languagePath, path))
			if err != nil {
				return errors.Wrapf(err, "error opening file %s", srcFile)
			}
			defer srcFile.Close()

			destPath := filepath.Join(moduleName, path)
			destFile, err := os.Create(destPath)
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", destPath)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return errors.Wrapf(err, "error executing template for %s", destPath)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to render all %s files", language)
	}
	return nil
}

// Render all the files in the new directory
func renderTemplate(c *cli.Context, module moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Rendering template files")
	languagePath := filepath.Join(templatesPath, module.Language)
	tempDir, err := fs.Sub(templates, languagePath)
	if err != nil {
		return err
	}
	err = fs.WalkDir(tempDir, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasPrefix(d.Name(), templatePrefix) {
			destPath := filepath.Join(module.ModuleName, strings.ReplaceAll(path, templatePrefix, ""))
			debugf(c.App.Writer, c.Bool(debugFlag), "\tRendering file %s", destPath)

			tFile, err := templates.Open(filepath.Join(languagePath, path))
			if err != nil {
				return err
			}
			defer tFile.Close()
			tBytes, err := io.ReadAll(tFile)
			if err != nil {
				return err
			}

			tmpl, err := template.New(path).Parse(string(tBytes))
			if err != nil {
				return err
			}

			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			err = tmpl.Execute(destFile, module)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Generate stubs for the resource
func generateStubs(c *cli.Context, module moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Generating %s stubs", module.Language)
	switch module.Language {
	case "python":
		return generatePythonStubs(module)
	default:
		return errors.Errorf("cannot generate stubs for language %s", module.Language)
	}
}

func generatePythonStubs(module moduleInputs) error {
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
	defer os.RemoveAll(venvName)

	script, err := scripts.ReadFile(filepath.Join(scriptsPath, "generate_stubs.py"))
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to open generator script")
	}
	cmd = exec.Command(filepath.Join(venvName, "bin", "python3"), "-c", string(script), module.ResourceType, module.ResourceSubtype, module.Namespace, module.ModuleName, module.ModelName)
	out, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- generator script encountered an error")
	}

	mainPath := filepath.Join(module.ModuleName, "src", "main.py")
	mainFile, err := os.Create(mainPath)
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to open file")
	}
	defer mainFile.Close()
	_, err = mainFile.Write(out)
	if err != nil {
		return errors.Wrap(err, "cannot generate python stubs -- unable to write to file")
	}

	return nil
}

func getLatestSDKTag(c *cli.Context, language string) (string, error) {
	var repo string
	if language == "python" {
		repo = "viam-python-sdk"
	}
	debugf(c.App.Writer, c.Bool(debugFlag), "Getting the latest release tag for %s", repo)
	url := fmt.Sprintf("https://api.github.com/repos/viamrobotics/%s/releases", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "cannot get latest %s release", repo)
	}
	defer resp.Body.Close()
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
	debugf(c.App.Writer, c.Bool(debugFlag), "\tLatest release for %s: %s", repo, version)
	return version, nil
}

func generateCloudBuild(c *cli.Context, module moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Setting cloud build functionality to %s", module.EnableCloudBuild)
	switch module.Language {
	case "python":
		if module.EnableCloudBuild {
			os.Remove(filepath.Join(module.ModuleName, "run.sh"))
		} else {
			os.Remove(filepath.Join(module.ModuleName, "build.sh"))
		}
	}
	return nil
}

func createModuleAndManifest(cCtx *cli.Context, c *viamClient, module moduleInputs) error {
	var moduleId moduleID
	if module.RegisterOnApp {
		debugf(cCtx.App.Writer, cCtx.Bool(debugFlag), "Registering module with Viam")
		orgID := module.Namespace
		_, err := uuid.Parse(module.Namespace)
		if err != nil {
			org, err := resolveOrg(c, module.Namespace, "")
			if err != nil {
				return errors.Wrapf(err, "failed to resolve organization from namespace %s", module.Namespace)
			}
			orgID = org.GetId()
		}
		moduleResponse, err := c.createModule(module.ModuleName, orgID)
		if err != nil {
			return errors.Wrap(err, "failed to register module")
		}
		moduleId, err = parseModuleID(moduleResponse.GetModuleId())
		if err != nil {
			return errors.Wrap(err, "failed to parse module identifier")
		}
	} else {
		debugf(cCtx.App.Writer, cCtx.Bool(debugFlag), "Creating a local-only module")
		moduleId.name = module.ModuleName
		moduleId.prefix = module.Namespace
	}
	err := renderManifest(cCtx, moduleId.String(), module)
	if err != nil {
		return errors.Wrap(err, "failed to render manifest")
	}
	return nil
}

// Create the meta.json manifest
func renderManifest(c *cli.Context, moduleID string, module moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Rendering module manifest")

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

	if module.Language == "python" {
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
	}

	if err := writeManifest(filepath.Join(module.ModuleName, defaultManifestFilename), manifest); err != nil {
		return err
	}

	return nil
}
