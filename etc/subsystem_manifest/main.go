// Package main implements the subsystem_manifest generator
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.viam.com/utils/pexec"
)

const (
	viamServer = "viam-server"
	linuxAmd64 = "linux/amd64"
	linuxArm64 = "linux/arm64"
)

type dumpedResourceRegistration struct {
	API             string             `json:"api"`
	Model           string             `json:"model"`
	AttributeSchema *jsonschema.Schema `json:"attribute_schema,omitempty"`
}

type viamServerMetadata struct {
	ResourceRegistrations []dumpedResourceRegistration `json:"resource_registrations"`
}

type substemManifest struct {
	Subsystem  string              `json:"subsystem"`
	Version    string              `json:"version"`
	Platform   string              `json:"platform"`
	ObjectPath string              `json:"object-path"`
	Sha256     string              `json:"sha256"`
	Metadata   *viamServerMetadata `json:"metadata,omitempty"`
}

func main() {
	subsystem := flag.String("subsystem", viamServer, "subsystem type")
	binaryPath := flag.String("binary-path", "", "path to subsystem binary")
	objectPath := flag.String("object-path", "", "path where this binary will be stored in gcs")
	version := flag.String("version", "", "version")
	arch := flag.String("arch", "", "arch (result of uname -m) ex: x86_64")

	flag.Parse()

	if *binaryPath == "" || *objectPath == "" || *arch == "" || *version == "" {
		log.Fatal("binary-path, arch, version, and url are required arguments")
	}

	binarySha, err := sha256sum(*binaryPath)
	if err != nil {
		log.Fatalf("failed to calculate binary sha: %v", err)
	}
	metadata, err := getViamServerMetadata(*binaryPath)
	if err != nil {
		log.Fatalf("failed to get viam-server metadata: %v", err)
	}
	platform, err := osArchToViamPlatform(*arch)
	if err != nil {
		log.Fatalf("failed to get platform: %v", err)
	}

	manifest := substemManifest{
		Subsystem:  *subsystem,
		Version:    strings.TrimPrefix(*version, "v"),
		Platform:   platform,
		ObjectPath: *objectPath,
		Sha256:     binarySha,
		Metadata:   metadata,
	}
	jsonResult, err := json.MarshalIndent(manifest, "", "\t")
	if err != nil {
		log.Fatalf("failed to marshall json result %v", err)
	}

	if _, err := os.Stdout.Write(jsonResult); err != nil {
		log.Fatalf("failed to print result %v", err)
	}
}

func getViamServerMetadata(path string) (*viamServerMetadata, error) {
	output := new(bytes.Buffer)
	processConfig := pexec.ProcessConfig{
		Name:      path,
		Args:      []string{"--dump-resources"},
		OneShot:   true,
		Log:       false,
		LogWriter: output,
	}
	proc := pexec.NewManagedProcess(processConfig, zap.NewNop().Sugar().Named("blank"))
	if err := proc.Start(context.Background()); err != nil {
		return nil, err
	}
	dumpedResourceRegistrations := []dumpedResourceRegistration{}
	if err := json.Unmarshal(output.Bytes(), &dumpedResourceRegistrations); err != nil {
		return nil, err
	}
	return &viamServerMetadata{
		ResourceRegistrations: dumpedResourceRegistrations,
	}, nil
}

func sha256sum(path string) (string, error) {
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	//nolint:errcheck,gosec
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func osArchToViamPlatform(arch string) (string, error) {
	switch arch {
	case "x86_64":
		return linuxAmd64, nil
	case "aarch64":
		return linuxArm64, nil
	default:
		return "", errors.Errorf("unknown architecture %q", arch)
	}
}
