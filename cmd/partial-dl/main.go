// partial-dl is a test entrypoint for the partial downloader logic.
// there is a mock in the test suite, but you must run this if you modify the utils/partial.go logic.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/partial"
)

var url = flag.String("url", "https://storage.googleapis.com/packages.viam.com/apps/viam-server/viam-server-latest-x86_64", "URL to use for testing")
var dest = flag.String("dest", "partial-dl-test", "base output path")
var maxRead = flag.Int("max-read", 1000000, "amount to read in first pass of resumption tests. set to less than size of your file")
var testCase = flag.String("case", "", "pass 'non', 'resume', 'mismatch' to run a specific case, leave blank to run all three")

func main() {
	flag.Parse()
	logger := logging.NewDebugLogger("partial-dl")
	pd := partial.PartialDownloader{Client: http.DefaultClient, Logger: logger}

	ctx := context.Background()
	if *testCase == "" || *testCase == "non" {
		pd.DontResume = true
		logger.Info("non-resumable download case")
		if err := pd.Download(ctx, *url, *dest+"-nr"); err != nil {
			panic(err)
		}
		pd.DontResume = false
	}

	if *testCase == "" || *testCase == "resume" {
		logger.Info("resumable download case")
		pd.MaxRead = *maxRead
		if err := pd.Download(ctx, *url, *dest+"-r"); err != nil && !errors.Is(err, partial.InterruptedDownload) {
			panic(err)
		}

		logger.Info("resuming")
		println("\nATTN USER: make sure 'precondition succeeded' logs below\n")
		pd.MaxRead = 0
		if err := pd.Download(ctx, *url, *dest+"-r"); err != nil {
			panic(err)
		}
	}

	if *testCase == "" || *testCase == "mismatch" {
		logger.Info("etags mismatch case")
		pd.MaxRead = *maxRead
		if err := pd.Download(ctx, *url, *dest+"-et"); err != nil && !errors.Is(err, partial.InterruptedDownload) {
			panic(err)
		}

		logger.Info("overwriting etag with garbage")
		if err := os.WriteFile(*dest+"-et.etag", []byte(`"GARBAGEEEEE"`), 0o755); err != nil {
			panic(err)
		}

		logger.Info("resuming")
		println("\nATTN USER: make sure 'precondition failed' logs below\n")
		pd.MaxRead = 0
		if err := pd.Download(ctx, *url, *dest+"-et"); err != nil {
			panic(err)
		}
	}

	logger.Info("tests complete")
	println("\nATTN USER: run `md5sum partial-dl-test-*` and make sure they all match\n")
}
