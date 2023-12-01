// Package main is a go test analyzer that publishes results to a MongoDB database.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"golang.org/x/tools/cover"

	"go.viam.com/rdk/logging"
)

var logger = logging.NewDebugLogger("analyzetests")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, _ []string, logger logging.Logger) error {
	profileDataAll, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	profileDatas := bytes.Split(profileDataAll, []byte("mode: "))

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
	baseSha, _ := os.LookupEnv("GITHUB_X_PR_BASE_SHA")

	branchName, _ := os.LookupEnv("GITHUB_X_HEAD_REF")

	mongoURI, ok := os.LookupEnv("MONGODB_TEST_OUTPUT_URI")
	if !ok || mongoURI == "" {
		logger.Warn("no MongoDB URI found; skipping")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	coll := client.Database("coverage").Collection("results")

	var closestPastResults *closetMergeBaseResults
	var closestPastResultsErr error
	if isPullRequest {
		closestPastResults, closestPastResultsErr = findClosestMergeBaseResults(
			ctx, coll, branchName, gitSHA, baseRef, baseSha)
	}

	createdAt := time.Now()

	covResults := map[string]coverageResult{}
	rawResults := map[string]*funcOutputResult{}
	for _, data := range profileDatas[1:] {
		newData := append([]byte("mode: "), data...)
		results, err := funcOutput(bytes.NewReader(newData))
		if err != nil {
			return err
		}
		for pkgName, result := range results {
			pctCovered := percent(result.covered, result.total)
			if _, ok := covResults[pkgName]; ok {
				newPct := pctCovered
				if pctCovered < covResults[pkgName].LineCoveragePct {
					newPct = covResults[pkgName].LineCoveragePct
				} else if pctCovered == covResults[pkgName].LineCoveragePct {
					continue
				}
				logger.CInfow(ctx, 
					"multiple coverage profiles for package; taking higher...",
					"package", pkgName,
					"prev", covResults[pkgName].LineCoveragePct,
					"new_entry", pctCovered,
					"new", newPct,
				)
				pctCovered = newPct
			}
			covResults[pkgName] = coverageResult{
				CreatedAt:           createdAt,
				GitSHA:              gitSHA,
				GitBranch:           branchName,
				GitHubRepository:    repository,
				GitHubRunID:         gitHubRunID,
				GitHubRunNumber:     gitHubRunNumber,
				GitHubRunAttempt:    gitHubRunAttempt,
				GitHubIsPullRequest: isPullRequest,
				Package:             pkgName,
				LineCoveragePct:     pctCovered,
				LinesCovered:        result.covered,
				LinesTotal:          result.total,
			}
			resultCopy := *result
			rawResults[pkgName] = &resultCopy
		}
	}

	if len(covResults) == 0 {
		return nil
	}

	resultsIfc := make([]interface{}, 0, len(covResults))
	for _, res := range covResults {
		resultsIfc = append(resultsIfc, res)
	}

	var totalCovered, totalLines int64
	for _, rawResult := range rawResults {
		totalCovered += rawResult.covered
		totalLines += rawResult.total
	}
	summaryResult := coverageResult{
		CreatedAt:           createdAt,
		GitSHA:              gitSHA,
		GitBranch:           branchName,
		GitHubRepository:    repository,
		GitHubRunID:         gitHubRunID,
		GitHubRunNumber:     gitHubRunNumber,
		GitHubRunAttempt:    gitHubRunAttempt,
		GitHubIsPullRequest: isPullRequest,
		LineCoveragePct:     percent(totalCovered, totalLines),
		LinesCovered:        totalCovered,
		LinesTotal:          totalLines,
	}
	resultsIfc = append(resultsIfc, summaryResult)

	if _, err := coll.InsertMany(ctx, resultsIfc); err != nil {
		logger.Errorw("error storing coverage results", "error", err)
	}

	mdOutput := generateMarkdownOutput(
		overallResults{covResults, summaryResult},
		generateBadge(summaryResult),
		closestPastResults,
		closestPastResultsErr,
	)
	//nolint:gosec
	return os.WriteFile("code-coverage-results.md", []byte(mdOutput), 0o644)
}

func generateBadge(summary coverageResult) string {
	var color string
	switch {
	case summary.LineCoveragePct < lowerThreshold:
		color = "critical"
	case summary.LineCoveragePct < upperThreshold:
		color = "yellow"
	default:
		color = "success"
	}
	return fmt.Sprintf("https://img.shields.io/badge/Code%%20Coverage-%.0f%%25-%s?style=flat", summary.LineCoveragePct, color)
}

const (
	lowerThreshold = 50
	upperThreshold = 70
)

func generateHealthIndicator(rate float64) string {
	switch {
	case rate < lowerThreshold:
		return "❌"
	case rate < upperThreshold:
		return "➖"
	default:
		return "✅"
	}
}

// based off of https://github.com/irongut/CodeCoverageSummary
func generateMarkdownOutput(
	results overallResults,
	badgeURL string,
	closestPastResults *closetMergeBaseResults,
	closestPastResultsErr error,
) string {
	var builder strings.Builder

	// The details/summary tags will auto-collapse the code coverage results. We keep the badge with
	// a percentage of coverage visible without needing to expand.
	builder.WriteString(fmt.Sprintf("![Code Coverage](%s)\n\n", badgeURL))
	builder.WriteString("<details>\n<summary>Code Coverage</summary>\n\n")

	if closestPastResultsErr != nil {
		builder.WriteString(fmt.Sprintf("**Note: %s**\n", closestPastResultsErr))
	}
	var canDelta bool
	if closestPastResults != nil {
		canDelta = len(closestPastResults.results.packages) != 0
		if !canDelta {
			builder.WriteString("**Note: no suitable past coverage found to compare against**\n")
		}
	}

	builder.WriteString("Package | Line Rate")
	if canDelta {
		builder.WriteString(" | Delta")
	}
	builder.WriteString(" | Health\n")
	builder.WriteString("-------- | ---------")
	if canDelta {
		builder.WriteString(" | ------")
	}
	builder.WriteString(" | ------\n")

	pkgNames := make([]string, 0, len(results.packages))
	for pkgName := range results.packages {
		pkgNames = append(pkgNames, pkgName)
	}
	sort.Strings(pkgNames)

	getDelta := func(now, past coverageResult) string {
		delta := now.LineCoveragePct - past.LineCoveragePct
		if math.Abs(delta) < 1e-2 {
			delta = 0
		}
		var deltaSign string
		if !(delta == 0 || math.Signbit(delta)) {
			deltaSign = "+"
		}
		deltaStr := fmt.Sprintf("%s%.2f%%", deltaSign, delta)
		if math.Abs(delta) > 5 {
			deltaStr = fmt.Sprintf("**%s**", deltaStr)
		}
		return deltaStr
	}
	for _, pkgName := range pkgNames {
		result := results.packages[pkgName]
		builder.WriteString(fmt.Sprintf("%s | %.0f%%", pkgName, result.LineCoveragePct))
		if canDelta {
			if pastResult, ok := closestPastResults.results.packages[pkgName]; ok {
				builder.WriteString(fmt.Sprintf(" | %s", getDelta(result, pastResult)))
			} else {
				builder.WriteString(" | N/A")
			}
		}
		builder.WriteString(fmt.Sprintf(" | %s\n", generateHealthIndicator(result.LineCoveragePct)))
	}

	builder.WriteString(fmt.Sprintf(
		"**Summary** | **%.0f%%** (%d / %d)",
		results.summary.LineCoveragePct,
		results.summary.LinesCovered,
		results.summary.LinesTotal))
	if canDelta {
		builder.WriteString(fmt.Sprintf(" | %s", getDelta(results.summary, closestPastResults.results.summary)))
	}
	builder.WriteString(fmt.Sprintf(" | %s\n", generateHealthIndicator(results.summary.LineCoveragePct)))

	// Close the detail tag from the beginning.
	builder.WriteString("</details>\n")

	return builder.String()
}

type funcOutputResult struct {
	covered int64
	total   int64
}

// *heavily* borrowed from https://cs.opensource.google/go/go/+/refs/tags/go1.19.2:src/cmd/cover/func.go

func funcOutput(profileData io.Reader) (map[string]*funcOutputResult, error) {
	profiles, err := cover.ParseProfilesFromReader(profileData)
	if err != nil {
		return nil, err
	}

	dirs, err := findPkgs(profiles)
	if err != nil {
		return nil, err
	}

	results := map[string]*funcOutputResult{}
	for _, profile := range profiles {
		fn := profile.FileName
		file, err := findFile(dirs, fn)
		if err != nil {
			return nil, err
		}
		funcs, err := findFuncs(file)
		if err != nil {
			return nil, err
		}
		// Now match up functions and profile blocks.
		for _, f := range funcs {
			c, t := f.coverage(profile)
			pkgName := dirs[path.Dir(fn)].ImportPath
			if results[pkgName] == nil {
				results[pkgName] = &funcOutputResult{}
			}
			results[pkgName].total += t
			results[pkgName].covered += c
		}
	}

	return results, nil
}

// findFuncs parses the file and returns a slice of FuncExtent descriptors.
func findFuncs(name string) ([]*FuncExtent, error) {
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		return nil, err
	}
	visitor := &FuncVisitor{
		fset:    fset,
		name:    name,
		astFile: parsedFile,
	}
	ast.Walk(visitor, visitor.astFile)
	return visitor.funcs, nil
}

// FuncExtent describes a function's extent in the source by file and position.
type FuncExtent struct {
	name      string
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

// FuncVisitor implements the visitor that builds the function position list for a file.
type FuncVisitor struct {
	fset    *token.FileSet
	name    string // Name of file.
	astFile *ast.File
	funcs   []*FuncExtent
}

// Visit implements the ast.Visitor interface.
func (v *FuncVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Body == nil {
			// Do not count declarations of assembly functions.
			break
		}
		start := v.fset.Position(n.Pos())
		end := v.fset.Position(n.End())
		fe := &FuncExtent{
			name:      n.Name.Name,
			startLine: start.Line,
			startCol:  start.Column,
			endLine:   end.Line,
			endCol:    end.Column,
		}
		v.funcs = append(v.funcs, fe)
	}
	return v
}

// coverage returns the fraction of the statements in the function that were covered, as a numerator and denominator.
func (f *FuncExtent) coverage(profile *cover.Profile) (num, den int64) {
	// We could avoid making this n^2 overall by doing a single scan and annotating the functions,
	// but the sizes of the data structures is never very large and the scan is almost instantaneous.
	var covered, total int64
	// The blocks are sorted, so we can stop counting as soon as we reach the end of the relevant block.
	for _, b := range profile.Blocks {
		if b.StartLine > f.endLine || (b.StartLine == f.endLine && b.StartCol >= f.endCol) {
			// Past the end of the function.
			break
		}
		if b.EndLine < f.startLine || (b.EndLine == f.startLine && b.EndCol <= f.startCol) {
			// Before the beginning of the function
			continue
		}
		total += int64(b.NumStmt)
		if b.Count > 0 {
			covered += int64(b.NumStmt)
		}
	}
	return covered, total
}

// Pkg describes a single package, compatible with the JSON output from 'go list'; see 'go help list'.
type Pkg struct {
	ImportPath string
	Dir        string
	Error      *struct {
		Err string
	}
}

func findPkgs(profiles []*cover.Profile) (map[string]*Pkg, error) {
	// Run go list to find the location of every package we care about.
	pkgs := make(map[string]*Pkg)
	var list []string
	for _, profile := range profiles {
		if strings.HasPrefix(profile.FileName, ".") || filepath.IsAbs(profile.FileName) {
			// Relative or absolute path.
			continue
		}
		pkg := path.Dir(profile.FileName)
		if _, ok := pkgs[pkg]; !ok {
			pkgs[pkg] = nil
			list = append(list, pkg)
		}
	}

	if len(list) == 0 {
		return pkgs, nil
	}

	// Note: usually run as "go tool cover" in which case $GOROOT is set,
	// in which case runtime.GOROOT() does exactly what we want.
	goTool := filepath.Join(runtime.GOROOT(), "bin/go")
	//nolint:gosec
	cmd := exec.Command(goTool, append([]string{"list", "-e", "-json"}, list...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cannot run go list: %w\n%s", err, stderr.Bytes())
	}
	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var pkg Pkg
		err := dec.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding go list json: %w", err)
		}
		pkgs[pkg.ImportPath] = &pkg
	}
	return pkgs, nil
}

// findFile finds the location of the named file in GOROOT, GOPATH etc.
func findFile(pkgs map[string]*Pkg, file string) (string, error) {
	if strings.HasPrefix(file, ".") || filepath.IsAbs(file) {
		// Relative or absolute path.
		return file, nil
	}
	pkg := pkgs[path.Dir(file)]
	if pkg != nil {
		if pkg.Dir != "" {
			return filepath.Join(pkg.Dir, path.Base(file)), nil
		}
		if pkg.Error != nil {
			return "", errors.New(pkg.Error.Err)
		}
	}
	return "", fmt.Errorf("did not find package for %s in go list output", file)
}

func percent(covered, total int64) float64 {
	if total == 0 {
		total = 1 // Avoid zero denominator.
	}
	return 100.0 * float64(covered) / float64(total)
}

type coverageResult struct {
	CreatedAt           time.Time `bson:"created_at"`
	GitSHA              string    `bson:"git_sha,omitempty"`
	GitBranch           string    `bson:"git_branch"`
	GitHubRepository    string    `bson:"github_repository"`
	GitHubRunID         int64     `bson:"github_run_id,omitempty"`
	GitHubRunNumber     int64     `bson:"github_run_number,omitempty"`
	GitHubRunAttempt    int64     `bson:"github_run_attempt,omitempty"`
	GitHubIsPullRequest bool      `bson:"github_is_pull_request"`
	LineCoveragePct     float64   `bson:"line_coverage_pct"`
	LinesCovered        int64     `bson:"lines_covered"`
	LinesTotal          int64     `bson:"lines_total"`
	Package             string    `bson:"package,omitempty"`
}

type overallResults struct {
	packages map[string]coverageResult
	summary  coverageResult
}

type closetMergeBaseResults struct {
	base    string
	results overallResults
}

func checkForResults(ctx context.Context, coll *mongo.Collection, gitSha string) []coverageResult {
	resultCount, err := coll.CountDocuments(ctx, bson.D{
		{"git_sha", gitSha},
	})
	if err != nil {
		logger.Errorw("failed to find coverage for merge base", "git_sha", gitSha, "error", err)
	}
	if resultCount == 0 {
		return nil
	}

	cur, err := coll.Find(ctx, bson.D{
		{"git_sha", gitSha},
	})
	if err != nil {
		logger.Errorw("failed to find coverage for merge base", "git_sha", gitSha, "error", err)
	}
	var results []coverageResult
	if err := cur.All(ctx, &results); err != nil {
		logger.Errorw("failed to find coverage for merge base", "git_sha", gitSha, "error", err)
	}
	return results
}

func findClosestMergeBaseResults(
	ctx context.Context,
	coll *mongo.Collection,
	branchName string,
	branchSha string,
	baseRef string,
	baseSha string,
) (*closetMergeBaseResults, error) {
	revParse := func(base string, back int) (string, error) {
		checkRef := fmt.Sprintf("%s~%d", base, back)
		//nolint:gosec
		cmd := exec.Command("git", "rev-parse", checkRef)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if len(out) != 0 {
				return "", fmt.Errorf("error running git rev-parse %s: %s", checkRef, string(out))
			}
			return "", err
		}
		return strings.TrimSuffix(string(out), "\n"), nil
	}

	// look back for results
	cmd := exec.Command("git", "merge-base", branchSha, baseSha)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) != 0 {
			return nil, fmt.Errorf(
				"error running git merge-base %s(%s) %s(%s): %s",
				branchName,
				branchSha,
				baseRef,
				baseSha,
				string(out),
			)
		}
		return nil, err
	}
	possibleMergeBaseRoot := strings.TrimSuffix(string(out), "\n")
	possibleMergeBaseCurr := possibleMergeBaseRoot

	var mergeBase string
	var mergeBaseResults []coverageResult
	mergeBaseCovResults := map[string]coverageResult{}
	var mergeBaseSummaryResult coverageResult

	var mergeBaseErr error
	for i := 0; i < 5; i++ {
		if i != 0 {
			var err error
			possibleMergeBaseCurr, err = revParse(possibleMergeBaseRoot, i)
			if err != nil {
				return nil, err
			}
		}

		mergeBaseResults = checkForResults(ctx, coll, possibleMergeBaseCurr)
		if len(mergeBaseResults) == 0 {
			continue
		}
		mergeBase = possibleMergeBaseCurr
		if i != 0 {
			mergeBaseErr = fmt.Errorf(
				"merge base coverage results not available, comparing against closest %s~%d=(%s) instead",
				possibleMergeBaseRoot, i, mergeBase)
		}
		break
	}

	if mergeBase == "" {
		// last resort, try the HEAD at this point in time of base ref
		mergeBaseResults = checkForResults(ctx, coll, baseSha)
		if len(mergeBaseResults) != 0 {
			mergeBase = baseSha
			mergeBaseErr = fmt.Errorf(
				"merge base coverage results not available, using HEAD(%s)=%s as no other closest candidate was found",
				baseRef, baseSha)
		} else {
			return nil, errors.New("failed to find any suitable merge base to compare against")
		}
	}

	// dedupe
	for _, result := range mergeBaseResults {
		resultCopy := result
		if result.Package == "" {
			mergeBaseSummaryResult = resultCopy
			continue
		}
		mergeBaseCovResults[result.Package] = resultCopy
	}
	return &closetMergeBaseResults{
		base: mergeBase,
		results: overallResults{
			packages: mergeBaseCovResults,
			summary:  mergeBaseSummaryResult,
		},
	}, mergeBaseErr
}
