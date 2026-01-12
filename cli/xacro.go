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

	// 4. Convert input to absolute path
	absInputFile := args.Input
	if !filepath.IsAbs(args.Input) {
		absInputFile = filepath.Join(cwd, args.Input)
	}

	// Validate input file exists
	if _, err := os.Stat(absInputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", args.Input)
	}

	// 5. Prepare relative paths
	relInputFile, err := filepath.Rel(cwd, absInputFile)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for input: %w", err)
	}

	// 6. Discover dependent packages
	dependentPkgs, err := discoverDependentPackages(absInputFile, cwd, pkgName)
	if err != nil {
		return fmt.Errorf("failed to discover dependent packages: %w", err)
	}
	if len(dependentPkgs) > 0 {
		printf(c.App.Writer, "Found dependent packages: %s\n", strings.Join(getPackageNames(dependentPkgs), ", "))
	}

	// 7. Process arguments (convert file paths to container paths)
	dockerArgs, err := processXacroArgs(args.Args, cwd, pkgName)
	if err != nil {
		return fmt.Errorf("failed to process xacro arguments: %w", err)
	}

	// 8. Build Docker command
	dockerCmd := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, args.DockerImg, dependentPkgs)

	if args.DryRun {
		printf(c.App.Writer, "Dry run - would execute:\n")
		printf(c.App.Writer, "docker %s\n", strings.Join(dockerCmd, " "))
		return nil
	}

	printf(c.App.Writer, "Processing with Docker...\n")

	// 9. Run Docker
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "docker", dockerCmd...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xacro processing failed: %w\nStderr: %s", err, stderr.String())
	}

	// 10. Write output file as-is from xacro (no URI transformation)
	output := stdout.String()

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

// packageInfo holds information about a ROS package.
type packageInfo struct {
	Name string
	Path string
}

// discoverDependentPackages scans xacro files for $(find package_name) and locates them.
func discoverDependentPackages(xacroPath string, currentPkgDir string, currentPkgName string) ([]packageInfo, error) {
	// Scan the xacro file and any includes to find $(find ...) patterns
	pkgNames := make(map[string]bool)
	if err := scanXacroForDependencies(xacroPath, currentPkgDir, pkgNames); err != nil {
		return nil, err
	}

	// Remove the current package from dependencies
	delete(pkgNames, currentPkgName)

	// Look for these packages in the parent directory
	parentDir := filepath.Dir(currentPkgDir)
	var packages []packageInfo

	for pkgName := range pkgNames {
		pkgPath := filepath.Join(parentDir, pkgName)
		if info, err := os.Stat(pkgPath); err == nil && info.IsDir() {
			// Verify it has a package.xml
			if _, err := os.Stat(filepath.Join(pkgPath, "package.xml")); err == nil {
				packages = append(packages, packageInfo{Name: pkgName, Path: pkgPath})
			}
		}
	}

	return packages, nil
}

// scanXacroForDependencies recursively scans xacro files for $(find package_name) references.
func scanXacroForDependencies(xacroPath string, currentPkgDir string, pkgNames map[string]bool) error {
	data, err := os.ReadFile(xacroPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Find all $(find package_name) patterns
	// Pattern: $(find package_name)
	for {
		start := strings.Index(content, "$(find ")
		if start == -1 {
			break
		}
		content = content[start+7:] // skip "$(find "
		end := strings.Index(content, ")")
		if end == -1 {
			break
		}
		pkgName := strings.TrimSpace(content[:end])
		// Extract just the package name (before any /)
		if idx := strings.Index(pkgName, "/"); idx != -1 {
			pkgName = pkgName[:idx]
		}
		pkgNames[pkgName] = true
		content = content[end+1:]
	}

	// Also scan included files
	content = string(data)
	for {
		start := strings.Index(content, "filename=\"")
		if start == -1 {
			break
		}
		content = content[start+10:] // skip 'filename="'
		end := strings.Index(content, "\"")
		if end == -1 {
			break
		}
		filename := content[:end]

		var includePath string

		// Handle $(find package_name)/path/file.xacro patterns
		if strings.HasPrefix(filename, "$(find ") {
			// Extract package name and relative path
			closeIdx := strings.Index(filename, ")")
			if closeIdx != -1 {
				// pkgRef := filename[7:closeIdx]
				remainingPath := ""
				if closeIdx+1 < len(filename) && filename[closeIdx+1] == '/' {
					remainingPath = filename[closeIdx+2:]
				}
				// If it references the current package's directory, scan it
				includePath = filepath.Join(currentPkgDir, remainingPath)
			}
		} else if !filepath.IsAbs(filename) {
			// Relative path
			includePath = filepath.Join(filepath.Dir(xacroPath), filename)
		}

		// If we resolved a path and it exists, recursively scan it
		if includePath != "" {
			if _, err := os.Stat(includePath); err == nil {
				scanXacroForDependencies(includePath, currentPkgDir, pkgNames)
			}
		}

		content = content[end+1:]
	}

	return nil
}

// getPackageNames extracts package names from packageInfo slice.
func getPackageNames(packages []packageInfo) []string {
	names := make([]string, len(packages))
	for i, pkg := range packages {
		names[i] = pkg.Name
	}
	return names
}

// buildDockerXacroCommand builds the docker command to run xacro.
func buildDockerXacroCommand(pkgName, cwd, relInputFile string, dockerArgs []string, dockerImg string, dependentPkgs []packageInfo) []string {
	if dockerImg == "" {
		dockerImg = "osrf/ros:humble-desktop"
	}

	// Build the list of packages to register
	allPkgs := []string{pkgName}
	for _, pkg := range dependentPkgs {
		allPkgs = append(allPkgs, pkg.Name)
	}

	// Build bash script that registers all packages
	var scriptParts []string
	scriptParts = append(scriptParts, "apt-get update -qq > /dev/null")
	scriptParts = append(scriptParts, "apt-get install -y -qq ros-humble-xacro > /dev/null")
	scriptParts = append(scriptParts, "mkdir -p /opt/ros/humble/share/ament_index/resource_index/packages")
	for _, pkg := range allPkgs {
		scriptParts = append(scriptParts, fmt.Sprintf("touch /opt/ros/humble/share/ament_index/resource_index/packages/%s", pkg))
	}
	scriptParts = append(scriptParts, "source /opt/ros/humble/setup.bash")
	scriptParts = append(scriptParts, fmt.Sprintf("ros2 run xacro xacro %s %s", relInputFile, strings.Join(dockerArgs, " ")))

	bashScript := strings.Join(scriptParts, " && \\\n")

	// Build docker command with volume mounts for all packages
	dockerCmd := []string{"run", "--rm"}

	// Mount main package
	dockerCmd = append(dockerCmd, "-v", fmt.Sprintf("%s:/opt/ros/humble/share/%s", cwd, pkgName))

	// Mount dependent packages
	for _, pkg := range dependentPkgs {
		dockerCmd = append(dockerCmd, "-v", fmt.Sprintf("%s:/opt/ros/humble/share/%s", pkg.Path, pkg.Name))
	}

	// Set working directory and run
	dockerCmd = append(dockerCmd,
		"-w", fmt.Sprintf("/opt/ros/humble/share/%s", pkgName),
		dockerImg,
		"bash", "-c", bashScript,
	)

	return dockerCmd
}
