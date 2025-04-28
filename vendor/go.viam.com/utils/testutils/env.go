package testutils

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

var (
	logger            = golog.Global().Named("test")
	noSkip            = false
	internetConnected *bool
)

func skipWithError(tb testing.TB, err error) {
	tb.Helper()
	if noSkip {
		tb.Fatal(err)
		return
	}
	tb.Skip(err)
}

// SkipUnlessInternet verifies there is an internet connection.
func SkipUnlessInternet(tb testing.TB) {
	tb.Helper()
	if internetConnected == nil {
		var connected bool
		conn, err := net.DialTimeout("tcp", "mozilla.org:80", 5*time.Second)
		if err == nil {
			test.That(tb, conn.Close(), test.ShouldBeNil)
			connected = true
		}
		internetConnected = &connected
	}
	if *internetConnected {
		return
	}
	skipWithError(tb, errors.New("internet not connected"))
}

func artifactGoogleCreds() (string, error) {
	creds, ok := os.LookupEnv("ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS")
	if !ok || creds == "" {
		return "", errors.New("no artifact google credentials found")
	}
	return creds, nil
}

// SkipUnlessArtifactGoogleCreds verifies google credentials are available for artifact.
func SkipUnlessArtifactGoogleCreds(tb testing.TB) {
	tb.Helper()
	_, err := artifactGoogleCreds()
	if err == nil {
		return
	}
	skipWithError(tb, err)
}

// ArtifactGoogleCreds returns the google credentials for artifact.
func ArtifactGoogleCreds(tb testing.TB) string {
	tb.Helper()
	creds, err := artifactGoogleCreds()
	if err != nil {
		skipWithError(tb, err)
		return ""
	}
	return creds
}

func backingMongoDBURI() (string, error) {
	mongoURI, ok := os.LookupEnv("TEST_MONGODB_URI")
	if !ok || mongoURI == "" {
		return "", errors.New("no MongoDB URI found")
	}
	setupMongoDBForTests()
	return mongoURI, nil
}

// SkipUnlessBackingMongoDBURI verifies there is a backing MongoDB URI to use.
func SkipUnlessBackingMongoDBURI(tb testing.TB) {
	tb.Helper()
	_, err := backingMongoDBURI()
	if err == nil {
		return
	}
	skipWithError(tb, err)
}

// BackingMongoDBURI returns the backing MongoDB URI to use.
func BackingMongoDBURI(tb testing.TB) string {
	tb.Helper()
	mongoURI, err := backingMongoDBURI()
	if err != nil {
		skipWithError(tb, err)
		return ""
	}
	return mongoURI
}
