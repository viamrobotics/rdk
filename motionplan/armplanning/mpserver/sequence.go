package mpserver

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
)

// sequenceTmpl renders a directory of plan files as an ordered list with a little detail on each,
// and a form for planning a run of them as one armplanning.PlanMotionSequence call.
var sequenceTmpl = template.Must(template.New("sequence").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Plan Sequence: {{.Dir}}</title>
<style>
  body {
    background-color: azure;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    margin: 20px;
  }
  h1 { color: #333; }
  table {
    background-color: #D0EEFF;
    border-spacing: 8px;
    border: 1px solid black;
  }
  th, td {
    background-color: bisque;
    border: 1px solid black;
    padding: 4px 8px;
    text-align: left;
  }
  button {
    padding: 4px 8px;
    border: 1px solid black;
    background-color: #D0EEFF;
    cursor: pointer;
  }
  .controls { margin: 12px 0; }
  .controls label { display: block; margin-bottom: 8px; }
  .controls button { display: block; }
  .error { color: #a00; }
  .muted { color: #555; }
  #result { margin-top: 12px; }
  details { margin: 8px 0; }
  summary { cursor: pointer; font-weight: 600; }
  caption { text-align: left; font-weight: 600; }
  .has-rejected { background-color: #ffd54f; font-weight: 600; }
  .half-turns { background-color: #6a1b9a; color: white; font-weight: 600; }
  .same { background-color: #1b5e20; color: white; font-weight: 600; }
  .note { font-weight: 600; }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
</style>
</head>
<body>
<p><a href="/">&larr; All plan files</a></p>
<h1>Plan Sequence</h1>
<p>{{.Dir}} <span class="muted">({{len .Plans}} plan(s))</span></p>
{{if not .Plans}}<p class="muted">No plan files in this directory.</p>{{else}}
<div class="controls">
  <label>First plan
    <select id="first">
      {{range .Plans}}<option value="{{.Index}}"{{if eq .Index $.FirstIndex}} selected{{end}}>{{.Number}}. {{.Name}}</option>{{end}}
    </select>
  </label>
  <label>Last plan
    <select id="last">
      {{range .Plans}}<option value="{{.Index}}"{{if eq .Index $.LastIndex}} selected{{end}}>{{.Number}}. {{.Name}}</option>{{end}}
    </select>
  </label>
  <label>Backtrack branch factor
    <input type="number" id="branch" min="1" step="1" value="{{.BranchFactor}}" style="width: 4em">
  </label>
  <button id="run" onclick="runSequence()">PlanMotionSequence</button>
</div>
<details id="plans-table" open>
<summary>Plans and constraints</summary>
<table>
  <tr><th>#</th><th>File</th><th>Goals</th><th>Linear constraints</th><th>Orientation constraints</th><th>Recorded plan steps</th></tr>
  {{range .Plans}}
  <tr>
    <td>{{.Number}}</td>
    <td><a href="/detail?file={{.Path}}">{{.Name}}</a></td>
    {{if .Error}}<td colspan="4" class="error">{{.Error}}</td>{{else}}
    <td>{{.Goals}}</td>
    <td>{{.LinearConstraints}}</td>
    <td>{{.OrientationConstraints}}</td>
    <td>{{if .RecordedSteps}}{{.RecordedSteps}}{{else}}-{{end}}</td>
    {{end}}
  </tr>
  {{end}}
</table>
</details>
<div id="result"></div>
{{end}}
<script>
const DIR = {{.Dir}};

function escapeHtml(text) {
  const element = document.createElement('div');
  element.textContent = text;
  return element.innerHTML;
}

let lastPlans = [];
// Bumped by every run and single-plan render, so an older renderAll loop notices it was superseded.
let renderGeneration = 0;

function renderPlan(plan) {
  return fetch('/render-plan?file=' + encodeURIComponent(plan.path), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(plan.trajectory),
  }).then(response => { if (!response.ok) response.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

function renderOne(planIdx) {
  renderGeneration++;
  renderPlan(lastPlans[planIdx]);
}

// Each render request returns once its animation finishes, so awaiting them plays the plans in order.
async function renderAll() {
  const generation = ++renderGeneration;
  for (const plan of lastPlans) {
    if (generation !== renderGeneration) return;
    if (plan.planned) await renderPlan(plan);
  }
}

function foldable(title, body, id) {
  return '<details open' + (id ? ' id="' + id + '"' : '') + '><summary>' + title + '</summary>' + body + '</details>';
}

// Opens the folds around a plan's backtracking tables so the in-page link lands on something visible.
function showBacktrack(planNumber) {
  document.getElementById('backtracking').open = true;
  document.getElementById('backtrack-plan-' + planNumber).open = true;
}

function resultsTable(plans) {
  let html = '<table><tr><th>#</th><th>File</th><th>Planned</th><th>Steps</th><th>Plan duration</th>' +
    '<th>Times planned</th><th>Rejected solutions</th><th></th></tr>';
  plans.forEach((plan, planIdx) => {
    html += '<tr><td>' + plan.number + '</td><td>' + escapeHtml(plan.name) + '</td><td>' +
      (plan.planned ? 'yes' : 'no') + '</td><td>' + (plan.planned ? plan.steps : '-') + '</td><td>' +
      escapeHtml(plan.duration || '-') + '</td><td>' + plan.times_planned + '</td><td' +
      (plan.rejected_solutions > 0 ? ' class="has-rejected"' : '') + '>' + plan.rejected_solutions +
      (plan.backtrack ? ' (<a href="#backtrack-plan-' + plan.number + '" onclick="showBacktrack(' + plan.number +
        ')">Details</a>)' : '') + '</td><td>' +
      (plan.planned ? '<button onclick="renderOne(' + planIdx + ')">Render</button>' : '') + '</td></tr>';
  });
  return foldable('Results', html + '</table>');
}

// One table per frame of each plan that had solutions rejected.
function backtrackTables(plans) {
  let html = '';
  plans.forEach(plan => {
    if (!plan.backtrack) return;
    let body = '<p class="muted">' + plan.rejected_solutions + ' rejected solution(s), planned ' +
      plan.times_planned + ' time(s).</p>';
    (plan.backtrack.notes || []).forEach(note => { body += '<p class="note">' + escapeHtml(note) + '</p>'; });
    plan.backtrack.frames.forEach(frame => {
      body += '<table><caption>frame ' + escapeHtml(frame.name) + '</caption><tr><th>joint</th>';
      plan.backtrack.columns.forEach(column => { body += '<th>' + escapeHtml(column) + '</th>'; });
      body += '</tr>';
      frame.joints.forEach((row, jointIdx) => {
        body += '<tr><td>' + jointIdx + '</td>';
        row.forEach(cell => {
          body += '<td class="num' + (cell.same ? ' same' : (cell.half_turns ? ' half-turns' : '')) + '">' + cell.value + '</td>';
        });
        body += '</tr>';
      });
      body += '<tr><th>Linf</th>';
      frame.linf.forEach(value => { body += '<td class="num">' + value + '</td>'; });
      body += '</tr><tr><th>Cost</th>';
      frame.cost.forEach(value => { body += '<td class="num">' + value + '</td>'; });
      body += '</tr></table>';
    });
    html += foldable('Backtracking: plan ' + plan.number + ' ' + escapeHtml(plan.name), body, 'backtrack-plan-' + plan.number);
  });
  if (!html) return '';
  const legend = '<p class="muted">Each table compares a plan\'s start, its successful end (or, when the sequence failed, ' +
    'its last successful end) and each rejected end. Linf is the largest per-joint difference from the successful end; ' +
    '* marks a difference within 0.01, which the planner treats as a repeat. Cost is the configuration distance ' +
    '(L2 unless the request says otherwise) over all frames from the start each solution was planned from; ' +
    '&dagger; marks a rejected solution planned from a different start than the successful one. The planner also ' +
    'adds a small bias for joints far from mid-range, not shown. Rejected joints: ' +
    '<span class="same">same value as the successful end</span>, ' +
    '<span class="half-turns">a whole number of half turns (&pi;) from it</span>.</p>';
  return foldable('Backtracking', legend + html, 'backtracking');
}

function runSequence() {
  const first = document.getElementById('first').value;
  const last = document.getElementById('last').value;
  const result = document.getElementById('result');
  const button = document.getElementById('run');
  button.disabled = true;
  // Plan numbers, so the link reads like the page: the table's # column.
  const branch = document.getElementById('branch').value;
  const shareParams = new URLSearchParams({dir: DIR, first: Number(first) + 1, last: Number(last) + 1, branch: branch});
  history.replaceState(null, '', '/sequence?' + shareParams);
  document.getElementById('plans-table').open = false;
  renderGeneration++;
  result.innerHTML = 'Planning...';
  const params = new URLSearchParams({dir: DIR, first: first, last: last, branch: branch});
  fetch('/sequence/run?' + params)
    .then(response => response.json())
    .then(data => {
      let html = '';
      if (data.error) {
        html += '<p class="error">' + escapeHtml(data.error) + '</p>';
      } else {
        html += '<p>Planned ' + data.plans.length + ' plan(s) in ' + escapeHtml(data.duration) + '.</p>';
      }
      if (data.plans && data.plans.length) {
        html += resultsTable(data.plans) + backtrackTables(data.plans);
      }
      result.innerHTML = html;
      lastPlans = data.plans || [];
      renderAll();
    })
    .catch(err => { result.innerHTML = '<p class="error">' + escapeHtml(String(err)) + '</p>'; })
    .finally(() => { button.disabled = false; });
}
</script>
</body>
</html>
`))

// sequencePlan is one plan file in a directory shown on the sequence page.
type sequencePlan struct {
	// Index is the position in the directory's lexical order, from zero. Number is Index + 1, as
	// shown to the user.
	Index  int
	Number int
	Name   string
	// Path is relative to rdkRoot, which is what the other handlers expect.
	Path                   string
	Goals                  int
	LinearConstraints      int
	OrientationConstraints int
	// RecordedSteps is the trajectory length of the plan recorded alongside the request, or zero for
	// a request-only file.
	RecordedSteps int
	// Error is set when the file could not be read as a plan request.
	Error string
}

type sequencePageData struct {
	Dir        string
	Plans      []sequencePlan
	FirstIndex int
	LastIndex  int
	// BranchFactor is how many endings of a plan are tried from one start state before backtracking
	// further.
	BranchFactor int
}

// resolvePlanDir validates a directory relative to rdkRoot that must sit inside the plan files
// root, and returns its absolute path.
func resolvePlanDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("missing dir parameter")
	}

	absolute := filepath.Join(rdkRoot, filepath.Clean(dir))
	relative, err := filepath.Rel(planFilesRoot, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q is not inside the plan files root", dir)
	}

	return absolute, nil
}

// planFilesInDir lists the .json files directly inside a directory, as paths relative to rdkRoot,
// in lexical order. That is the order a directory of recorded plans is run in.
func planFilesInDir(absoluteDir string) ([]string, error) {
	entries, err := os.ReadDir(absoluteDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}

		relative, err := filepath.Rel(rdkRoot, filepath.Join(absoluteDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		files = append(files, relative)
	}

	return files, nil
}

func describeSequencePlan(index int, path string) sequencePlan {
	plan := sequencePlan{Index: index, Number: index + 1, Name: filepath.Base(path), Path: path}

	request, recorded, err := armplanning.ReadRequestAndResponseFromFile(filepath.Join(rdkRoot, path))
	if err != nil {
		plan.Error = fmt.Sprintf("reading plan file: %v", err)
		return plan
	}

	plan.Goals = len(request.Goals)
	if request.Constraints != nil {
		plan.LinearConstraints = len(request.Constraints.LinearConstraint)
		plan.OrientationConstraints = len(request.Constraints.OrientationConstraint)
	}
	if recorded != nil {
		plan.RecordedSteps = len(recorded.Trajectory())
	}

	return plan
}

func handleSequence(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		absoluteDir, err := resolvePlanDir(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		files, err := planFilesInDir(absoluteDir)
		if err != nil {
			http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}

		data := sequencePageData{
			Dir:          filepath.ToSlash(filepath.Clean(dir)),
			LastIndex:    len(files) - 1,
			BranchFactor: armplanning.NewDefaultSequenceOptions().BacktrackBranchFactor,
		}
		if branch, err := strconv.Atoi(r.URL.Query().Get("branch")); err == nil && branch >= 1 {
			data.BranchFactor = branch
		}
		if firstNumber, lastNumber := r.URL.Query().Get("first"), r.URL.Query().Get("last"); firstNumber != "" && lastNumber != "" {
			// A shared link only preselects the dropdowns, so a range that no longer fits falls back to the defaults.
			if first, last, ok := parseSequenceNumbers(firstNumber, lastNumber, len(files)); ok {
				data.FirstIndex, data.LastIndex = first, last
			}
		}
		for index, file := range files {
			data.Plans = append(data.Plans, describeSequencePlan(index, filepath.ToSlash(file)))
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := sequenceTmpl.Execute(w, data); err != nil {
			logger.Errorf("rendering sequence page: %v", err)
		}
	}
}

// sequenceRunPlan is what planning one request of a sequence produced.
type sequenceRunPlan struct {
	Number            int    `json:"number"`
	Name              string `json:"name"`
	Planned           bool   `json:"planned"`
	Steps             int    `json:"steps"`
	Duration          string `json:"duration,omitempty"`
	TimesPlanned      int    `json:"times_planned"`
	RejectedSolutions int    `json:"rejected_solutions"`
	// Path is relative to rdkRoot and Trajectory is the plan's steps, together what /render-plan
	// needs to animate the plan.
	Path       string                `json:"path"`
	Trajectory []map[string][]string `json:"trajectory,omitempty"`
	Backtrack  *sequenceBacktrack    `json:"backtrack,omitempty"`
}

type sequenceRunResult struct {
	Error    string            `json:"error,omitempty"`
	Duration string            `json:"duration,omitempty"`
	Plans    []sequenceRunPlan `json:"plans,omitempty"`
}

// parseSequenceRange reads the first and last plan indexes, both inclusive, and checks them
// against the number of plans in the directory.
func parseSequenceRange(firstParam, lastParam string, count int) (int, int, error) {
	first, err := strconv.Atoi(firstParam)
	if err != nil {
		return 0, 0, fmt.Errorf("bad first parameter %q", firstParam)
	}

	last, err := strconv.Atoi(lastParam)
	if err != nil {
		return 0, 0, fmt.Errorf("bad last parameter %q", lastParam)
	}

	if first < 0 || last >= count || first > last {
		return 0, 0, fmt.Errorf("plans %d to %d is not a range within the %d plans in the directory", first+1, last+1, count)
	}

	return first, last, nil
}

// parseSequenceNumbers reads the plan numbers, 1-based and inclusive, that a shared page link
// carries and returns them as indexes, or false when they are not a range within count plans.
func parseSequenceNumbers(firstParam, lastParam string, count int) (int, int, bool) {
	firstNumber, firstErr := strconv.Atoi(firstParam)
	lastNumber, lastErr := strconv.Atoi(lastParam)
	if firstErr != nil || lastErr != nil || firstNumber < 1 || lastNumber > count || firstNumber > lastNumber {
		return 0, 0, false
	}

	return firstNumber - 1, lastNumber - 1, true
}

func handleSequenceRun(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		absoluteDir, err := resolvePlanDir(r.URL.Query().Get("dir"))
		if err != nil {
			writeJSON(w, sequenceRunResult{Error: err.Error()})
			return
		}

		files, err := planFilesInDir(absoluteDir)
		if err != nil {
			writeJSON(w, sequenceRunResult{Error: fmt.Sprintf("scan error: %v", err)})
			return
		}

		first, last, err := parseSequenceRange(r.URL.Query().Get("first"), r.URL.Query().Get("last"), len(files))
		if err != nil {
			writeJSON(w, sequenceRunResult{Error: err.Error()})
			return
		}

		sequenceOptions := armplanning.NewDefaultSequenceOptions()
		if branchParam := r.URL.Query().Get("branch"); branchParam != "" {
			branch, err := strconv.Atoi(branchParam)
			if err != nil || branch < 1 {
				writeJSON(w, sequenceRunResult{Error: fmt.Sprintf("bad branch parameter %q, want an integer of at least 1", branchParam)})
				return
			}
			sequenceOptions.BacktrackBranchFactor = branch
		}

		files = files[first : last+1]
		requests := make([]*armplanning.PlanRequest, 0, len(files))
		for _, file := range files {
			request, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
			if err != nil {
				writeJSON(w, sequenceRunResult{Error: fmt.Sprintf("reading %s: %v", file, err)})
				return
			}
			requests = append(requests, request)
		}

		armplanning.ClearSeedCache()

		start := time.Now()
		plans, metas, err := armplanning.PlanMotionSequence(r.Context(), logger, requests, sequenceOptions)

		result := sequenceRunResult{Duration: time.Since(start).Truncate(time.Millisecond).String()}
		if err != nil {
			result.Error = err.Error()
		}

		for index, file := range files {
			entry := sequenceRunPlan{Number: first + index + 1, Name: filepath.Base(file), Path: file}
			if plans[index] != nil {
				entry.Planned = true
				entry.Steps = len(plans[index].Path())
				entry.Trajectory = planTrajectoryToStrings(plans[index])
			}
			if metas[index] != nil {
				entry.Duration = metas[index].Duration.Truncate(time.Millisecond).String()
				entry.TimesPlanned = metas[index].BacktrackCounter + 1
				entry.RejectedSolutions = len(metas[index].RejectedSolutions)
				entry.Backtrack = buildSequenceBacktrack(requests[index], plans[index], metas[index], err == nil)
			}
			result.Plans = append(result.Plans, entry)
		}

		writeJSON(w, result)
	}
}
