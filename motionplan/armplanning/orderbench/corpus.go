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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ManifestName is the file, committed alongside the corpus, that pins replay order and labels.
const ManifestName = "manifest.json"

// recordedTimeLayout matches the timestamp prefix that the data capture pipeline puts on every
// exported plan file, e.g. "20260828_153733.046_carry.json".
const recordedTimeLayout = "20060102_150405.000"

var exportFileRe = regexp.MustCompile(`^(\d{8}_\d{6}\.\d{3})_(.+)\.json$`)

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

// tagLabels pulls the `tag=<key>_<value>` path components the export uses to encode capture
// labels. The order step, the motion kind and the planning outcome all arrive this way.
func tagLabels(relPath string) map[string]string {
	labels := map[string]string{}
	for part := range strings.SplitSeq(filepath.ToSlash(relPath), "/") {
		tag, ok := strings.CutPrefix(part, "tag=")
		if !ok {
			continue
		}
		key, value, ok := strings.Cut(tag, "_")
		if !ok {
			continue
		}
		labels[key] = value
	}
	return labels
}

func copyFile(src, dst string) (string, error) {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return "", err
	}
	defer in.Close() //nolint:errcheck

	out, err := os.OpenFile(filepath.Clean(dst), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, hasher), in); err != nil {
		out.Close() //nolint:errcheck
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
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
