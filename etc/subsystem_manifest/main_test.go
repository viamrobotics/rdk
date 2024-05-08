package main

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestSha256(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "a.txt")
	os.WriteFile(filePath, []byte("hello"), 0o700)
	sum, err := sha256sum(filePath)
	test.That(t, err, test.ShouldBeNil)
	// derived with bash -c "printf "hello" > test.txt ; sha256sum test.txt"
	test.That(t, sum, test.ShouldEqual, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
}

func TestArchConversion(t *testing.T) {
	arch, err := osArchToViamPlatform("x86_64")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arch, test.ShouldEqual, "linux/amd64")

	arch, err = osArchToViamPlatform("aarch64")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arch, test.ShouldEqual, "linux/arm64")

	_, err = osArchToViamPlatform("invalid")
	test.That(t, err, test.ShouldNotBeNil)
}
