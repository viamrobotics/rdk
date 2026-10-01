package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"
)

func sourceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func sourceRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	sourceGit(t, root, "init")
	sourceGit(t, root, "config", "user.name", "Source test")
	sourceGit(t, root, "config", "user.email", "source@example.invalid")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	sourceGit(t, root, "add", ".")
	sourceGit(t, root, "commit", "-m", "source")
	return root, sourceGit(t, root, "rev-parse", "HEAD")
}

func sourceArchive(t *testing.T, root string) (moduleSourceMetadata, map[string][]byte) {
	t.Helper()
	vc := &viamClient{}
	archive, err := vc.createGitArchive(t.Context(), root)
	test.That(t, err, test.ShouldBeNil)
	files := archiveContents(t, archive)
	var metadata moduleSourceMetadata
	test.That(t, json.Unmarshal(files[moduleSourceMetadataFile], &metadata), test.ShouldBeNil)
	return metadata, files
}

func TestSourceArchiveGitMetadata(t *testing.T) {
	root, revision := sourceRepo(t)
	metadata, files := sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})
	test.That(t, files["main.go"], test.ShouldResemble, []byte("package main\n"))
	_, err := os.Stat(filepath.Join(root, moduleSourceMetadataFile))
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)

	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})
	writeFile(t, filepath.Join(root, "ignored", "output"), "ignored")
	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git.Modified, test.ShouldBeFalse)

	writeFile(t, filepath.Join(root, "main.go"), "package changed\n")
	metadata, files = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision, Modified: true})
	test.That(t, files["main.go"], test.ShouldResemble, []byte("package changed\n"))
	sourceGit(t, root, "add", "main.go")
	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git.Modified, test.ShouldBeTrue)
	sourceGit(t, root, "commit", "-m", "changed")
	revision = sourceGit(t, root, "rev-parse", "HEAD")
	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})

	writeFile(t, filepath.Join(root, "new.go"), "package changed\n")
	metadata, files = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision, Modified: true})
	test.That(t, files["new.go"], test.ShouldResemble, []byte("package changed\n"))
}

func TestSourceArchiveWorktree(t *testing.T) {
	root, revision := sourceRepo(t)
	worktree := filepath.Join(t.TempDir(), "worktree")
	sourceGit(t, root, "worktree", "add", "--detach", worktree, revision)
	metadata, files := sourceArchive(t, worktree)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})
	_, gitIncluded := files[".git"]
	test.That(t, gitIncluded, test.ShouldBeFalse)

	writeFile(t, filepath.Join(worktree, "module", "main.go"), "package main\n")
	sourceGit(t, worktree, "add", "module")
	sourceGit(t, worktree, "commit", "-m", "module")
	revision = sourceGit(t, worktree, "rev-parse", "HEAD")
	metadata, files = sourceArchive(t, filepath.Join(worktree, "module"))
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})
	test.That(t, files["main.go"], test.ShouldResemble, []byte("package main\n"))
}

func TestSourceArchiveReplacesStaleMetadata(t *testing.T) {
	root, revision := sourceRepo(t)
	stale := `{"git":{"revision":"stale","modified":true}}`
	writeFile(t, filepath.Join(root, moduleSourceMetadataFile), stale)
	metadata, _ := sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldResemble, &moduleSourceGit{Revision: revision})
	content, err := os.ReadFile(filepath.Join(root, moduleSourceMetadataFile))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(content), test.ShouldEqual, stale)

	t.Setenv("PATH", t.TempDir())
	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldBeNil)
}

func TestSourceArchiveWithoutCommit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, moduleSourceMetadataFile), `{"git":{"revision":"stale","modified":false}}`)
	metadata, files := sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldBeNil)
	test.That(t, files["main.go"], test.ShouldResemble, []byte("package main\n"))
	sourceGit(t, root, "init")
	metadata, _ = sourceArchive(t, root)
	test.That(t, metadata.Git, test.ShouldBeNil)
}
