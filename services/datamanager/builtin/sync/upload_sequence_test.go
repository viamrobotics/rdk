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
	t.Run("mixed_entries", func(t *testing.T) {
		captureDir := t.TempDir()
		seqDir := filepath.Join(captureDir, data.SequencesDir)
		test.That(t, os.MkdirAll(seqDir, 0o700), test.ShouldBeNil)

		orphanProg := filepath.Join(seqDir, "orphan"+data.InProgressSequenceFileExt)
		dupProg := filepath.Join(seqDir, "dup"+data.InProgressSequenceFileExt)
		dupSeq := filepath.Join(seqDir, "dup"+data.CompletedSequenceFileExt)
		loneSeq := filepath.Join(seqDir, "lone"+data.CompletedSequenceFileExt)
		strayTmp := filepath.Join(seqDir, "garbage"+data.CompletedSequenceFileExt+".tmp")

		for _, p := range []string{orphanProg, dupProg, dupSeq, loneSeq, strayTmp} {
			test.That(t, os.WriteFile(p, []byte("{}"), 0o600), test.ShouldBeNil)
		}

		handleOrphanedOpenSequences(captureDir, logging.NewTestLogger(t))

		// orphan .progseq → moved to failed/
		_, err := os.Stat(orphanProg)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
		movedOrphan := filepath.Join(captureDir, FailedDir, data.SequencesDir, "orphan"+data.InProgressSequenceFileExt)
		_, err = os.Stat(movedOrphan)
		test.That(t, err, test.ShouldBeNil)

		// dup .progseq deleted, dup .seq untouched
		_, err = os.Stat(dupProg)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
		_, err = os.Stat(dupSeq)
		test.That(t, err, test.ShouldBeNil)

		// lone .seq untouched
		_, err = os.Stat(loneSeq)
		test.That(t, err, test.ShouldBeNil)

		// stray .tmp removed
		_, err = os.Stat(strayTmp)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	})

	t.Run("no_sequences_dir", func(t *testing.T) {
		captureDir := t.TempDir()
		handleOrphanedOpenSequences(captureDir, logging.NewTestLogger(t))

		_, err := os.Stat(filepath.Join(captureDir, FailedDir))
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	})
}
