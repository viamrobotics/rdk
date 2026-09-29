package main

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

// writeFiles creates each named file, empty, under dir, creating parent directories as needed.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		test.That(t, os.MkdirAll(filepath.Dir(path), 0o755), test.ShouldBeNil)
		test.That(t, os.WriteFile(path, nil, 0o600), test.ShouldBeNil)
	}
}

func TestCollectRequestFiles(t *testing.T) {
	t.Run("no arguments", func(t *testing.T) {
		_, err := collectRequestFiles(nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("files are taken in the order given", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "b.json", "a.json")

		files, err := collectRequestFiles([]string{
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "a.json"),
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "a.json"),
		})
	})

	t.Run("a file that is not json is still taken as given", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "capture.txt")

		files, err := collectRequestFiles([]string{filepath.Join(dir, "capture.txt")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{filepath.Join(dir, "capture.txt")})
	})

	t.Run("a missing path errors", func(t *testing.T) {
		_, err := collectRequestFiles([]string{filepath.Join(t.TempDir(), "nope.json")})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("a directory expands alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "c.json", "a.json", "b.json")

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "a.json"),
			filepath.Join(dir, "b.json"),
			filepath.Join(dir, "c.json"),
		})
	})

	t.Run("a directory keeps only json entries, case-insensitively", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "a.json", "b.JSON", "notes.txt", "plan.json.bak", "no-extension")

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "a.json"),
			filepath.Join(dir, "b.JSON"),
		})
	})

	t.Run("a directory is not descended into", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "a.json", filepath.Join("nested", "b.json"))

		files, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{filepath.Join(dir, "a.json")})
	})

	t.Run("a directory with no json errors", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "notes.txt")

		_, err := collectRequestFiles([]string{dir})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, dir)
	})

	t.Run("an empty directory errors", func(t *testing.T) {
		_, err := collectRequestFiles([]string{t.TempDir()})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("files and directories mix, each expanded in place", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, "lead.json", filepath.Join("seq", "b.json"), filepath.Join("seq", "a.json"), "tail.json")

		files, err := collectRequestFiles([]string{
			filepath.Join(dir, "lead.json"),
			filepath.Join(dir, "seq"),
			filepath.Join(dir, "tail.json"),
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, files, test.ShouldResemble, []string{
			filepath.Join(dir, "lead.json"),
			filepath.Join(dir, "seq", "a.json"),
			filepath.Join(dir, "seq", "b.json"),
			filepath.Join(dir, "tail.json"),
		})
	})
}
