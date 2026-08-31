package orderbench

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.viam.com/utils/artifact"
)

// DefaultCorpus is the corpus replayed when none is named: one full espresso-drink order captured
// from a Cappuccina machine, 98 plans over about seven and a half minutes.
const DefaultCorpus = "cappuccina-5fb95a4c"

// corpora holds the committed manifests. They are embedded rather than read from disk so the
// harness stays self-contained when it is compiled inside a checkout of an older revision, which is
// how a base-vs-head comparison is built.
//
//go:embed corpora/*.json
var corpora embed.FS

// ListCorpora names every manifest compiled into this binary.
func ListCorpora() []string {
	entries, err := corpora.ReadDir("corpora")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	return names
}

// LoadEmbeddedManifest returns the committed manifest for a named corpus.
func LoadEmbeddedManifest(name string) (*Manifest, error) {
	if name == "" {
		name = DefaultCorpus
	}
	data, err := corpora.ReadFile(filepath.Join("corpora", name+".json"))
	if err != nil {
		return nil, fmt.Errorf("no corpus named %q is compiled in; have %v", name, ListCorpora())
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Resolve fetches a corpus's payloads from the artifact store and returns the directory holding
// them. The payloads run to about 100 MB, far past what belongs in git, so the manifest names an
// artifact path and this pulls it on demand -- the same mechanism the rest of the repo's large test
// fixtures use.
//
// The returned directory is verified against the manifest before use: a corpus that has silently
// drifted would invalidate every comparison made against it.
func Resolve(m *Manifest) (string, error) {
	if m.ArtifactPath == "" {
		return "", fmt.Errorf("corpus %q has no artifact path", m.Order)
	}

	dir, err := artifact.Path(m.ArtifactPath)
	if err != nil {
		return "", fmt.Errorf("fetching corpus %q from the artifact store: %w", m.ArtifactPath, err)
	}

	// Nothing is written into the resolved directory: it is the artifact cache, and a stray file
	// there would be swept up by the next `artifact push` as though it were corpus payload.
	if err := m.Verify(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ResolveCorpusDir works out which directory a replay should read, given either an explicit local
// directory or the name of an embedded corpus.
func ResolveCorpusDir(dir, name string) (string, *Manifest, error) {
	if dir != "" {
		manifest, err := LoadManifest(dir)
		if err != nil {
			return "", nil, err
		}
		return dir, manifest, manifest.Verify(dir)
	}

	manifest, err := LoadEmbeddedManifest(name)
	if err != nil {
		return "", nil, err
	}
	resolved, err := Resolve(manifest)
	if err != nil {
		return "", nil, err
	}
	return resolved, manifest, nil
}

// StageCorpus materializes a corpus into dstDir as symlinks to the artifact cache, plus the
// manifest. Useful when a run wants a stable path (a CI step, say) without copying 100 MB.
func StageCorpus(srcDir string, m *Manifest, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return err
	}
	srcAbs, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range m.Entries {
		link := filepath.Join(dstDir, entry.File)
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Symlink(filepath.Join(srcAbs, entry.File), link); err != nil {
			return err
		}
	}
	return m.WriteToFile(filepath.Join(dstDir, ManifestName))
}
