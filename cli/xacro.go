package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	xacroFindPrefix      = "$(find "
	defaultROSDistro     = "humble"
	rosSharePathPattern  = "/opt/ros/%s/share"
	rosSetupPathPattern  = "/opt/ros/%s/setup.bash"
	rosXacroPackPattern  = "ros-%s-xacro"
	amentIndexPathSuffix = "/ament_index/resource_index/packages"

	xacroFilenamePrefix = "filename=\""
	xacroArgSeparator   = ":="
	fileOutputPerm      = 0o644
	packageXMLFilename  = "package.xml"

	dockerExecutable = "docker"
	dockerRunCmd     = "run"
	dockerRmFlag     = "--rm"
)

type xacroConvertArgs struct {
	InputFile           string
	OutputFile          string
	Args                []string
	DryRun              bool
	DockerImage         string
	PackageXML          string
	CollapseFixedJoints bool
	InstallPackages     bool
	RosDistro           string
}

// Note: XML-tagged struct fields must be exported for encoding/xml to work.
type urdfRobot struct {
	XMLName xml.Name    `xml:"robot"`
	Name    string      `xml:"name,attr"`
	Links   []urdfLink  `xml:"link"`
	Joints  []urdfJoint `xml:"joint"`
}

type urdfLink struct {
	XMLName  xml.Name `xml:"link"`
	Name     string   `xml:"name,attr"`
	InnerXML string   `xml:",innerxml"` // Preserve unparsed content
}

type urdfJoint struct {
	XMLName  xml.Name    `xml:"joint"`
	Name     string      `xml:"name,attr"`
	Type     string      `xml:"type,attr"`
	Parent   urdfLinkRef `xml:"parent"`
	Child    urdfLinkRef `xml:"child"`
	Origin   *urdfOrigin `xml:"origin"`
	Axis     *urdfAxis   `xml:"axis"`
	Limit    *urdfLimit  `xml:"limit"`
	InnerXML string      `xml:",innerxml"` // Preserve unparsed content
}

type urdfLinkRef struct {
	Link string `xml:"link,attr"`
}

type urdfOrigin struct {
	XYZ string `xml:"xyz,attr"`
	RPY string `xml:"rpy,attr"`
}

type urdfAxis struct {
	XYZ string `xml:"xyz,attr"`
}

type urdfLimit struct {
	Lower    string `xml:"lower,attr"`
	Upper    string `xml:"upper,attr"`
	Effort   string `xml:"effort,attr"`
	Velocity string `xml:"velocity,attr"`
}

// XacroConvertAction converts a xacro file to URDF format.
func XacroConvertAction(c *cli.Context, args xacroConvertArgs) error {
	if _, err := exec.LookPath(dockerExecutable); err != nil {
		return fmt.Errorf("%s not found - please install Docker to use xacro conversion: %w", dockerExecutable, err)
	}

	packageXMLPath := args.PackageXML
	if packageXMLPath == "" {
		packageXMLPath = packageXMLFilename
	}

	if _, err := os.Stat(packageXMLPath); os.IsNotExist(err) {
		return fmt.Errorf("%s not found at %s (specify with --package-xml if in a different location)", packageXMLFilename, packageXMLPath)
	}

	pkgName, err := extractPackageName(packageXMLPath)
	if err != nil {
		return fmt.Errorf("failed to detect package name: %w", err)
	}
	printf(c.App.Writer, "Detected package: %s\n", pkgName)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if packageXMLPath != packageXMLFilename {
		cwd = filepath.Dir(packageXMLPath)
	}

	absInputFile := args.InputFile
	if !filepath.IsAbs(args.InputFile) {
		absInputFile = filepath.Join(cwd, args.InputFile)
	}

	if _, err := os.Stat(absInputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s (check the path is correct)", args.InputFile)
	}

	// Fail early before expensive Docker processing
	if err := validateOutputWritable(args.OutputFile); err != nil {
		return fmt.Errorf("output path not writable: %w (check directory exists and you have write permissions)", err)
	}

	relInputFile, err := filepath.Rel(cwd, absInputFile)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for input: %w", err)
	}

	dependentPkgs, err := discoverDependentPackages(absInputFile, cwd, pkgName)
	if err != nil {
		return fmt.Errorf(
			"failed to discover dependent packages: %w\n\nSuggestion: Ensure dependent packages are in the same parent directory as this package",
			err,
		)
	}
	if len(dependentPkgs) > 0 {
		printf(c.App.Writer, "Found dependent packages: %s\n", strings.Join(getPackageNames(dependentPkgs), ", "))
	}

	dockerArgs, err := processXacroArgs(args.Args, cwd, pkgName)
	if err != nil {
		return fmt.Errorf("failed to process xacro arguments: %w", err)
	}

	rosDistro := args.RosDistro
	if rosDistro == "" {
		rosDistro = extractROSDistro(args.DockerImage)
	}

	dockerCmd := buildDockerXacroCommand(
		pkgName, cwd, relInputFile, dockerArgs,
		args.DockerImage, dependentPkgs, args.InstallPackages, rosDistro,
	)

	if args.DryRun {
		printf(c.App.Writer, "Dry run - would execute:\n")
		printf(c.App.Writer, "%s %s\n", dockerExecutable, strings.Join(dockerCmd, " "))
		if args.CollapseFixedJoints {
			printf(c.App.Writer, "\nAfter Docker processing, would collapse fixed joint chains:\n")
			printf(c.App.Writer, "  - Removes fixed joints where the child link is a leaf (has no children)\n")
			printf(c.App.Writer, "  - Removes the corresponding child links\n")
			printf(c.App.Writer, "  - Simplifies kinematic structure while preserving functionality\n")
		}
		return nil
	}

	printf(c.App.Writer, "Processing with Docker...\n")

	ctx := context.Background()
	//nolint:gosec // G204: Docker command constructed from validated user input
	cmd := exec.CommandContext(ctx, dockerExecutable, dockerCmd...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("xacro processing failed: %v\nStderr: %s", err, stderr.String())
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "Cannot connect") {
			errMsg += fmt.Sprintf("\n\nSuggestion: Check that Docker is running (try '%s ps')", dockerExecutable)
		}
		return fmt.Errorf("%s", errMsg)
	}

	output := stdout.String()

	// Note: Only use this flag if the generated URDF has multiple end-effectors.
	// This happens when there are fixed joints (as opposed to revolute/prismatic).
	if args.CollapseFixedJoints {
		printf(c.App.Writer, "Collapsing fixed joint chains...\n")
		collapsed, err := collapseFixedJoints(output)
		if err != nil {
			if writeErr := os.WriteFile(args.OutputFile, []byte(output), fileOutputPerm); writeErr == nil {
				printf(c.App.Writer, "Warning: Collapse failed, wrote uncollapsed output to %s\n", args.OutputFile)
			}
			return fmt.Errorf(
				"failed to collapse fixed joints: %w\n\nSuggestion: The uncollapsed URDF has been saved. "+
					"Try running without --collapse-fixed-joints, or check the URDF structure",
				err,
			)
		}
		output = collapsed
	}

	if err := os.WriteFile(args.OutputFile, []byte(output), fileOutputPerm); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	printf(c.App.Writer, "Success! Generated: %s\n", args.OutputFile)
	return nil
}

// packageXML represents the minimal structure we need from a ROS package.xml file.
type packageXML struct {
	XMLName xml.Name `xml:"package"`
	Name    string   `xml:"name"`
}

// validateOutputWritable checks if we can write to the output path.
// This validates early before expensive Docker processing.
func validateOutputWritable(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if info, err := os.Stat(dir); err != nil {
			return fmt.Errorf("output directory does not exist: %s", dir)
		} else if !info.IsDir() {
			return fmt.Errorf("output directory path is not a directory: %s", dir)
		}
	}

	//nolint:gosec // G304: Output path specified by user
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE, fileOutputPerm)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	// Remove empty test file (don't leave artifacts if we just created it)
	if info, statErr := os.Stat(outputPath); statErr == nil && info.Size() == 0 {
		if err := os.Remove(outputPath); err != nil {
			return err
		}
	}

	return nil
}

// extractROSDistro extracts the ROS distribution from a docker image name.
// Examples:
//   - "osrf/ros:humble-desktop" -> "humble"
//   - "osrf/ros:iron-base" -> "iron"
//   - "myimage:latest" -> "humble" (default)
func extractROSDistro(dockerImg string) string {
	if dockerImg == "" {
		return defaultROSDistro
	}

	if strings.Contains(dockerImg, ":") {
		parts := strings.Split(dockerImg, ":")
		if len(parts) >= 2 {
			tag := parts[1]

			for _, suffix := range []string{"-desktop", "-base", "-perception", "-ros-core", "-ros-base"} {
				tag = strings.TrimSuffix(tag, suffix)
			}

			knownDistros := []string{"humble", "iron", "jazzy", "rolling", "noetic", "melodic", "foxy", "galactic"}
			for _, distro := range knownDistros {
				if tag == distro {
					return distro
				}
			}
		}
	}

	return defaultROSDistro
}

// extractPackageName extracts the package name from package.xml.
func extractPackageName(packageXMLPath string) (string, error) {
	//nolint:gosec // G304: Package XML path specified by user
	data, err := os.ReadFile(packageXMLPath)
	if err != nil {
		return "", err
	}

	var pkg packageXML
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("failed to parse package.xml: %w", err)
	}

	if pkg.Name == "" {
		return "", fmt.Errorf("package.xml does not contain a <name> element")
	}

	return strings.TrimSpace(pkg.Name), nil
}

// processXacroArgs processes xacro arguments, converting file paths to container paths.
// Returns relative paths (e.g., package/file.yaml) prefixed with ROS share path later.
func processXacroArgs(args []string, cwd, pkgName string) ([]string, error) {
	dockerArgs := make([]string, 0, len(args))

	for _, arg := range args {
		processed, err := processArgIfFilePath(arg, cwd, pkgName)
		if err != nil {
			return nil, err
		}
		dockerArgs = append(dockerArgs, processed)
	}

	return dockerArgs, nil
}

// processArgIfFilePath converts file path arguments to container paths.
// If the argument value is an existing file, converts it to package-relative format.
func processArgIfFilePath(arg, cwd, pkgName string) (string, error) {
	if !strings.Contains(arg, xacroArgSeparator) {
		return arg, nil
	}

	parts := strings.SplitN(arg, xacroArgSeparator, 2)
	key, value := parts[0], parts[1]

	if stat, err := os.Stat(value); err == nil && stat.Mode().IsRegular() {
		relPath, err := filepath.Rel(cwd, value)
		if err != nil {
			return "", fmt.Errorf("failed to compute relative path for %s: %w", value, err)
		}
		return fmt.Sprintf("%s%s%s/%s", key, xacroArgSeparator, pkgName, relPath), nil
	}

	return arg, nil
}

// packageInfo holds information about a ROS package.
type packageInfo struct {
	name string
	path string
}

// discoverDependentPackages scans xacro files for $(find package_name) and locates them.
func discoverDependentPackages(xacroPath, currentPkgDir, currentPkgName string) ([]packageInfo, error) {
	pkgNames := make(map[string]bool)
	if err := scanXacroForDependencies(xacroPath, currentPkgDir, pkgNames); err != nil {
		return nil, err
	}

	delete(pkgNames, currentPkgName)

	parentDir := filepath.Dir(currentPkgDir)
	var packages []packageInfo

	for pkgName := range pkgNames {
		pkgPath := filepath.Join(parentDir, pkgName)
		if info, err := os.Stat(pkgPath); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(pkgPath, packageXMLFilename)); err == nil {
				packages = append(packages, packageInfo{name: pkgName, path: pkgPath})
			}
		}
	}

	return packages, nil
}

// scanXacroForDependencies recursively scans xacro files for $(find package_name) references.
func scanXacroForDependencies(xacroPath, currentPkgDir string, pkgNames map[string]bool) error {
	//nolint:gosec // G304: Xacro files are user input for conversion
	data, err := os.ReadFile(xacroPath)
	if err != nil {
		return err
	}

	content := string(data)

	extractPatterns(content, xacroFindPrefix, ")", func(match string) {
		pkgName := strings.TrimSpace(match)
		if idx := strings.Index(pkgName, "/"); idx != -1 {
			pkgName = pkgName[:idx]
		}
		pkgNames[pkgName] = true
	})

	extractPatterns(content, xacroFilenamePrefix, "\"", func(filename string) {
		includePath := resolveIncludePath(filename, xacroPath, currentPkgDir)
		if includePath != "" {
			if _, statErr := os.Stat(includePath); statErr == nil {
				//nolint:errcheck // Intentionally ignoring errors from included files
				_ = scanXacroForDependencies(includePath, currentPkgDir, pkgNames)
			}
		}
	})

	return nil
}

// extractPatterns extracts all occurrences of a pattern from content and calls callback for each match.
func extractPatterns(content, startMarker, endMarker string, callback func(string)) {
	for {
		start := strings.Index(content, startMarker)
		if start == -1 {
			break
		}
		content = content[start+len(startMarker):]
		end := strings.Index(content, endMarker)
		if end == -1 {
			break
		}
		callback(content[:end])
		content = content[end+1:]
	}
}

// resolveIncludePath resolves an include filename to an absolute path.
// Handles $(find package_name)/path/file.xacro patterns and relative paths.
func resolveIncludePath(filename, xacroPath, currentPkgDir string) string {
	if strings.HasPrefix(filename, xacroFindPrefix) {
		closeIdx := strings.Index(filename, ")")
		if closeIdx != -1 {
			remainingPath := ""
			if closeIdx+1 < len(filename) && filename[closeIdx+1] == '/' {
				remainingPath = filename[closeIdx+2:]
			}
			return filepath.Join(currentPkgDir, remainingPath)
		}
	} else if !filepath.IsAbs(filename) {
		return filepath.Join(filepath.Dir(xacroPath), filename)
	}
	return ""
}

// getPackageNames extracts package names from packageInfo slice.
func getPackageNames(packages []packageInfo) []string {
	names := make([]string, len(packages))
	for i, pkg := range packages {
		names[i] = pkg.name
	}
	return names
}

// rosConfig holds ROS-specific path configurations.
type rosConfig struct {
	sharePath        string
	setupScript      string
	xacroPackageName string
}

// newROSConfig creates ROS path configuration for a given distribution.
func newROSConfig(rosDistro string) rosConfig {
	return rosConfig{
		sharePath:        fmt.Sprintf(rosSharePathPattern, rosDistro),
		setupScript:      fmt.Sprintf(rosSetupPathPattern, rosDistro),
		xacroPackageName: fmt.Sprintf(rosXacroPackPattern, rosDistro),
	}
}

// buildDockerXacroCommand builds the docker command to run xacro.
func buildDockerXacroCommand(
	pkgName, cwd, relInputFile string,
	dockerArgs []string,
	dockerImg string,
	dependentPkgs []packageInfo,
	installPackages bool,
	rosDistro string,
) []string {
	if dockerImg == "" {
		dockerImg = fmt.Sprintf("osrf/ros:%s-desktop", rosDistro)
	}

	ros := newROSConfig(rosDistro)
	allPkgs := collectPackageNames(pkgName, dependentPkgs)
	prefixedArgs := prefixArgsWithROSPath(dockerArgs, ros.sharePath)
	bashScript := buildXacroScript(relInputFile, prefixedArgs, allPkgs, ros, installPackages)

	dockerCmd := []string{dockerRunCmd, dockerRmFlag}
	dockerCmd = appendVolumeMounts(dockerCmd, pkgName, cwd, dependentPkgs, ros.sharePath)
	dockerCmd = append(dockerCmd,
		"-w", fmt.Sprintf("%s/%s", ros.sharePath, pkgName),
		dockerImg,
		"bash", "-c", bashScript,
	)

	return dockerCmd
}

// collectPackageNames gathers all package names (main + dependencies).
func collectPackageNames(mainPkg string, deps []packageInfo) []string {
	names := make([]string, 0, len(deps)+1)
	names = append(names, mainPkg)
	for _, pkg := range deps {
		names = append(names, pkg.name)
	}
	return names
}

// prefixArgsWithROSPath adds ROS share path prefix to relative path arguments.
// Distinguishes between file paths (containing '/') and plain strings (like name:=ur20).
func prefixArgsWithROSPath(args []string, rosSharePath string) []string {
	prefixed := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, xacroArgSeparator) {
			parts := strings.SplitN(arg, xacroArgSeparator, 2)
			value := parts[1]
			if strings.Contains(value, "/") && !strings.HasPrefix(value, "/") {
				prefixed[i] = fmt.Sprintf("%s%s%s/%s", parts[0], xacroArgSeparator, rosSharePath, value)
				continue
			}
		}
		prefixed[i] = arg
	}
	return prefixed
}

// buildXacroScript creates the bash script for running xacro in the container.
func buildXacroScript(inputFile string, args, packages []string, ros rosConfig, installPackages bool) string {
	var parts []string

	if installPackages {
		parts = append(parts,
			"apt-get update -qq > /dev/null",
			fmt.Sprintf("apt-get install -y -qq %s > /dev/null", ros.xacroPackageName),
		)
	}

	amentIndexPath := ros.sharePath + amentIndexPathSuffix
	parts = append(parts, fmt.Sprintf("mkdir -p %s", amentIndexPath))
	for _, pkg := range packages {
		parts = append(parts, fmt.Sprintf("touch %s/%s", amentIndexPath, pkg))
	}

	parts = append(parts,
		fmt.Sprintf("source %s", ros.setupScript),
		//nolint:dupword // "ros2 run xacro xacro" is the correct ROS command
		fmt.Sprintf("ros2 run xacro xacro %s %s", inputFile, strings.Join(args, " ")),
	)

	return strings.Join(parts, " && \\\n")
}

// appendVolumeMounts adds volume mount flags for all packages.
func appendVolumeMounts(cmd []string, mainPkg, mainPath string, deps []packageInfo, rosSharePath string) []string {
	cmd = append(cmd, "-v", fmt.Sprintf("%s:%s/%s", mainPath, rosSharePath, mainPkg))
	for _, pkg := range deps {
		cmd = append(cmd, "-v", fmt.Sprintf("%s:%s/%s", pkg.path, rosSharePath, pkg.name))
	}
	return cmd
}

// collapseFixedJoints removes fixed joints and their child links when the child is a leaf node.
// This simplifies the kinematic structure by removing non-functional branches like "base_link -> base"
// and "link_6_t -> flange -> tool0" chains.
func collapseFixedJoints(urdfContent string) (string, error) {
	var robot urdfRobot
	if err := xml.Unmarshal([]byte(urdfContent), &robot); err != nil {
		return "", fmt.Errorf("failed to parse URDF: %w", err)
	}

	childLinks := make(map[string]int)
	for _, joint := range robot.Joints {
		childLinks[joint.Child.Link]++
	}

	parentLinks := make(map[string]bool)
	for _, joint := range robot.Joints {
		parentLinks[joint.Parent.Link] = true
	}

	fixedLeafJoints := make(map[string]bool)
	for _, joint := range robot.Joints {
		if joint.Type == "fixed" && !parentLinks[joint.Child.Link] {
			fixedLeafJoints[joint.Name] = true
		}
	}

	if len(fixedLeafJoints) == 0 {
		return urdfContent, nil
	}

	var newJoints []urdfJoint
	leafLinksToRemove := make(map[string]bool)
	for _, joint := range robot.Joints {
		if fixedLeafJoints[joint.Name] {
			leafLinksToRemove[joint.Child.Link] = true
			continue
		}
		newJoints = append(newJoints, joint)
	}
	robot.Joints = newJoints

	var newLinks []urdfLink
	for _, link := range robot.Links {
		if leafLinksToRemove[link.Name] {
			continue
		}
		newLinks = append(newLinks, link)
	}
	robot.Links = newLinks

	output, err := xml.MarshalIndent(&robot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal URDF: %w", err)
	}

	return xml.Header + string(output) + "\n", nil
}
