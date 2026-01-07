package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	xacroFlagArgs       = "args"
	xacroFlagDryRun     = "dry-run"
	xacroFlagDockerImg  = "docker-image"
	xacroFlagPackageXML = "package-xml"
)

type xacroConvertArgs struct {
	Input      string
	Output     string
	Args       []string
	DryRun     bool
	DockerImg  string
	PackageXML string
}

func xacroConvertAction(c *cli.Context, args xacroConvertArgs) error {
	// 1. Validate package.xml exists
	packageXMLPath := args.PackageXML
	if packageXMLPath == "" {
		packageXMLPath = "package.xml"
	}

	if _, err := os.Stat(packageXMLPath); os.IsNotExist(err) {
		return fmt.Errorf("package.xml not found at %s (specify with --package-xml if in a different location)", packageXMLPath)
	}

	// 2. Detect package name
	pkgName, err := extractPackageName(packageXMLPath)
	if err != nil {
		return fmt.Errorf("failed to detect package name: %w", err)
	}
	printf(c.App.Writer, "Detected package: %s\n", pkgName)

	// 3. Get current directory (or package.xml directory)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// If package.xml is in a different directory, use that as base
	if packageXMLPath != "package.xml" {
		cwd = filepath.Dir(packageXMLPath)
	}

	// 4. Validate input file exists
	if _, err := os.Stat(args.Input); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", args.Input)
	}

	// 5. Prepare relative paths
	relInputFile, err := filepath.Rel(cwd, args.Input)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for input: %w", err)
	}

	// 6. Process arguments (convert file paths to container paths)
	dockerArgs, err := processXacroArgs(args.Args, cwd, pkgName)
	if err != nil {
		return fmt.Errorf("failed to process xacro arguments: %w", err)
	}

	// 7. Build Docker command
	dockerCmd := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, args.DockerImg)

	if args.DryRun {
		printf(c.App.Writer, "Dry run - would execute:\n")
		printf(c.App.Writer, "docker %s\n", strings.Join(dockerCmd, " "))
		return nil
	}

	printf(c.App.Writer, "Processing with Docker...\n")

	// 8. Run Docker
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "docker", dockerCmd...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xacro processing failed: %w\nStderr: %s", err, stderr.String())
	}

	// 9. Post-process output (fix paths to be relative)
	output := stdout.String()
	output = strings.ReplaceAll(output, fmt.Sprintf("package://%s/", pkgName), "")

	// 10. Write output file
	if err := os.WriteFile(args.Output, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	printf(c.App.Writer, "Success! Generated: %s\n", args.Output)
	return nil
}

// extractPackageName extracts the package name from package.xml.
func extractPackageName(packageXMLPath string) (string, error) {
	data, err := os.ReadFile(packageXMLPath)
	if err != nil {
		return "", err
	}

	// Simple extraction: find <name>...</name>
	content := string(data)
	start := strings.Index(content, "<name>")
	end := strings.Index(content, "</name>")
	if start == -1 || end == -1 {
		return "", fmt.Errorf("could not find <name> tag in package.xml")
	}

	return strings.TrimSpace(content[start+6 : end]), nil
}

// processXacroArgs processes xacro arguments, converting file paths to container paths.
func processXacroArgs(args []string, cwd, pkgName string) ([]string, error) {
	dockerArgs := make([]string, 0, len(args))

	for _, arg := range args {
		if strings.Contains(arg, ":=") {
			parts := strings.SplitN(arg, ":=", 2)
			key := parts[0]
			value := parts[1]

			// Check if value is a file that exists
			if _, err := os.Stat(value); err == nil {
				// Convert to container absolute path
				relPath, err := filepath.Rel(cwd, value)
				if err != nil {
					return nil, fmt.Errorf("failed to compute relative path for %s: %w", value, err)
				}
				absPath := fmt.Sprintf("/opt/ros/humble/share/%s/%s", pkgName, relPath)
				dockerArgs = append(dockerArgs, fmt.Sprintf("%s:=%s", key, absPath))
				continue
			}
		}
		dockerArgs = append(dockerArgs, arg)
	}

	return dockerArgs, nil
}

// buildDockerXacroCommand builds the docker command to run xacro.
func buildDockerXacroCommand(pkgName, cwd, relInputFile string, dockerArgs []string, dockerImg string) []string {
	if dockerImg == "" {
		dockerImg = "osrf/ros:humble-desktop"
	}

	bashScript := fmt.Sprintf(`
apt-get update -qq > /dev/null && \
apt-get install -y -qq ros-humble-xacro > /dev/null && \
mkdir -p /opt/ros/humble/share/ament_index/resource_index/packages && \
touch /opt/ros/humble/share/ament_index/resource_index/packages/%s && \
source /opt/ros/humble/setup.bash && \
ros2 run xacro xacro %s %s
`, pkgName, relInputFile, strings.Join(dockerArgs, " "))

	return []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/opt/ros/humble/share/%s", cwd, pkgName),
		"-w", fmt.Sprintf("/opt/ros/humble/share/%s", pkgName),
		dockerImg,
		"bash", "-c", bashScript,
	}
}
