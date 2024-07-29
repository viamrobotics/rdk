package cli

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// samePath returns true if abs(path1) and abs(path2) are the same.
func samePath(path1, path2 string) (bool, error) {
	abs1, err := filepath.Abs(path1)
	if err != nil {
		return false, err
	}
	abs2, err := filepath.Abs(path2)
	if err != nil {
		return false, err
	}
	return abs1 == abs2, nil
}

// getMapString is a helper that returns map_[key] if it exists and is a string, otherwise empty string.
func getMapString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return ""
		}
	}
	return ""
}

// replaceInTarGz adds entries to a .tar.gz file by the awkward method of copying
// its contents to a new file.
func replaceInTarGz(path string, newEntries map[string][]byte) error {
	tmpPath := path + ".tmp"
	// nested function so the Close() calls finish before we rename.
	err := func() error {
		// open reader
		sourceFile, err := os.Open(path) //nolint:gosec
		if err != nil {
			return err
		}
		defer sourceFile.Close() //nolint:errcheck
		gzRead, err := gzip.NewReader(sourceFile)
		if err != nil {
			return err
		}
		defer gzRead.Close() //nolint:errcheck
		tarRead := tar.NewReader(gzRead)

		// open writer
		destFile, err := os.Create(tmpPath) //nolint:gosec
		if err != nil {
			return err
		}
		defer destFile.Close() //nolint:errcheck
		gzw := gzip.NewWriter(destFile)
		tarWrite := tar.NewWriter(gzw)

		consumed := make(map[string]bool)
		// copy existing files, using newEntries where necessary
		for {
			head, err := tarRead.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}

			if entry, ok := newEntries[head.Name]; ok {
				head.Size = int64(len(entry))
				if err := tarWrite.WriteHeader(head); err != nil {
					return err
				}
				if _, err := tarWrite.Write(entry); err != nil {
					return err
				}
				consumed[head.Name] = true
			} else {
				if err := tarWrite.WriteHeader(head); err != nil {
					return err
				}
				if _, err := io.Copy(tarWrite, tarRead); err != nil { //nolint:gosec
					return err
				}
			}
		}

		// copy any remaining newEntries
		for name, entry := range newEntries {
			if consumed[name] {
				continue
			}
			if err := tarWrite.WriteHeader(&tar.Header{
				Typeflag: tar.TypeReg,
				Name:     name,
				Size:     int64(len(entry)),
				ModTime:  time.Now(),
			}); err != nil {
				return err
			}
			if _, err := tarWrite.Write(entry); err != nil {
				return err
			}
		}

		// cleanup
		if err := tarWrite.Close(); err != nil {
			return err
		}
		if err := gzw.Close(); err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		return err
	}
	// note: body of function is closure-scoped so the `defer close` actions finish
	// before the Rename below.
	return os.Rename(tmpPath, path)
}
