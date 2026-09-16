//go:build !windows

package packages

// describePathHolder returns a human-readable description of the process(es) holding path open, or
// "" when that can't be determined. Only Windows keeps a directory pinned as a process's working
// directory in a way that blocks deletion/rename, so on every other platform there is nothing to
// report and describeReplaceFailure falls back to its generic wording.
func describePathHolder(_ string) string {
	return ""
}
