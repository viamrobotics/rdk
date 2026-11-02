// Package main implements the subsystem_manifest generator
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
)

const (
	viamServer   = "viam-server"
	linuxAmd64   = "linux/amd64"
	linuxArm64   = "linux/arm64"
	linuxArm32v7 = "linux/arm32v7"
)

type substemManifest struct {
	Subsystem  string `json:"subsystem"`
	Version    string `json:"version"`
	Platform   string `json:"platform"`
	UploadPath string `json:"upload-path"`
	Sha256     string `json:"sha256"`
}

func main() {
	subsystem := flag.String("subsystem", viamServer, "subsystem type") // default to viam-server
	binaryPath := flag.String("binary-path", "", "path to subsystem binary")
	uploadPath := flag.String("upload-path", "", "path where this binary will be stored in gcs")
	outputPath := flag.String("output-path", "", "path where this manifest json file will be written")
	version := flag.String("version", "", "version")
	arch := flag.String("arch", "", "arch (result of uname -m) ex: x86_64")

	flag.Parse()

	ensureStringFlagPresent(*binaryPath, "binary-path")
	ensureStringFlagPresent(*uploadPath, "upload-path")
	ensureStringFlagPresent(*outputPath, "output-path")
	ensureStringFlagPresent(*version, "version")
	ensureStringFlagPresent(*arch, "arch")

	binarySha, err := sha256sum(*binaryPath)
	if err != nil {
		log.Fatalf("failed to calculate binary sha: %v", err)
	}
	platform, err := osArchToViamPlatform(*arch)
	if err != nil {
		log.Fatalf("failed to get platform: %v", err)
	}

	manifest := substemManifest{
		Subsystem:  *subsystem,
		Version:    strings.TrimPrefix(*version, "v"),
		Platform:   platform,
		UploadPath: *uploadPath,
		Sha256:     binarySha,
	}

	// marshall and output the manifest to the provided output-path
	jsonResult, err := json.MarshalIndent(manifest, "", "\t")
	if err != nil {
		log.Fatalf("failed to marshall json result %v", err)
	}

	if err := os.WriteFile(*outputPath, jsonResult, 0o600); err != nil {
		log.Fatalf("failed to write result %v", err)
	}
}

func sha256sum(path string) (string, error) {
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	//nolint:errcheck
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
	case "armv7l":
		return linuxArm32v7, nil
	default:
		return "", errors.Errorf("unknown architecture %q", arch)
	}
}

func ensureStringFlagPresent(flagValue, flagName string) {
	if flagValue == "" {
		log.Fatalf("%q is a required flag", flagName)
	}
}
