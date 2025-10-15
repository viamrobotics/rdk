package partial

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

var (
	contents      = []byte("contents contents contents contents contents contents")
	otherContents = []byte("other other other other other other")
)

func TestPartialDownloader(t *testing.T) {
	// mock httptest server for exercising the different start-resume paths
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/non-resumable":
			w.Write(contents)
		case "/resumable":
			// note: this is a mock and may not match actual server implementation.
			// You need to test with `go run ./cmd/partial-dl` as well if you change these code paths.
			query := r.URL.Query()
			slice := contents
			if query.Get("other") == "yes" {
				slice = otherContents
			}
			ifmatch := r.Header.Get("If-Match")
			// note: we read etag from the query of the request so that each test can control what it gets back.
			// A normal server would manage etags itself, obviously.
			if ifmatch != "" && ifmatch != query.Get("etag") {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			if r.Header.Get("Range") != "" {
				byteRange, _ := strconv.ParseInt(regexp.MustCompile(`bytes=(\d+)\-`).FindStringSubmatch(r.Header.Get("Range"))[1], 10, 64)
				slice = slice[byteRange:]
				w.WriteHeader(http.StatusPartialContent)
			}
			w.Header().Add("Etag", query.Get("etag"))
			w.Header().Add("Accept-Ranges", "bytes")
			w.Write(slice)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	cwd, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { os.Chdir(cwd) })
	test.That(t, os.Chdir(t.TempDir()), test.ShouldBeNil)

	logger := logging.NewTestLogger(t)

	t.Run("from-scratch", func(t *testing.T) {
		dest := "scratch.txt"
		pd := Downloader{Client: server.Client(), Logger: logger}
		err := pd.Download(context.Background(), server.URL+"/non-resumable", dest)
		test.That(t, err, test.ShouldBeNil)
		body, _ := os.ReadFile(dest)
		test.That(t, body, test.ShouldResemble, contents)
	})

	t.Run("resume", func(t *testing.T) {
		t.Run("etag-match", func(t *testing.T) {
			dest := "etag-match.txt"
			pd := Downloader{Client: server.Client(), Logger: logger, MaxRead: 10}
			err := pd.Download(context.Background(), server.URL+"/resumable?etag=match", dest)
			test.That(t, err, test.ShouldBeError, ErrInterruptedDownload)
			body, _ := os.ReadFile(dest + partSuffix)
			test.That(t, body, test.ShouldHaveLength, 10)

			pd.MaxRead = 0
			logger.Info("resuming with SAME etag")
			err = pd.Download(context.Background(), server.URL+"/resumable?etag=match", dest)
			test.That(t, err, test.ShouldBeNil)
			body, _ = os.ReadFile(dest)
			test.That(t, body, test.ShouldResemble, contents)
		})

		t.Run("etag-mismatch", func(t *testing.T) {
			dest := "etag-mismatch.txt"
			pd := Downloader{Client: server.Client(), Logger: logger, MaxRead: 10}
			err := pd.Download(context.Background(), server.URL+"/resumable?etag=match1", dest)
			test.That(t, err, test.ShouldBeError, ErrInterruptedDownload)
			body, _ := os.ReadFile(dest + partSuffix)
			test.That(t, body, test.ShouldHaveLength, 10)

			pd.MaxRead = 0
			logger.Info("resuming with DIFFERENT etag")
			// this should trigger the If-Match precondition fail, which should put us into a from-scratch download.
			err = pd.Download(context.Background(), server.URL+"/resumable?etag=match2&other=yes", dest)
			test.That(t, err, test.ShouldBeNil)
			body, _ = os.ReadFile(dest)
			test.That(t, body, test.ShouldHaveLength, len(otherContents))
			test.That(t, body, test.ShouldResemble, otherContents)
		})
	})
}
