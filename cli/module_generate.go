package cli

import (
	"embed"
	_ "embed"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

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
)

//go:embed templates/*
var templates embed.FS

const version = "0.1.0"

// module contains the necessary information to fill out template files
type moduleInputs struct {
	ModuleName        string
	Namespace         string
	Language          string
	Resource          string
	ResourceType      string
	ResourceSubtype   string
	ModelName         string
	InitializeGitBool bool
	InitializeGit     string
	GeneratorVersion  string
	GeneratedOn       string
}

// func debugf(c.App.Writer, c.Bool(debugFlag) bool, format string, a ...interface{}) {
// 	if !c.App.Writer, c.Bool(debugFlag) {
// 		return
// 	}
// 	if _, err := color.New(color.Bold, color.FgHiBlack).Fprint(os.Stdout, "Debug: "); err != nil {
// 		log.Fatal(err)
// 	}
// 	fmt.Printf(format+"\n", a...)
// }

// toKebabCase converts str to kebab case
func toKebabCase(str string) string {
	return strings.ToLower(strings.ReplaceAll(str, " ", "-"))
}

// runCLI runs a CLI that prompts the user for information regarding the module they want to create
// returns the moduleInputs struct that contains the information the user entered
func runCLI() moduleInputs {
	var newModule moduleInputs
	// c.App.Writer, c.Bool(debugFlag) := ""

	// debugForm := huh.NewForm(
	// 	huh.NewGroup(
	// 		huh.NewInput().
	// 			Title("Generate a new modular resource!").
	// 			Value(&c.App.Writer, c.Bool(debugFlag)).
	// 			Description("add --debug flag for debug mode. otherwise press enter to continue").
	// 			Validate(func(s string) error {
	// 				if s != "--debug" && s != "" {
	// 					return errors.New("not a valid flag")
	// 				}
	// 				return nil
	// 			}).WithHeight(4),
	// 	),
	// )
	// err := debugForm.Run()
	// if err != nil {
	// 	log.Default()
	// 	fmt.Println("uh oh cli is having issues:", err)
	// 	os.Exit(1)
	// }

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Set a module name:").
				Value(&newModule.ModuleName).
				Placeholder("my-module").
				Suggestions([]string{"my-module"}).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("Module name must not be empty!")
					}
					if _, err := os.Stat(s); err == nil {
						return errors.New("This module directory already exists!")
					}
					if _, err := os.Stat(toKebabCase(s)); err == nil {
						return errors.New("This module directory already exists!")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Specify the language for the module:").
				Options(
					huh.NewOption("Python", "python"),
					huh.NewOption("Go", "go"),
				).
				Value(&newModule.Language),
			huh.NewInput().
				Title("Namespace or organization ID:").
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
				Value(&newModule.ModelName).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("Model name must not be empty!")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Initalize git workflow?").
				Value(&newModule.InitializeGitBool),
		),
	).WithHeight(25)
	err := form.Run()
	if err != nil {
		log.Default()
		fmt.Println("uh oh cli is having issues:", err)
		os.Exit(1)
	}

	// fill in other info
	newModule.ModuleName = toKebabCase(newModule.ModuleName)
	newModule.GeneratedOn = time.Now().UTC().Format("2006-01-02 15:04:05 MST")
	newModule.GeneratorVersion = version
	newModule.ResourceSubtype = strings.Split(newModule.Resource, " ")[0]
	newModule.ResourceType = strings.Split(newModule.Resource, " ")[1]
	if newModule.InitializeGitBool {
		newModule.InitializeGit = "y"
	} else {
		newModule.InitializeGit = "n"
	}

	return newModule
}

// setUpDirectories creates a new directory with moduleName and a subdirectory src
func setUpDirectories(c *cli.Context, moduleName string) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Setting up directories")

	err := os.Mkdir(moduleName, 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(filepath.Join(moduleName, "src"), 0755)
	if err != nil {
		return err
	}
	debugf(c.App.Writer, c.Bool(debugFlag), "Successfully set up %s directory and %s/src directory", moduleName, moduleName)

	return nil
}

// copyLanguageTemplate copies the files from templates/language directory into the moduleName root directory
func copyLanguageTemplate(c *cli.Context, language string, moduleName string) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Creating %s template files...", language)
	templateLangPath := filepath.Join("templates", language)
	if language != "python" {
		return errors.New("this language template directory does not exist yet")
	}

	subdir, err := fs.Sub(templates, templateLangPath)
	if err != nil {
		return err
	}

	err = fs.WalkDir(subdir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			srcFile, err := templates.Open(filepath.Join(templateLangPath, path))
			if err != nil {
				return errors.Errorf("Error opening file %s: %v", srcFile, err)
			}
			defer srcFile.Close()

			destPath := filepath.Join(moduleName, path)
			destFile, err := os.Create(destPath)
			if err != nil {
				return errors.Errorf("Failed to create file %s: %v", destPath, err)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return errors.Errorf("Error executing template for %s: %v", destPath, err)
			}
			debugf(c.App.Writer, c.Bool(debugFlag), "Succesfully created %v", destPath)

		}
		return nil
	})
	if err != nil {
		debugf(c.App.Writer, c.Bool(debugFlag), "Failed to create all %s files", language)
		return err
	}
	return nil
}

// generate creates the ".viam-gen-info" and "meta.json" files in the module directory based on newModule
func generate(c *cli.Context, newModule moduleInputs) error {
	tmpl, err := template.New("files").ParseFS(templates, filepath.Join("templates", ".viam-gen-info"), filepath.Join("templates", "meta.json"))
	if err != nil {
		return errors.Wrap(err, "Error parsing file system")
	}
	for _, name := range []string{"meta.json", ".viam-gen-info"} {
		file, err := os.Create(filepath.Join(newModule.ModuleName, name))
		if err != nil {
			debugf(c.App.Writer, c.Bool(debugFlag), "Failed to create file: %v", err)
			return err
		}
		defer file.Close()
		err = tmpl.ExecuteTemplate(file, name, newModule)
		if err != nil {
			debugf(c.App.Writer, c.Bool(debugFlag), "Error executing template: %v", err)
			return err
		}
		debugf(c.App.Writer, c.Bool(debugFlag), "Successfully generated %v", name)
	}

	return nil
}

func GenerateModuleAction(c *cli.Context) error {
	debugf(c.App.Writer, c.Bool(debugFlag), "Debug mode is on")

	newModule := runCLI()

	err := setUpDirectories(c, newModule.ModuleName)
	if err != nil {
		fmt.Printf("Error setting up directories: %v \n", err)
	}

	err = copyLanguageTemplate(c, newModule.Language, newModule.ModuleName)
	if err != nil {
		fmt.Printf("Error creating language templates: %v\n", err)
	}

	err = generate(c, newModule)
	if err != nil {
		fmt.Printf("Error generating files: %v \n", err)
	}
	debugf(c.App.Writer, c.Bool(debugFlag), "module should be generated")

	//executing python script
	// debugf(c.App.Writer, c.Bool(debugFlag), "Running python script..")
	// cmd := exec.Command("python3", "-c", string(pyScript), newModule.ModuleName+"/.viam-gen-info")
	// out, err := cmd.Output()
	// if err != nil {
	// 	debugf(c.App.Writer, c.Bool(debugFlag), "Could not run python script: %v", err)
	// }
	// fmt.Println(string(out))
	return nil

}
