package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestExtractROSDistro(t *testing.T) {
	t.Run("extracts from standard osrf images", func(t *testing.T) {
		test.That(t, extractROSDistro("osrf/ros:humble-desktop"), test.ShouldEqual, "humble")
		test.That(t, extractROSDistro("osrf/ros:iron-desktop"), test.ShouldEqual, "iron")
		test.That(t, extractROSDistro("osrf/ros:jazzy-base"), test.ShouldEqual, "jazzy")
		test.That(t, extractROSDistro("osrf/ros:rolling-perception"), test.ShouldEqual, "rolling")
	})

	t.Run("handles different suffixes", func(t *testing.T) {
		test.That(t, extractROSDistro("osrf/ros:humble-desktop"), test.ShouldEqual, "humble")
		test.That(t, extractROSDistro("osrf/ros:iron-base"), test.ShouldEqual, "iron")
		test.That(t, extractROSDistro("osrf/ros:jazzy-perception"), test.ShouldEqual, "jazzy")
		test.That(t, extractROSDistro("osrf/ros:rolling-ros-core"), test.ShouldEqual, "rolling")
	})

	t.Run("fallback to humble for unknown images", func(t *testing.T) {
		test.That(t, extractROSDistro("myimage:latest"), test.ShouldEqual, "humble")
		test.That(t, extractROSDistro("myimage:v1.0"), test.ShouldEqual, "humble")
		test.That(t, extractROSDistro("ubuntu:22.04"), test.ShouldEqual, "humble")
	})

	t.Run("empty image defaults to humble", func(t *testing.T) {
		test.That(t, extractROSDistro(""), test.ShouldEqual, "humble")
	})

	t.Run("no colon uses default", func(t *testing.T) {
		result := extractROSDistro("myimage")
		test.That(t, result, test.ShouldEqual, "humble")
	})
}

func TestExtractPackageName(t *testing.T) {
	t.Run("valid package.xml", func(t *testing.T) {
		tmpDir := t.TempDir()
		packageXML := filepath.Join(tmpDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <name>test_package</name>
  <version>1.0.0</version>
</package>`
		err := os.WriteFile(packageXML, []byte(content), 0o644)
		test.That(t, err, test.ShouldBeNil)

		name, err := extractPackageName(packageXML)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, name, test.ShouldEqual, "test_package")
	})

	t.Run("package.xml with whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		packageXML := filepath.Join(tmpDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <name>
    ur_description
  </name>
</package>`
		err := os.WriteFile(packageXML, []byte(content), 0o644)
		test.That(t, err, test.ShouldBeNil)

		name, err := extractPackageName(packageXML)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, name, test.ShouldEqual, "ur_description")
	})

	t.Run("missing name tag", func(t *testing.T) {
		tmpDir := t.TempDir()
		packageXML := filepath.Join(tmpDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <version>1.0.0</version>
</package>`
		err := os.WriteFile(packageXML, []byte(content), 0o644)
		test.That(t, err, test.ShouldBeNil)

		_, err = extractPackageName(packageXML)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "does not contain a <name> element")
	})

	t.Run("file does not exist", func(t *testing.T) {
		_, err := extractPackageName("/nonexistent/package.xml")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("invalid XML", func(t *testing.T) {
		tmpDir := t.TempDir()
		packageXML := filepath.Join(tmpDir, "package.xml")
		content := `this is not valid XML`
		err := os.WriteFile(packageXML, []byte(content), 0o644)
		test.That(t, err, test.ShouldBeNil)

		_, err = extractPackageName(packageXML)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to parse package.xml")
	})
}

func TestProcessXacroArgs(t *testing.T) {
	tmpDir := t.TempDir()
	pkgName := "test_pkg"

	t.Run("simple string arguments", func(t *testing.T) {
		args := []string{"model:=ur20", "version:=1.0"}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, args)
	})

	t.Run("file path argument gets converted", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(testFile, []byte("test: data"), 0o644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"config:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 1)
		// Returns relative path (ROS share path prefix added later in buildDockerXacroCommand)
		test.That(t, result[0], test.ShouldEqual, "config:=test_pkg/config.yaml")
	})

	t.Run("nested file path argument", func(t *testing.T) {
		// Create nested directory structure
		configDir := filepath.Join(tmpDir, "config", "ur20")
		err := os.MkdirAll(configDir, 0o755)
		test.That(t, err, test.ShouldBeNil)

		testFile := filepath.Join(configDir, "params.yaml")
		err = os.WriteFile(testFile, []byte("test: data"), 0o644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"params:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 1)
		test.That(t, result[0], test.ShouldEqual, "params:=test_pkg/config/ur20/params.yaml")
	})

	t.Run("mixed arguments", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(testFile, []byte("test: data"), 0o644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{
			"model:=ur20",
			"config:=" + testFile,
			"version:=1.0",
		}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 3)
		test.That(t, result[0], test.ShouldEqual, "model:=ur20")
		test.That(t, result[1], test.ShouldEqual, "config:=test_pkg/config.yaml")
		test.That(t, result[2], test.ShouldEqual, "version:=1.0")
	})

	t.Run("non-file path value unchanged", func(t *testing.T) {
		args := []string{"config:=/some/nonexistent/file.yaml"}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, args)
	})

	t.Run("argument without := separator", func(t *testing.T) {
		args := []string{"--debug", "verbose"}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, args)
	})
}

func TestBuildDockerXacroCommand(t *testing.T) {
	pkgName := "test_pkg"
	cwd := "/home/user/test_pkg"
	relInputFile := "urdf/robot.urdf.xacro"
	dockerArgs := []string{"model:=ur20", "config:=/opt/ros/humble/share/test_pkg/config.yaml"}

	t.Run("default docker image", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "", nil, false, "humble")

		test.That(t, result[0], test.ShouldEqual, "run")
		test.That(t, result[1], test.ShouldEqual, "--rm")

		// Check volume mount
		volumeMount := ""
		for i, arg := range result {
			if arg == "-v" && i+1 < len(result) {
				volumeMount = result[i+1]
				break
			}
		}
		test.That(t, volumeMount, test.ShouldEqual, "/home/user/test_pkg:/opt/ros/humble/share/test_pkg")

		// Check working directory
		workDir := ""
		for i, arg := range result {
			if arg == "-w" && i+1 < len(result) {
				workDir = result[i+1]
				break
			}
		}
		test.That(t, workDir, test.ShouldEqual, "/opt/ros/humble/share/test_pkg")

		// Check default image (should be before "bash", "-c", "<script>")
		// Command ends with: [..., "<IMAGE>", "bash", "-c", "<script>"]
		imageIdx := len(result) - 4
		test.That(t, result[imageIdx], test.ShouldEqual, "osrf/ros:humble-desktop")
	})

	t.Run("custom docker image", func(t *testing.T) {
		customImage := "osrf/ros:iron-desktop"
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, customImage, nil, false, "iron")

		imageIdx := len(result) - 4
		test.That(t, result[imageIdx], test.ShouldEqual, customImage)
	})

	t.Run("bash script contains xacro command", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "", nil, false, "humble")

		// Last argument should be the bash script
		bashScript := result[len(result)-1]

		test.That(t, bashScript, test.ShouldContainSubstring, "ros2 run xacro xacro")
		test.That(t, bashScript, test.ShouldContainSubstring, relInputFile)
		// Arguments now get prefixed with ROS share path in buildDockerXacroCommand
		test.That(t, bashScript, test.ShouldContainSubstring, "/opt/ros/humble/share")
	})

	t.Run("package registration in bash script", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "", nil, false, "humble")

		bashScript := result[len(result)-1]

		// Should create package index entry
		test.That(t, bashScript, test.ShouldContainSubstring, "ament_index/resource_index/packages")
		test.That(t, bashScript, test.ShouldContainSubstring, pkgName)
	})

	t.Run("installPackages false skips apt-get", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "", nil, false, "humble")

		bashScript := result[len(result)-1]

		// Should NOT have apt-get commands
		test.That(t, bashScript, test.ShouldNotContainSubstring, "apt-get update")
		test.That(t, bashScript, test.ShouldNotContainSubstring, "apt-get install")
	})

	t.Run("installPackages true includes apt-get", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "", nil, true, "humble")

		bashScript := result[len(result)-1]

		// Should have apt-get commands
		test.That(t, bashScript, test.ShouldContainSubstring, "apt-get update")
		test.That(t, bashScript, test.ShouldContainSubstring, "apt-get install -y -qq ros-humble-xacro")
	})

	t.Run("ROS distro affects paths and packages", func(t *testing.T) {
		// Test with Iron distro
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, []string{}, "", nil, true, "iron")

		bashScript := result[len(result)-1]

		// Check Iron-specific paths and packages
		test.That(t, bashScript, test.ShouldContainSubstring, "/opt/ros/iron/share")
		test.That(t, bashScript, test.ShouldContainSubstring, "ros-iron-xacro")
		test.That(t, bashScript, test.ShouldContainSubstring, "source /opt/ros/iron/setup.bash")

		// Check volume mounts use Iron path
		foundMount := false
		for i, arg := range result {
			if arg == "-v" && i+1 < len(result) {
				if strings.Contains(result[i+1], "/opt/ros/iron/share") {
					foundMount = true
				}
			}
		}
		test.That(t, foundMount, test.ShouldBeTrue)
	})

	t.Run("string arguments not prefixed with ROS path", func(t *testing.T) {
		// String arguments (no slashes) should NOT be prefixed with ROS path
		stringArgs := []string{"name:=ur20e", "ur_type:=ur20", "version:=3.14"}
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, stringArgs, "", nil, false, "humble")

		bashScript := result[len(result)-1]

		// String arguments should appear unchanged (not prefixed with /opt/ros/humble/share)
		test.That(t, bashScript, test.ShouldContainSubstring, "name:=ur20e")
		test.That(t, bashScript, test.ShouldContainSubstring, "ur_type:=ur20")
		test.That(t, bashScript, test.ShouldNotContainSubstring, "name:=/opt/ros/humble/share/ur20e")
		test.That(t, bashScript, test.ShouldNotContainSubstring, "ur_type:=/opt/ros/humble/share/ur20")
	})

	t.Run("file path arguments prefixed with ROS path", func(t *testing.T) {
		// File path arguments (with slashes) should be prefixed with ROS path
		fileArgs := []string{"config:=test_pkg/config/params.yaml", "meshes:=test_pkg/meshes"}
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, fileArgs, "", nil, false, "humble")

		bashScript := result[len(result)-1]

		// File path arguments should be prefixed
		test.That(t, bashScript, test.ShouldContainSubstring, "config:=/opt/ros/humble/share/test_pkg/config/params.yaml")
		test.That(t, bashScript, test.ShouldContainSubstring, "meshes:=/opt/ros/humble/share/test_pkg/meshes")
	})
}

func TestXacroConvertActionEdgeCases(t *testing.T) {
	t.Run("package.xml in subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "ros_pkg")
		err := os.MkdirAll(subDir, 0o755)
		test.That(t, err, test.ShouldBeNil)

		packageXML := filepath.Join(subDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <name>subdir_pkg</name>
</package>`
		err = os.WriteFile(packageXML, []byte(content), 0o644)
		test.That(t, err, test.ShouldBeNil)

		name, err := extractPackageName(packageXML)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, name, test.ShouldEqual, "subdir_pkg")
	})

	t.Run("relative path computation", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yaml")
		err := os.WriteFile(testFile, []byte("data"), 0o644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"file:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, "pkg")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result[0], test.ShouldEqual, "file:=pkg/test.yaml")
	})
}

func TestDockerCommandStructure(t *testing.T) {
	t.Run("command structure is valid", func(t *testing.T) {
		cmd := buildDockerXacroCommand("pkg", "/path", "input.xacro", []string{}, "", nil, false, "humble")

		// Should start with "run --rm"
		test.That(t, cmd[0], test.ShouldEqual, "run")
		test.That(t, cmd[1], test.ShouldEqual, "--rm")

		// Should have -v flag
		hasVolumeFlag := false
		for i, arg := range cmd {
			if arg == "-v" && i+1 < len(cmd) {
				hasVolumeFlag = true
				// Volume mount should contain both paths
				test.That(t, strings.Contains(cmd[i+1], "/path"), test.ShouldBeTrue)
				test.That(t, strings.Contains(cmd[i+1], "/opt/ros/humble/share/pkg"), test.ShouldBeTrue)
			}
		}
		test.That(t, hasVolumeFlag, test.ShouldBeTrue)

		// Should have -w flag
		hasWorkDirFlag := false
		for i, arg := range cmd {
			if arg == "-w" && i+1 < len(cmd) {
				hasWorkDirFlag = true
			}
		}
		test.That(t, hasWorkDirFlag, test.ShouldBeTrue)

		// Should end with bash -c <script>
		test.That(t, cmd[len(cmd)-3], test.ShouldEqual, "bash")
		test.That(t, cmd[len(cmd)-2], test.ShouldEqual, "-c")
	})
}

func TestValidateOutputWritable(t *testing.T) {
	t.Run("valid writable path", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "output.urdf")
		err := validateOutputWritable(outputPath)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("path in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tmpDir)

		err := validateOutputWritable("output.urdf")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		err := validateOutputWritable("/nonexistent/dir/output.urdf")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "does not exist")
	})

	t.Run("can overwrite existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "existing.urdf")
		err := os.WriteFile(outputPath, []byte("existing content"), 0o644)
		test.That(t, err, test.ShouldBeNil)

		err = validateOutputWritable(outputPath)
		test.That(t, err, test.ShouldBeNil)

		// Verify original file still exists and has content
		content, err := os.ReadFile(outputPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(content), test.ShouldEqual, "existing content")
	})

	t.Run("read-only directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping read-only test when running as root")
		}

		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		err := os.Mkdir(readOnlyDir, 0o555)
		test.That(t, err, test.ShouldBeNil)
		defer os.Chmod(readOnlyDir, 0o755) // Cleanup

		outputPath := filepath.Join(readOnlyDir, "output.urdf")
		err = validateOutputWritable(outputPath)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
