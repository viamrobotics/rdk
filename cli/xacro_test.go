package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
)

func TestExtractPackageName(t *testing.T) {
	t.Run("valid package.xml", func(t *testing.T) {
		tmpDir := t.TempDir()
		packageXML := filepath.Join(tmpDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <name>test_package</name>
  <version>1.0.0</version>
</package>`
		err := os.WriteFile(packageXML, []byte(content), 0644)
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
		err := os.WriteFile(packageXML, []byte(content), 0644)
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
		err := os.WriteFile(packageXML, []byte(content), 0644)
		test.That(t, err, test.ShouldBeNil)

		_, err = extractPackageName(packageXML)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "could not find <name> tag")
	})

	t.Run("file does not exist", func(t *testing.T) {
		_, err := extractPackageName("/nonexistent/package.xml")
		test.That(t, err, test.ShouldNotBeNil)
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
		err := os.WriteFile(testFile, []byte("test: data"), 0644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"config:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 1)
		test.That(t, result[0], test.ShouldEqual, "config:=/opt/ros/humble/share/test_pkg/config.yaml")
	})

	t.Run("nested file path argument", func(t *testing.T) {
		// Create nested directory structure
		configDir := filepath.Join(tmpDir, "config", "ur20")
		err := os.MkdirAll(configDir, 0755)
		test.That(t, err, test.ShouldBeNil)

		testFile := filepath.Join(configDir, "params.yaml")
		err = os.WriteFile(testFile, []byte("test: data"), 0644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"params:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, pkgName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result), test.ShouldEqual, 1)
		test.That(t, result[0], test.ShouldEqual, "params:=/opt/ros/humble/share/test_pkg/config/ur20/params.yaml")
	})

	t.Run("mixed arguments", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(testFile, []byte("test: data"), 0644)
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
		test.That(t, result[1], test.ShouldEqual, "config:=/opt/ros/humble/share/test_pkg/config.yaml")
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
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "")

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

		// Check default image
		test.That(t, result[len(result)-3], test.ShouldEqual, "osrf/ros:humble-desktop")
	})

	t.Run("custom docker image", func(t *testing.T) {
		customImage := "osrf/ros:iron-desktop"
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, customImage)

		test.That(t, result[len(result)-3], test.ShouldEqual, customImage)
	})

	t.Run("bash script contains xacro command", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "")

		// Last argument should be the bash script
		bashScript := result[len(result)-1]

		test.That(t, bashScript, test.ShouldContainSubstring, "ros2 run xacro xacro")
		test.That(t, bashScript, test.ShouldContainSubstring, relInputFile)
		test.That(t, bashScript, test.ShouldContainSubstring, "model:=ur20")
		test.That(t, bashScript, test.ShouldContainSubstring, "config:=/opt/ros/humble/share/test_pkg/config.yaml")
		test.That(t, bashScript, test.ShouldContainSubstring, "apt-get install -y -qq ros-humble-xacro")
	})

	t.Run("package registration in bash script", func(t *testing.T) {
		result := buildDockerXacroCommand(pkgName, cwd, relInputFile, dockerArgs, "")

		bashScript := result[len(result)-1]

		// Should create package index entry
		test.That(t, bashScript, test.ShouldContainSubstring, "ament_index/resource_index/packages")
		test.That(t, bashScript, test.ShouldContainSubstring, pkgName)
	})
}

func TestXacroConvertActionEdgeCases(t *testing.T) {
	t.Run("package.xml in subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "ros_pkg")
		err := os.MkdirAll(subDir, 0755)
		test.That(t, err, test.ShouldBeNil)

		packageXML := filepath.Join(subDir, "package.xml")
		content := `<?xml version="1.0"?>
<package format="2">
  <name>subdir_pkg</name>
</package>`
		err = os.WriteFile(packageXML, []byte(content), 0644)
		test.That(t, err, test.ShouldBeNil)

		name, err := extractPackageName(packageXML)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, name, test.ShouldEqual, "subdir_pkg")
	})

	t.Run("relative path computation", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yaml")
		err := os.WriteFile(testFile, []byte("data"), 0644)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"file:=" + testFile}
		result, err := processXacroArgs(args, tmpDir, "pkg")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result[0], test.ShouldEqual, "file:=/opt/ros/humble/share/pkg/test.yaml")
	})
}

func TestDockerCommandStructure(t *testing.T) {
	t.Run("command structure is valid", func(t *testing.T) {
		cmd := buildDockerXacroCommand("pkg", "/path", "input.xacro", []string{}, "")

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
