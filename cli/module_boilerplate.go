// Package cli contains all business logic needed by the CLI command.
package cli

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"
	goutils "go.viam.com/utils"
)

//go:embed modulegen/dist/__main__
var executable []byte

func writeExecutableFile(fileName string) error {
	if err := os.WriteFile(fileName, executable, 0700); err != nil {
		return err
	}
	return nil
}

func removeFile(fileName string) {
	os.Remove(fileName)
}

// ModuleBoilerplateGenerationAction is the corresponding action for 'module generate'
func ModuleBoilerplateGenerationAction(*cli.Context) error {
	fileName := goutils.RandomAlphaString(8)
	if err := writeExecutableFile(fileName); err != nil {
		return err
	}
	defer removeFile(fileName)

	currDir, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(currDir, fileName)

	cmd := exec.Command(path)
	stdIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("%s: %s", err.Error(), stderr.String())
	}

	for {
		var buf = make([]byte, 512)
		n, err := stdOut.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		fmt.Println(string(buf[:n]))
		var input string
		fmt.Scanln(&input)
		input = fmt.Sprintf("%s\n", input)
		stdIn.Write([]byte(input))
	}

	stdIn.Close()
	cmd.Wait()
	return nil
}
