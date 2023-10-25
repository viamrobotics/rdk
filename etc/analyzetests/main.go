// Package main is a go test analyzer that publishes results to a MongoDB database.
package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"gotest.tools/gotestsum/testjson"

	"go.viam.com/rdk/logging"
)

var logger = logging.NewDebugLogger("analyzetests")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
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

	gitSHA, _ := os.LookupEnv("GITHUB_X_HEAD_SHA")
	repository, _ := os.LookupEnv("GITHUB_REPOSITORY")
	var gitHubRunID, gitHubRunNumber, gitHubRunAttempt int64
	gitHubRunIDStr, ok := os.LookupEnv("GITHUB_RUN_ID")
	if ok {
		var err error
		gitHubRunID, err = strconv.ParseInt(gitHubRunIDStr, 10, 64)
		if err != nil {
			return err
		}
	}
	gitHubRunNumberStr, ok := os.LookupEnv("GITHUB_RUN_NUMBER")
	if ok {
		var err error
		gitHubRunNumber, err = strconv.ParseInt(gitHubRunNumberStr, 10, 64)
		if err != nil {
			return err
		}
	}
	gitHubRunAttemptStr, ok := os.LookupEnv("GITHUB_RUN_ATTEMPT")
	if ok {
		var err error
		gitHubRunAttempt, err = strconv.ParseInt(gitHubRunAttemptStr, 10, 64)
		if err != nil {
			return err
		}
	}

	baseRef, ok := os.LookupEnv("GITHUB_X_PR_BASE_REF")
	isPullRequest := ok && baseRef != ""

	branchName, _ := os.LookupEnv("GITHUB_X_HEAD_REF")

	createdAt := time.Now()
	parseTest := func(status string, tc testjson.TestCase) testResult {
		root, sub := tc.Test.Split()
		return testResult{
			CreatedAt:           createdAt,
			GitSHA:              gitSHA,
			GitBranch:           branchName,
			GitHubRepository:    repository,
			GitHubRunID:         gitHubRunID,
			GitHubRunNumber:     gitHubRunNumber,
			GitHubRunAttempt:    gitHubRunAttempt,
			GitHubIsPullRequest: isPullRequest,
			Status:              status,
			Package:             tc.Package,
			Test:                tc.Test.Name(),
			RootTest:            root,
			SubTest:             sub,
			Elapsed:             tc.Elapsed.Milliseconds(),
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
	CreatedAt           time.Time `bson:"created_at"`
	GitSHA              string    `bson:"git_sha,omitempty"`
	GitBranch           string    `bson:"git_branch"`
	GitHubRepository    string    `bson:"github_repository"`
	GitHubRunID         int64     `bson:"github_run_id,omitempty"`
	GitHubRunNumber     int64     `bson:"github_run_number,omitempty"`
	GitHubRunAttempt    int64     `bson:"github_run_attempt,omitempty"`
	GitHubIsPullRequest bool      `bson:"github_is_pull_request"`
	Status              string    `bson:"status"`
	Package             string    `bson:"package"`
	Test                string    `bson:"test"`
	RootTest            string    `bson:"root_test"`
	SubTest             string    `bson:"sub_test,omitempty"`
	Elapsed             int64     `bson:"elapsed,omitempty"`
}
