package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/go-errors/errors"
	"go.viam.com/test"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	gcphttp "google.golang.org/api/transport/http"
	iampb "google.golang.org/genproto/googleapis/iam/v1"

	"go.viam.com/core/testutils"
	"go.viam.com/core/utils"
)

func TestNewGoogleStorageStore(t *testing.T) {
	testutils.SkipUnlessInternet(t)
	testutils.SkipUnlessArtifactGoogleCreds(t)

	_, err := newGoogleStorageStore(&googleStorageStoreConfig{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bucket required")

	undo := setGoogleCredsPath("")
	defer undo()
	store, err := newGoogleStorageStore(&googleStorageStoreConfig{Bucket: "somebucket"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, store.Close(), test.ShouldBeNil)

	setGoogleCredsPath("unknownpath")
	store, err = newGoogleStorageStore(&googleStorageStoreConfig{Bucket: "somebucket"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")

	credsPath := testutils.ArtifactGoogleCreds(t)
	var creds map[string]interface{}
	credsRd, err := ioutil.ReadFile(credsPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, json.Unmarshal(credsRd, &creds), test.ShouldBeNil)
	projectID, ok := creds["project_id"].(string)
	test.That(t, ok, test.ShouldBeTrue)

	httpTransport := &http.Transport{}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: httpTransport})
	gcpTransport, err := gcphttp.NewTransport(ctx, httpTransport, option.WithCredentialsFile(credsPath), option.WithScopes(storage.ScopeFullControl))
	test.That(t, err, test.ShouldBeNil)

	client, err := storage.NewClient(context.Background(), option.WithHTTPClient(&http.Client{Transport: gcpTransport}))
	test.That(t, err, test.ShouldBeNil)
	bucketName := fmt.Sprintf("test-viam-%s", strings.ToLower(utils.RandomAlphaString(32)))

	bucket := client.Bucket(bucketName)
	test.That(t, bucket.Create(context.Background(), projectID, nil), test.ShouldBeNil)
	t.Cleanup(func() {
		defer func() {
			httpTransport.CloseIdleConnections()
			test.That(t, client.Close(), test.ShouldBeNil)
		}()
		bucket := client.Bucket(bucketName)
		objectsIter := client.Bucket(bucketName).Objects(context.Background(), nil)
		for {
			objAttrs, err := objectsIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				t.Logf("error getting objects: %s", err)
				break
			}
			if err := bucket.Object(objAttrs.Name).Delete(context.Background()); err != nil {
				t.Logf("error deleting object: %s", err)
			}
		}
		test.That(t, bucket.Delete(context.Background()), test.ShouldBeNil)
	})

	policy, err := bucket.IAM().V3().Policy(context.Background())
	test.That(t, err, test.ShouldBeNil)
	role := "roles/storage.objectViewer"
	policy.Bindings = append(policy.Bindings, &iampb.Binding{
		Role:    role,
		Members: []string{iam.AllUsers},
	})
	test.That(t, bucket.IAM().V3().SetPolicy(ctx, policy), test.ShouldBeNil)

	setGoogleCredsPath(testutils.ArtifactGoogleCreds(t))
	store, err = newGoogleStorageStore(&googleStorageStoreConfig{Bucket: bucketName})
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, store.Close(), test.ShouldBeNil)
	}()

	testStore(t, store, false)

	setGoogleCredsPath(testutils.ArtifactGoogleCreds(t))
	sameStore, err := newGoogleStorageStore(&googleStorageStoreConfig{Bucket: bucketName})
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, sameStore.Close(), test.ShouldBeNil)
	}()
	testStore(t, sameStore, true)

	setGoogleCredsPath("")
	readOnlyStore, err := newGoogleStorageStore(&googleStorageStoreConfig{Bucket: bucketName})
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, readOnlyStore.Close(), test.ShouldBeNil)
	}()
	testStore(t, readOnlyStore, true)
}
