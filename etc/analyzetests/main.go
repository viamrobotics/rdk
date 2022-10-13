// Package main is a go test analyzer that publishes results to a MongoDB database.
package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/edaniels/golog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"gotest.tools/gotestsum/testjson"
)

var logger = golog.NewDebugLogger("analyzetests")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout: os.Stdin,
	})
	if err != nil {
		return err
	}

	mongoURI, ok := os.LookupEnv("MONGODB_TEST_OUTPUT_URI")
	if !ok || mongoURI == "" {
		logger.Warn("no MongoDB URI found; skipping")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	if err := client.Connect(ctx); err != nil {
		return err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return multierr.Combine(err, client.Disconnect(ctx))
	}
	defer func() {
		utils.UncheckedError(client.Disconnect(ctx))
	}()

	gitSHA, _ := os.LookupEnv("GITHUB_SHA")
	var gitHubRunID, gitHubJobID int64
	gitHubRunIDStr, ok := os.LookupEnv("GITHUB_RUN_ID")
	if ok {
		var err error
		gitHubRunID, err = strconv.ParseInt(gitHubRunIDStr, 10, 64)
		if err != nil {
			return err
		}
	}
	gitHubJobIDStr, ok := os.LookupEnv("GITHUB_JOB")
	if ok {
		var err error
		gitHubJobID, err = strconv.ParseInt(gitHubJobIDStr, 10, 64)
		if err != nil {
			return err
		}
	}

	createdAt := time.Now()
	parseTest := func(status string, tc testjson.TestCase) testResult {
		root, sub := tc.Test.Split()
		return testResult{
			CreatedAt:   createdAt,
			GitSHA:      gitSHA,
			GitHubRunID: gitHubRunID,
			GitHubJobID: gitHubJobID,
			Status:      status,
			Package:     tc.Package,
			Test:        tc.Test.Name(),
			RootTest:    root,
			SubTest:     sub,
			Elapsed:     tc.Elapsed.Milliseconds(),
		}
	}
	parseTests := func(status string, tcs []testjson.TestCase) []testResult {
		var results []testResult
		results = make([]testResult, 0, len(tcs))
		for _, tc := range tcs {
			results = append(results, parseTest(status, tc))
		}
		return results
	}

	var results []testResult
	for _, pkgName := range exec.Packages() {
		pkg := exec.Package(pkgName)
		results = append(results, parseTests("passed", pkg.Passed)...)
		results = append(results, parseTests("skipped", pkg.Skipped)...)
		results = append(results, parseTests("failed", testjson.FilterFailedUnique(pkg.Failed))...)
	}

	if len(results) == 0 {
		return nil
	}

	resultsIfc := make([]interface{}, 0, len(results))
	for _, res := range results {
		resultsIfc = append(resultsIfc, res)
	}

	coll := client.Database("tests").Collection("results")
	_, err = coll.InsertMany(context.Background(), resultsIfc)
	return err
}

type testResult struct {
	CreatedAt   time.Time `bson:"created_at"`
	GitSHA      string    `bson:"git_sha,omitempty"`
	GitHubRunID int64     `bson:"github_run_id,omitempty"`
	GitHubJobID int64     `bson:"github_job_id,omitempty"`
	Status      string    `bson:"status"`
	Package     string    `bson:"package"`
	Test        string    `bson:"test"`
	RootTest    string    `bson:"root_test"`
	SubTest     string    `bson:"sub_test,omitempty"`
	Elapsed     int64     `bson:"elapsed,omitempty"`
}
