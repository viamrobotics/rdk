package sync

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
)

// handleOrphanedOpenSequences should:
//   - move a lone .progseq to failed/sequences/
//   - delete a .progseq that has a matching .seq (we crashed mid-rename)
//   - leave a lone .seq untouched
//   - delete any stray .tmp (partial atomic write from a crash)
//   - no-op gracefully when sequences/ doesn't exist
func TestHandleOrphanedOpenSequences(t *testing.T) {
	captureDir := t.TempDir()
	seqDir := filepath.Join(captureDir, data.SequencesDir)
	test.That(t, os.MkdirAll(seqDir, 0o700), test.ShouldBeNil)

	orphanProg := filepath.Join(seqDir, "orphan"+data.InProgressSequenceFileExt)
	dupProg := filepath.Join(seqDir, "dup"+data.InProgressSequenceFileExt)
	dupSeq := filepath.Join(seqDir, "dup"+data.CompletedSequenceFileExt)
	loneSeq := filepath.Join(seqDir, "lone"+data.CompletedSequenceFileExt)
	strayTmp := filepath.Join(seqDir, "garbage"+data.CompletedSequenceFileExt+".tmp")
	movedOrphan := filepath.Join(captureDir, FailedDir, data.SequencesDir, "orphan"+data.InProgressSequenceFileExt)

	for _, p := range []string{orphanProg, dupProg, dupSeq, loneSeq, strayTmp} {
		test.That(t, os.WriteFile(p, []byte("{}"), 0o600), test.ShouldBeNil)
	}

	handleOrphanedOpenSequences(captureDir, logging.NewTestLogger(t))

	// A captureDir with no sequences/ subdir is a clean restart with nothing to clean up:
	// the sweep should also not create the failed/ tree as a side effect.
	emptyCaptureDir := t.TempDir()
	handleOrphanedOpenSequences(emptyCaptureDir, logging.NewTestLogger(t))
	emptyFailedDir := filepath.Join(emptyCaptureDir, FailedDir)

	for _, tc := range []struct {
		desc        string
		path        string
		shouldExist bool
	}{
		{desc: "orphan .progseq removed from sequences/", path: orphanProg, shouldExist: false},
		{desc: "orphan moved into failed/", path: movedOrphan, shouldExist: true},
		{desc: "dup .progseq deleted", path: dupProg, shouldExist: false},
		{desc: "dup .seq untouched", path: dupSeq, shouldExist: true},
		{desc: "lone .seq untouched", path: loneSeq, shouldExist: true},
		{desc: "stray .tmp removed", path: strayTmp, shouldExist: false},
		{desc: "no sequences directory, failed/ not created", path: emptyFailedDir, shouldExist: false},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := os.Stat(tc.path)
			if tc.shouldExist {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
			}
		})
	}
}
