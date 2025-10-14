// partial provides partial download support using range headers
package partial

// important: if you modify this file, you must run ./cmd/partial-dl in addition to running the test suite.
// The test suite contains mocks, but you must test against a live server too.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.viam.com/rdk/logging"
)

const (
	partSuffix = ".part"
	etagSuffix = ".etag"
)

var InterruptedDownload = errors.New("interrupting download to respect MaxRead")

// manages on-disk state and decision tree for partial downloads
type PartialDownloader struct {
	Client *http.Client
	Logger logging.Logger
	// maximum bytes to read in one call to Download(). used only for testing.
	MaxRead int
	// turn off the resume path for testing
	DontResume bool
}

type downloadState struct {
	// the ultimate destination of the download
	dest string
	// the stat of dest + '.part'
	partInfo os.FileInfo
	// contents of dest + '.etag'
	etag string
}

func getDownloadState(dest string) (*downloadState, error) {
	stat, err := os.Stat(dest + partSuffix)
	if err != nil {
		return nil, err
	}
	etag, err := os.ReadFile(dest + etagSuffix)
	if err != nil {
		return nil, err
	}
	return &downloadState{dest: dest, partInfo: stat, etag: string(etag)}, nil
}

// entrypoint for the partial download process
func (p *PartialDownloader) Download(ctx context.Context, url, dest string) error {
	state, err := getDownloadState(dest)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if p.DontResume {
		p.Logger.Info("setting state=nil because the p.DontResume test flag is set")
		state = nil
	}
	if state == nil {
		p.Logger.Debug("no partial, downloading from start")
		return p.downloadFromStart(ctx, url, dest)
	}
	p.Logger.Debug("partial found, resuming")
	return p.resumeDownload(ctx, url, dest, state)
}

func (p *PartialDownloader) downloadFromStart(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status %q", resp.Status)
	}
	return p.downloadResponseFromStart(ctx, dest, resp)
}

// inner from-start logic for downloadFromstart and the fail case in resumeDownload
func (p *PartialDownloader) downloadResponseFromStart(ctx context.Context, dest string, resp *http.Response) error {
	var destFile *os.File
	var err error

	// todo: confirm there are no other values than 'bytes' for this header
	isPartial := false
	if resp.Header.Get("Accept-Ranges") == "bytes" && resp.Header.Get("Etag") != "" {
		p.Logger.Debugf("headers found (%q, %q), starting partial", resp.Header.Get("Accept-Ranges"), resp.Header.Get("Etag"))
		if err := os.WriteFile(dest+etagSuffix, []byte(resp.Header.Get("Etag")), 0o755); err != nil {
			return err
		}
		isPartial = true
		destFile, err = os.Create(dest + partSuffix)
	} else {
		p.Logger.Debugf("missing range or etag header (%q, %q), downloading without resume",
			resp.Header.Get("Accept-Ranges"), resp.Header.Get("Etag"))
		destFile, err = os.Create(dest)
	}
	if err != nil {
		return err
	}
	defer destFile.Close()
	if err := p.copyWithProgress(ctx, resp, destFile); err != nil {
		return err
	}
	if isPartial {
		return p.cleanup(dest, true)
	}
	return nil
}

// try to resume a download; if not resumable, clean up state and download from start instead.
func (p *PartialDownloader) resumeDownload(ctx context.Context, url, dest string, state *downloadState) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-", state.partInfo.Size()))
	req.Header.Add("If-Match", state.etag)
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		fallthrough // I think 200 here means the server isn't serving a range; treat this as an error
	case http.StatusPreconditionFailed:
		p.Logger.Debug("precondition failed, downloading from start")
		if err := p.cleanup(dest, false); err != nil {
			return err
		}
		return p.downloadFromStart(ctx, url, dest)
	case http.StatusPartialContent:
		break
	default:
		return fmt.Errorf("unexpected status %q", resp.Status)
	}

	p.Logger.Debug("precondition succeeded, beginning resume")
	destFile, err := os.OpenFile(dest+partSuffix, os.O_APPEND|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if err := p.copyWithProgress(ctx, resp, destFile); err != nil {
		return err
	}
	return p.cleanup(dest, true)
}

// rename part to dest if success, otherwise delete part. delete etag. call this after a download succeeds or fails irrecoverably.
func (p *PartialDownloader) cleanup(dest string, success bool) error {
	// note: failures here can cause resume to break; the code that uses PartialDownloader needs to check checksum after finishing.
	var err error
	if success {
		p.Logger.Debugf("renaming %q -> %q", dest+partSuffix, dest)
		err = errors.Join(err, os.Rename(dest+partSuffix, dest))
	} else {
		err = errors.Join(err, os.Remove(dest+partSuffix))
	}
	return errors.Join(err, os.Remove(dest+etagSuffix))
}

func (p *PartialDownloader) copyWithProgress(_ context.Context, resp *http.Response, destFile *os.File) error {
	// todo: wrap in progress
	// todo: no Copy with context?
	if p.MaxRead != 0 {
		_, err := io.CopyN(destFile, resp.Body, int64(p.MaxRead))
		if err != nil {
			return err
		}
		return InterruptedDownload
	}
	_, err := io.Copy(destFile, resp.Body)
	return err
}
