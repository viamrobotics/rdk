package cli

import (
	"context"
	"os/exec"
	"strings"
)

const moduleSourceMetadataFile = ".viam-source.json"

type moduleSourceMetadata struct {
	Git *moduleSourceGit `json:"git"`
}

type moduleSourceGit struct {
	Revision string `json:"revision"`
	Modified bool   `json:"modified"`
}

func readModuleSourceMetadata(ctx context.Context, sourcePath string) moduleSourceMetadata {
	// Use Git itself so linked worktrees and its ignore rules have their usual semantics.
	revisionCommand := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD")
	revisionCommand.Dir = sourcePath
	revision, err := revisionCommand.Output()
	if err != nil {
		return moduleSourceMetadata{}
	}
	statusCommand := exec.CommandContext(ctx, "git", "--no-optional-locks",
		"status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".",
		":(exclude).VIAM_RELOAD_ARCHIVE.tar.gz", ":(exclude)"+moduleSourceMetadataFile)
	statusCommand.Dir = sourcePath
	status, err := statusCommand.Output()
	if err != nil {
		return moduleSourceMetadata{}
	}
	return moduleSourceMetadata{Git: &moduleSourceGit{
		Revision: strings.TrimSpace(string(revision)),
		Modified: len(status) != 0,
	}}
}
