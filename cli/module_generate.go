package cli

import (
	"embed"
	"encoding/json"
	"regexp"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
)

//go:embed module_generate/scripts/*
var scripts embed.FS

//go:embed module_generate/templates/*
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
	GeneratorVersion string    `json:"generator_version"`
	GeneratedOn      time.Time `json:"generated_on"`

	ModulePascal          string `json:"-"`
	API                   string `json:"-"`
	ResourceSubtypePascal string `json:"-"`
	ModelPascal           string `json:"-"`
	ModelTriple           string `json:"-"`
}

func GenerateModuleAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.generateModuleAction(cCtx)
}

func (c *viamClient) generateModuleAction(cCtx *cli.Context) error {
	newModule := promptUser()

	err := setupDirectories(cCtx, newModule.ModuleName)
	if err != nil {
		return err
	}

	renderCommonFiles(cCtx, newModule)

	err = copyLanguageTemplate(cCtx, newModule.Language, newModule.ModuleName)
	if err != nil {
		return err
	}

	err = renderTemplate(cCtx, newModule)
	if err != nil {
		return err
	}

	err = generateStubs(cCtx, newModule)
	if err != nil {
		return err
	}

	// Create module on app.viam and manifest
	// moduleResponse, err := c.createModule(newModule.ModuleName, newModule.Namespace)
	// if err != nil {
	// 	return err
	// }
	// moduleID, err := parseModuleID(moduleResponse.GetModuleId())
	// if err != nil {
	// 	return err
	// }
	// err = renderManifest(cCtx, moduleID.String(), &newModule)
	// if err != nil {
	// 	return err
	// }

	return nil
}

// Prompt the user for information regarding the module they want to create
// returns the moduleInputs struct that contains the information the user entered
func promptUser() moduleInputs {
	var newModule moduleInputs
	form := huh.NewForm(
		huh.NewGroup(
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
				TitleFunc(func() string {
					if newModule.IsPublic {
						return "Public namespace:"
					}
					return "Organization ID"
				}, &newModule.IsPublic).
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
					huh.NewOption("Sensors Service", "sensors service"),
					huh.NewOption("SLAM Service", "slam service"),
					huh.NewOption("Vision Service", "vision service"),
				).
				Value(&newModule.Resource).WithHeight(25),
			huh.NewInput().
				Title("Set a model name of the resource:").
				Description("The model name can contain only alphanumeric characters, dashes, and underscores.").
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
				Title("Enable cloud build?").
				Description("If enabled, Viam will build your module as executables for a variety of platforms.").
				Value(&newModule.EnableCloudBuild),
			huh.NewConfirm().
				Title("Initialize git repository?").
				Description("Create a git repository for this module.").
				Value(&newModule.InitializeGit),
		),
	).WithHeight(25)
	err := form.Run()
	if err != nil {
		log.Default()
		fmt.Println("uh oh cli is having issues:", err)
		os.Exit(1)
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

	return newModule
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

	// .viam-gen-info
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
				err = os.Mkdir(filepath.Join(moduleName, d.Name()), 0755)
				if err != nil {
					return err
				}
			}
		} else if !strings.HasPrefix(d.Name(), "tmpl") {
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
			debugf(c.App.Writer, c.Bool(debugFlag), "\tSuccesfully created %v", destPath)

		}
		return nil
	})
	if err != nil {
		debugf(c.App.Writer, c.Bool(debugFlag), "Failed to create all %s files", language)
		return err
	}
	return nil
}

// Render all the files in the new directory
func renderTemplate(c *cli.Context, newModule moduleInputs) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Rendering template files")
	languagePath := filepath.Join(templatesPath, newModule.Language)
	tempDir, err := fs.Sub(templates, languagePath)
	if err != nil {
		return err
	}
	err = fs.WalkDir(tempDir, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasPrefix(d.Name(), templatePrefix) {
			destPath := filepath.Join(newModule.ModuleName, strings.ReplaceAll(path, templatePrefix, ""))
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

			err = tmpl.Execute(destFile, newModule)
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
	var err error
	action := func() {
		switch module.Language {
		case "python":
			err = generatePythonStubs(c, module)
		default:
			err = errors.Errorf("Cannot generate stubs for language %s", module.Language)
		}
	}
	spinner.New().Title(fmt.Sprintf("Generating %s stubs...", module.Language)).Action(action).Run()
	return err
}

func generatePythonStubs(c *cli.Context, module moduleInputs) error {
	time.Sleep(5 * time.Second)
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
				Arch:  []string{"linux/amd64", "linux/arm64", "darwin/arm64"},
			}
			manifest.Entrypoint = "dist/main"
		} else {
			manifest.Entrypoint = "./run.sh"
		}
	}

	if err := writeManifest(defaultManifestFilename, manifest); err != nil {
		return err
	}

	return nil
}
