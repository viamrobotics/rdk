// Package orderbench replays an ordered sequence of production plan requests captured from a
// single robot order, so that planner latency and trajectory quality can be compared across two
// RDK revisions.
//
// A single plan request in isolation says very little about the planner: armplanning carries
// learned roadmaps, smart-seed caches and a static-scene SDF registry across calls within a
// process, so the cost of a plan depends on which plans ran before it. Replaying a whole order in
// recorded order is what exercises that, and it is the shape production actually runs.
package orderbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManifestName is the file, committed alongside the corpus, that pins replay order and labels.
const ManifestName = "manifest.json"

// Entry describes one captured plan request within a corpus. Entries are ordered by RecordedAt,
// which is the order the robot actually planned them in.
type Entry struct {
	Index      int       `json:"index"`
	File       string    `json:"file"`
	RecordedAt time.Time `json:"recorded_at"`
	Step       string    `json:"step"`
	Motion     string    `json:"motion"`
	Outcome    string    `json:"outcome"`
	Bytes      int64     `json:"bytes"`
	SHA256     string    `json:"sha256"`
}

// Name is the stable label used to join a plan across two runs of the benchmark. It must not
// depend on timing or on the revision under test.
func (e Entry) Name() string {
	return fmt.Sprintf("%03d/%s/%s", e.Index, e.Step, e.Motion)
}

// Manifest is the committed index of a corpus. The payloads themselves are far too large for git
// and live in the artifact store; this file is what makes the replay reproducible: it fixes the
// order, the labels, and the content hashes.
type Manifest struct {
	// Order is the data-capture tag the corpus was exported from.
	Order string `json:"order"`
	// ArtifactPath is where the payloads live in the artifact tree.
	ArtifactPath string  `json:"artifact_path"`
	Entries      []Entry `json:"entries"`
}

// TotalBytes reports the on-disk size of the corpus payloads.
func (m *Manifest) TotalBytes() int64 {
	var total int64
	for _, e := range m.Entries {
		total += e.Bytes
	}
	return total
}

// WriteToFile writes the manifest as indented JSON, which keeps it reviewable in a diff.
func (m *Manifest) WriteToFile(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// LoadManifest reads a manifest from a corpus directory or from a direct path to the file.
func LoadManifest(path string) (*Manifest, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, ManifestName)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("manifest %q has no entries", path)
	}
	return &m, nil
}

// Verify checks that every payload named by the manifest is present in dir and unmodified. A
// corpus that has silently drifted would invalidate a cross-revision comparison, so the replay
// refuses to run without this passing.
func (m *Manifest) Verify(dir string) error {
	for _, e := range m.Entries {
		path := filepath.Join(dir, e.File)
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("corpus entry %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != e.SHA256 {
			return fmt.Errorf("corpus entry %s: content hash %s does not match manifest %s", e.Name(), got, e.SHA256)
		}
	}
	return nil
}
