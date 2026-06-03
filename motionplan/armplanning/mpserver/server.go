// Package mpserver is a webserver for diagnosing motion plans.
//
//nolint:lll // HTML templates contain long lines that cannot be split
package mpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	viz "github.com/viam-labs/motion-tools/client/client"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// rdkRoot is resolved from this source file's location so the server runs
// against whichever checkout it was built from. server.go lives at
// motionplan/armplanning/mpserver/server.go — three directories deep from
// the repo root.
var (
	rdkRoot       = resolveRDKRoot()
	planFilesRoot = filepath.Join(rdkRoot, "mplans")
)

func resolveRDKRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

const (
	renderFramePeriod = 5 * time.Millisecond
	// shadowCount is the number of intermediate configurations to draw between start and end when
	// rendering shadows along a straight-line path. We interpolate directly instead of going through
	// InterpolateSegmentFS because that helper also enforces a per-joint step size (~range/1000)
	// which produces hundreds-to-thousands of steps for typical arm motions.
	shadowCount       = 10
	shadowFramePeriod = 100 * time.Millisecond
)

// ---- render coordination ----
//
// All rendering targets a single shared visualizer (the package-level `viz`
// client), so only one render may own it at a time. Renders are driven by
// independent HTTP requests that may originate from different pages (e.g. the
// detail page's trajectory playback and the IK-inspect page's "Render Start +
// Goals" button), so coordination has to live here on the server rather than in
// any single page's JavaScript. beginRender cancels whatever render is currently
// in flight and returns a context that the next render will cancel in turn;
// long-running renders (trajectory playback, shadows) must check it and bail out
// promptly once superseded.
var (
	renderMu     sync.Mutex
	renderCancel context.CancelFunc
)

func beginRender() context.Context {
	renderMu.Lock()
	defer renderMu.Unlock()
	if renderCancel != nil {
		renderCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	renderCancel = cancel
	return ctx
}

// ---- templates ----

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Motion Plan Files</title>
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
  }
  button {
    padding: 4px 8px;
    border: 1px solid black;
    background-color: #D0EEFF;
    cursor: pointer;
  }
</style>
</head>
<body>
<h1>Motion Plan Files</h1>
<table>
  <tr><th>File</th><th>Visualize</th><th>Details</th></tr>
  {{range .}}
  <tr>
    <td>{{.}}</td>
    <td><button onclick="renderStart('{{.}}')">Render State</button></td>
    <td><a href="/detail?file={{.}}">Details</a></td>
  </tr>
  {{end}}
</table>
<script>
function renderStart(file) {
  fetch('/render-start?file=' + encodeURIComponent(file))
    .then(r => { if (!r.ok) r.text().then(msg => alert('Error: ' + msg)); })
    .catch(err => alert('Error: ' + err));
}
</script>
</body>
</html>
`))

var detailTmpl = template.Must(template.New("detail").Parse(`<!DOCTYPE html>
<html>
<head>
<title>{{.File}} — Motion Plan Detail</title>
<style>
  body {
    background-color: azure;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    margin: 20px;
  }
  h1, h2 { color: #333; }
  table {
    background-color: #D0EEFF;
    border-spacing: 8px;
    border: 1px solid black;
  }
  th, td {
    background-color: bisque;
    border: 1px solid black;
    padding: 4px 8px;
    vertical-align: top;
  }
  button {
    padding: 4px 8px;
    border: 1px solid black;
    background-color: #D0EEFF;
    cursor: pointer;
  }
  pre {
    background-color: bisque;
    border: 1px solid black;
    padding: 8px;
    white-space: pre-wrap;
  }
  #result { margin-top: 16px; }
</style>
</head>
<body>
<h1>{{.File}}</h1>
<a href="/">← Back</a>

<h2>Motion Planning</h2>
<label>Timeout (seconds): <input id="timeout" type="number" min="0" step="1" value="0" style="width:6ch; padding:4px 8px; border:1px solid black;"></label>
&nbsp;<label>Seed: <input id="seed" type="number" step="1" value="0" style="width:6ch; padding:4px 8px; border:1px solid black;"></label>
&nbsp;<button onclick="runPlanning()">Do Motion Planning</button>
&nbsp;<button onclick="renderState()">Render Start State</button>
<div id="result"></div>

<h2>Start Inputs</h2>
{{if .StartInputs}}
<table>
  <tr><th>Frame</th><th>Inputs</th></tr>
  {{range .StartInputs}}
  <tr>
    <td>{{.Name}}</td>
    <td><code>{{.Inputs}}</code></td>
  </tr>
  {{end}}
</table>
{{else}}
<p><em>No moving-frame inputs in start state.</em></p>
{{end}}

<h2>Goals</h2>
{{if .Goals}}
{{range .Goals}}
<h3>Goal {{.Index}} — <a href="{{.IKInspectURL}}">Inspect IK</a></h3>
<h4>Start Configuration</h4>
{{if .StartConfig}}
<table>
  <tr><th>Frame</th><th>Inputs</th></tr>
  {{range .StartConfig}}
  <tr><td>{{.Name}}</td><td><code>{{.Inputs}}</code></td></tr>
  {{end}}
</table>
{{else}}
<p><em>No moving-frame inputs.</em></p>
{{end}}
<h4>Goal Poses (World Frame)</h4>
{{if .GoalPoses}}
<ul>{{range .GoalPoses}}<li><strong>{{.Frame}}</strong>: <code>{{.Pose}}</code></li>{{end}}</ul>
{{else}}
<p><em>No goal poses.</em></p>
{{end}}
{{end}}
{{else}}
<p><em>No goals.</em></p>
{{end}}

<h2>Frame System</h2>
<table>
  <tr><th>Frame</th><th>DoF</th><th>Parent</th></tr>
  {{range .Frames}}
  <tr>
    <td>{{.Name}}</td>
    <td>{{.DoF}}</td>
    <td>{{.Parent}}</td>
  </tr>
  {{end}}
</table>

<script>
function renderState() {
  fetch('/render-start?file=' + encodeURIComponent('{{.File}}'))
    .then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

renderState();

let planAbortController = null;

function runPlanning() {
  if (planAbortController) {
    planAbortController.abort();
  }
  planAbortController = new AbortController();
  const div = document.getElementById('result');
  const timeout = document.getElementById('timeout').value;
  const seed = document.getElementById('seed').value;
  div.textContent = 'Running…';
  fetch('/plan/run?file=' + encodeURIComponent('{{.File}}') +
        '&timeout=' + encodeURIComponent(timeout) +
        '&seed=' + encodeURIComponent(seed),
        { signal: planAbortController.signal })
    .then(r => r.json())
    .then(data => {
      if (data.error) {
        div.innerHTML = '<pre style="color:#cc0000">Error: ' + data.error + '</pre>';
        return;
      }
      let html = '<p><strong>Steps:</strong> ' + data.steps +
                 ' &nbsp; <strong>Duration:</strong> ' + data.duration +
                 ' &nbsp; <strong>Goals processed:</strong> ' + data.goals_processed + '</p>';
      (data.per_goal || []).forEach((pg, goalIdx) => {
        let goalHeader = 'Goal ' + goalIdx;
        if (pg.ik_inspect_url) goalHeader += ' — <a href="' + escHtml(pg.ik_inspect_url) + '">Inspect IK</a>';
        html += '<h3>' + goalHeader + '</h3>';
        html += buildSolutionTable('{{.File}}', 'Valid solutions', pg.valid_solutions || [], false);
        html += buildSolutionTable('{{.File}}', 'checkPath failures', pg.check_path_failures || [], true);
        if (pg.constraint_failures_by_type && Object.keys(pg.constraint_failures_by_type).length) {
          html += '<h4>Constraint failures</h4><table><tr><th>Constraint</th><th>Count</th></tr>';
          for (const [k, v] of Object.entries(pg.constraint_failures_by_type)) {
            html += '<tr><td>' + escHtml(k) + '</td><td>' + v + '</td></tr>';
          }
          html += '</table>';
        }
      });
      div.innerHTML = html;
      if (data.trajectory && data.trajectory.length) {
        renderPlan('{{.File}}', data.trajectory);
      }
    })
    .catch(err => { if (err.name !== 'AbortError') div.textContent = 'Fetch error: ' + err; });
}

function renderPlan(file, trajectory) {
  fetch('/render-plan?file=' + encodeURIComponent(file), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(trajectory),
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

function buildSolutionTable(file, title, solutions, showError) {
  if (!solutions.length) return '';
  let html = '<h4>' + title + ' (' + solutions.length + ')</h4>';
  html += '<table><tr><th>Score</th><th>Inputs</th>';
  if (showError) html += '<th>Error</th><th>Last good inputs</th>';
  html += '</tr>';
  for (const sn of solutions) {
    html += '<tr><td>' + sn.score.toFixed(4) + '</td>';
    const inputsArg = JSON.stringify(sn.inputs);
    html += '<td><code>' + formatInputs(sn.inputs) + '</code><br>' +
            '<button onclick=\'renderSolution(' + JSON.stringify(file) + ',' + inputsArg + ')\'>Render</button> ' +
            '<button onclick=\'renderShadows(' + JSON.stringify(file) + ',' + inputsArg + ')\'>Shadows</button></td>';
    if (showError) {
      html += '<td>' + escHtml(sn.check_path_error) + '</td>';
      if (sn.last_good_inputs) {
        const lastArg = JSON.stringify(sn.last_good_inputs);
        html += '<td><code>' + formatInputs(sn.last_good_inputs) + '</code><br>' +
                '<button onclick=\'renderSolution(' + JSON.stringify(file) + ',' + lastArg + ')\'>Render</button> ' +
                '<button onclick=\'renderShadows(' + JSON.stringify(file) + ',' + lastArg + ')\'>Shadows</button></td>';
      } else {
        html += '<td></td>';
      }
    }
    html += '</tr>';
  }
  html += '</table>';
  return html;
}

// inputs is map[string][]string — values are already full-precision strings from the server.
function formatInputs(inputs) {
  return Object.entries(inputs)
    .map(([f, vs]) => f + ': [' + vs.join(', ') + ']')
    .join('<br>');
}

function renderSolution(file, inputs) {
  fetch('/render-solution?file=' + encodeURIComponent(file), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(inputs),
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

function renderShadows(file, inputs) {
  fetch('/render-shadows?file=' + encodeURIComponent(file), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(inputs),
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Shadows error: ' + msg)); })
    .catch(err => console.error('Shadows error: ' + err));
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body>
</html>
`))

var ikInspectTmpl = template.Must(template.New("ik-inspect").Parse(`<!DOCTYPE html>
<html>
<head>
<title>IK Inspect — {{.File}}</title>
<style>
  body {
    background-color: azure;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    margin: 20px;
  }
  h1, h2 { color: #333; }
  table {
    background-color: #D0EEFF;
    border-spacing: 8px;
    border: 1px solid black;
  }
  th, td {
    background-color: bisque;
    border: 1px solid black;
    padding: 4px 8px;
    vertical-align: top;
  }
  button {
    padding: 4px 8px;
    border: 1px solid black;
    background-color: #D0EEFF;
    cursor: pointer;
  }
  #ik-table td.cell-green  { background-color: #b6e7b0; }
  #ik-table td.cell-yellow { background-color: #f3e6a0; }
  #ik-table td.cell-red    { background-color: #f0b0b0; }
  #ik-table td.cell-empty  { background-color: #e8e8e8; }
  #ik-table td { text-align: right; font-variant-numeric: tabular-nums; cursor: default; }
  #ik-table th { text-align: center; }
  .legend span { display: inline-block; padding: 2px 8px; border: 1px solid black; margin-right: 8px; }
</style>
</head>
<body>
<h1>IK Inspect</h1>
<p>{{.File}} — <a href="/detail?file={{.File}}">← Back to Detail</a></p>

<button onclick="renderStartAndGoals()">Render Start + Goals</button>
&nbsp;<button onclick="runIKInspect()">Run IK Inspection</button>

<div id="ik-result" style="margin-top:16px;"></div>

<h2>Start Configuration</h2>
{{if .StartConfig}}
<table>
  <tr><th>Frame</th><th>Inputs</th></tr>
  {{range .StartConfig}}
  <tr>
    <td>{{.Name}}</td>
    <td><code>{{.Inputs}}</code></td>
  </tr>
  {{end}}
</table>
{{else}}
<p><em>No moving-frame inputs in start state.</em></p>
{{end}}

<h2>Goal Poses (World Frame)</h2>
{{if .GoalPoses}}
<ul>
  {{range .GoalPoses}}<li><strong>{{.Frame}}</strong>: <code>{{.Pose}}</code></li>{{end}}
</ul>
{{else}}
<p><em>No goal poses.</em></p>
{{end}}

<script>
const START_CONFIG = {{.StartConfigJSON}};
const GOAL_POSES = {{.GoalPosesJSON}};

function renderStartAndGoals() {
  fetch('/render-start?file=' + encodeURIComponent('{{.File}}'))
    .then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

let ikInspectAbort = null;

function runIKInspect() {
  if (ikInspectAbort) ikInspectAbort.abort();
  ikInspectAbort = new AbortController();
  const div = document.getElementById('ik-result');
  div.textContent = 'Running IK inspection…';
  fetch('/ik-inspect/run?file=' + encodeURIComponent('{{.File}}') +
        '&start_config=' + encodeURIComponent(JSON.stringify(START_CONFIG)) +
        '&goal_poses=' + encodeURIComponent(JSON.stringify(GOAL_POSES)),
        { signal: ikInspectAbort.signal })
    .then(r => r.json())
    .then(data => {
      if (data.error) {
        div.innerHTML = '<pre style="color:#cc0000">Error: ' + escHtml(data.error) + '</pre>';
        return;
      }
      div.innerHTML = buildIKTable(data.threads || []);
    })
    .catch(err => { if (err.name !== 'AbortError') div.textContent = 'Fetch error: ' + err; });
}

// threads is column-major: threads[col] is the ordered list of solutions thread col emitted.
function buildIKTable(threads) {
  const legend = '<p class="legend">' +
    '<span class="cell-green" style="background:#b6e7b0">valid + checkPath ok</span>' +
    '<span class="cell-yellow" style="background:#f3e6a0">valid, checkPath failed</span>' +
    '<span class="cell-red" style="background:#f0b0b0">invalid (e.g. collision)</span>' +
    '</p>';

  let maxRows = 0;
  for (const col of threads) maxRows = Math.max(maxRows, col.length);
  if (maxRows === 0) return legend + '<p><em>No IK solutions were emitted.</em></p>';

  let html = legend + '<table id="ik-table"><tr><th></th>';
  for (let c = 0; c < threads.length; c++) html += '<th>thread ' + c + '</th>';
  html += '</tr>';
  for (let row = 0; row < maxRows; row++) {
    html += '<tr><th>' + row + '</th>';
    for (let c = 0; c < threads.length; c++) {
      const cell = threads[c][row];
      if (!cell) { html += '<td class="cell-empty"></td>'; continue; }
      html += renderIKCell(cell);
    }
    html += '</tr>';
  }
  html += '</table>';
  return html;
}

function renderIKCell(cell) {
  let cls = 'cell-red';
  if (cell.valid && cell.check_path_ok) cls = 'cell-green';
  else if (cell.valid) cls = 'cell-yellow';

  const tip = [];
  tip.push('cost: ' + cell.cost);
  tip.push('goalDist: ' + cell.goal_dist + (cell.exact ? ' (exact)' : ''));
  if (cell.state_error) tip.push('invalid: ' + cell.state_error);
  if (cell.check_path_error) tip.push('checkPath: ' + cell.check_path_error);

  const goalDist = (typeof cell.goal_dist === 'number') ? cell.goal_dist.toExponential(2) : cell.goal_dist;
  const inner = '<strong>' + cell.cost.toFixed(4) + '</strong><br><small>d=' + goalDist + '</small>';
  return '<td class="' + cls + '" title="' + escHtml(tip.join('\n')) + '">' + inner + '</td>';
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body>
</html>
`))

// ---- data types ----

type frameInfo struct {
	Name   string
	DoF    int
	Parent string
}

type frameInputs struct {
	Name   string
	Inputs string
}

// poseDisplay holds a single frame's pose as a human-readable string for template rendering.
type poseDisplay struct {
	Frame string
	Pose  string
}

type goalDetail struct {
	Index        int
	StartConfig  []frameInputs
	GoalPoses    []poseDisplay
	IKInspectURL string
}

type detailData struct {
	File        string
	Frames      []frameInfo
	StartInputs []frameInputs
	Goals       []goalDetail
}

type ikInspectData struct {
	File            string
	StartConfig     []frameInputs
	StartConfigJSON template.JS
	GoalPoses       []poseDisplay
	GoalPosesJSON   template.JS
}

type planRunResult struct {
	Error          string                `json:"error,omitempty"`
	Steps          int                   `json:"steps,omitempty"`
	Duration       string                `json:"duration,omitempty"`
	GoalsProcessed int                   `json:"goals_processed,omitempty"`
	Partial        bool                  `json:"partial,omitempty"`
	PartialError   string                `json:"partial_error,omitempty"`
	PerGoal        []perGoalResult       `json:"per_goal,omitempty"`
	Trajectory     []map[string][]string `json:"trajectory,omitempty"`
}

type perGoalResult struct {
	IKInspectURL             string               `json:"ik_inspect_url,omitempty"`
	ValidSolutions           []solutionNodeResult `json:"valid_solutions,omitempty"`
	CheckPathFailures        []solutionNodeResult `json:"check_path_failures,omitempty"`
	ConstraintFailuresByType map[string]int       `json:"constraint_failures_by_type,omitempty"`
}

// solutionNodeResult uses string-valued inputs to preserve full float64 precision across
// the Go→JSON→JS→JSON→Go round-trip.
type solutionNodeResult struct {
	Score          float64             `json:"score"`
	CheckPathError string              `json:"check_path_error,omitempty"`
	Inputs         map[string][]string `json:"inputs"`
	LastGoodInputs map[string][]string `json:"last_good_inputs,omitempty"`
}

// ikInspectRunResult is the JSON payload for the IK-inspect table. Threads is column-major:
// Threads[i] is the ordered list of solutions nlopt thread i emitted.
type ikInspectRunResult struct {
	Error   string                  `json:"error,omitempty"`
	Threads [][]ikInspectCellResult `json:"threads,omitempty"`
}

// ikInspectCellResult is one solution emitted by one thread. Inputs use string-valued floats to
// preserve precision, matching solutionNodeResult.
type ikInspectCellResult struct {
	Cost           float64             `json:"cost"`
	GoalDist       float64             `json:"goal_dist"`
	Exact          bool                `json:"exact"`
	Inputs         map[string][]string `json:"inputs,omitempty"`
	Valid          bool                `json:"valid"`
	StateError     string              `json:"state_error,omitempty"`
	CheckPathOK    bool                `json:"check_path_ok"`
	CheckPathError string              `json:"check_path_error,omitempty"`
}

// linearInputsToStrings converts LinearInputs to a map of string slices so that float64 values
// are transmitted to the frontend without precision loss.
func linearInputsToStrings(li *referenceframe.LinearInputs) map[string][]string {
	out := make(map[string][]string)
	for frameName, inputs := range li.Items() {
		if len(inputs) == 0 {
			continue
		}
		strs := make([]string, len(inputs))
		for idx, v := range inputs {
			strs[idx] = strconv.FormatFloat(v, 'g', -1, 64)
		}
		out[frameName] = strs
	}
	return out
}

// stringsToLinearInputs parses string-valued inputs (as sent from the frontend) back to LinearInputs.
func stringsToLinearInputs(data map[string][]string) (*referenceframe.LinearInputs, error) {
	li := referenceframe.NewLinearInputs()
	for frameName, strs := range data {
		floats := make([]float64, len(strs))
		for idx, s := range strs {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("frame %s index %d: %w", frameName, idx, err)
			}
			floats[idx] = v
		}
		li.Put(frameName, floats)
	}
	return li, nil
}

// ---- helpers ----

func findPlanFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".json" {
			rel, err := filepath.Rel(rdkRoot, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func buildFrameInfo(fs *referenceframe.FrameSystem) []frameInfo {
	var frames []frameInfo
	for _, name := range fs.FrameNames() {
		frame := fs.Frame(name)
		parentName := ""
		if parent, err := fs.Parent(frame); err == nil && parent != nil {
			parentName = parent.Name()
		}
		frames = append(frames, frameInfo{
			Name:   name,
			DoF:    len(frame.DoF()),
			Parent: parentName,
		})
	}
	sort.Slice(frames, func(idx, jdx int) bool {
		if frames[idx].DoF != frames[jdx].DoF {
			return frames[idx].DoF > frames[jdx].DoF
		}
		return frames[idx].Name < frames[jdx].Name
	})
	return frames
}

func buildStartInputs(cfg referenceframe.FrameSystemInputs) []frameInputs {
	var rows []frameInputs
	for name, inputs := range cfg {
		if len(inputs) == 0 {
			continue
		}
		parts := make([]string, len(inputs))
		for i, v := range inputs {
			parts[i] = strconv.FormatFloat(v, 'g', -1, 64)
		}
		rows = append(rows, frameInputs{Name: name, Inputs: "[" + strings.Join(parts, ", ") + "]"})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// poseComponents holds the 7 scalar components of a pose as float64 strings.
// Using strings preserves full float64 precision across the Go→JSON→JS→JSON→Go round-trip,
// and prevents JavaScript from silently converting component values to numbers.
type poseComponents struct {
	X string `json:"x"`
	Y string `json:"y"`
	Z string `json:"z"`
	W string `json:"w"`
	I string `json:"i"`
	J string `json:"j"`
	K string `json:"k"`
}

func poseToComponents(pose spatialmath.Pose) poseComponents {
	pt := pose.Point()
	q := pose.Orientation().Quaternion()
	return poseComponents{
		X: strconv.FormatFloat(pt.X, 'g', -1, 64),
		Y: strconv.FormatFloat(pt.Y, 'g', -1, 64),
		Z: strconv.FormatFloat(pt.Z, 'g', -1, 64),
		W: strconv.FormatFloat(q.Real, 'g', -1, 64),
		I: strconv.FormatFloat(q.Imag, 'g', -1, 64),
		J: strconv.FormatFloat(q.Jmag, 'g', -1, 64),
		K: strconv.FormatFloat(q.Kmag, 'g', -1, 64),
	}
}

func componentsToSpatialPose(pc poseComponents) (spatialmath.Pose, error) {
	parse := func(label, s string) (float64, error) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing %s: %w", label, err)
		}
		return v, nil
	}
	x, err := parse("x", pc.X)
	if err != nil {
		return nil, err
	}
	y, err := parse("y", pc.Y)
	if err != nil {
		return nil, err
	}
	z, err := parse("z", pc.Z)
	if err != nil {
		return nil, err
	}
	w, err := parse("w", pc.W)
	if err != nil {
		return nil, err
	}
	qi, err := parse("i", pc.I)
	if err != nil {
		return nil, err
	}
	qj, err := parse("j", pc.J)
	if err != nil {
		return nil, err
	}
	qk, err := parse("k", pc.K)
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPose(r3.Vector{X: x, Y: y, Z: z}, &spatialmath.Quaternion{Real: w, Imag: qi, Jmag: qj, Kmag: qk}), nil
}

// computeGoalPoseMap returns the poses for one goal, keyed by frame name, transformed into the
// world frame and encoded as poseComponents (string-valued scalars).
func computeGoalPoseMap(req *armplanning.PlanRequest, goalIdx int) (map[string]poseComponents, error) {
	if goalIdx < 0 || goalIdx >= len(req.Goals) {
		return nil, fmt.Errorf("goal index %d out of range (have %d goals)", goalIdx, len(req.Goals))
	}
	poses, err := req.Goals[goalIdx].ComputePoses(context.Background(), req.FrameSystem)
	if err != nil {
		return nil, err
	}
	result := make(map[string]poseComponents, len(poses))
	for frameName, poseValue := range poses {
		poseInWorldFrame := poseValue.Transform(
			referenceframe.NewPoseInFrame(
				req.FrameSystem.World().Name(),
				spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
		result[frameName] = poseToComponents(poseInWorldFrame.Pose())
	}
	return result, nil
}

// poseMapToDisplays converts a frame→poseComponents map into a sorted slice for template rendering.
func poseMapToDisplays(poseMap map[string]poseComponents) []poseDisplay {
	displays := make([]poseDisplay, 0, len(poseMap))
	for frame, pc := range poseMap {
		displays = append(displays, poseDisplay{
			Frame: frame,
			Pose:  fmt.Sprintf("pos=[%s, %s, %s] quat=[w=%s, i=%s, j=%s, k=%s]", pc.X, pc.Y, pc.Z, pc.W, pc.I, pc.J, pc.K),
		})
	}
	sort.Slice(displays, func(i, j int) bool { return displays[i].Frame < displays[j].Frame })
	return displays
}

// frameSystemPosesToMap encodes a FrameSystemPoses (already world-frame) as a frame→poseComponents map.
func frameSystemPosesToMap(poses referenceframe.FrameSystemPoses) map[string]poseComponents {
	result := make(map[string]poseComponents, len(poses))
	for frameName, pif := range poses {
		result[frameName] = poseToComponents(pif.Pose())
	}
	return result
}

// buildIKInspectURL constructs the /ik-inspect URL with start config and goal poses encoded as
// JSON query params. All float64 values are represented as strings so they survive the
// Go→JSON→JS→JSON→Go round-trip without any numeric conversion.
func buildIKInspectURL(file string, startConfig *referenceframe.LinearInputs, goalPoseMap map[string]poseComponents) string {
	startJSON, _ := json.Marshal(linearInputsToStrings(startConfig))
	goalJSON, _ := json.Marshal(goalPoseMap)
	return "/ik-inspect?file=" + url.QueryEscape(file) +
		"&start_config=" + url.QueryEscape(string(startJSON)) +
		"&goal_poses=" + url.QueryEscape(string(goalJSON))
}

func drawGoalPoses(req *armplanning.PlanRequest) error {
	var goalPoses []spatialmath.Pose
	for _, goalPlanState := range req.Goals {
		poses, err := goalPlanState.ComputePoses(context.Background(), req.FrameSystem)
		if err != nil {
			return err
		}
		for _, poseValue := range poses {
			poseInWorldFrame := poseValue.Transform(
				referenceframe.NewPoseInFrame(
					req.FrameSystem.World().Name(),
					spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
			goalPoses = append(goalPoses, poseInWorldFrame.Pose())
		}
	}
	return viz.DrawPoses(goalPoses, []string{"blue"}, true)
}

func renderState(relPath string) error {
	req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, relPath))
	if err != nil {
		return fmt.Errorf("reading plan file: %w", err)
	}
	startInputs := req.StartState.Configuration()
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		return fmt.Errorf("clearing visualizer: %w", err)
	}
	if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, startInputs); err != nil {
		return fmt.Errorf("drawing world state: %w", err)
	}
	if err := viz.DrawFrameSystem(req.FrameSystem, startInputs); err != nil {
		return fmt.Errorf("drawing frame system: %w", err)
	}
	if err := drawGoalPoses(req); err != nil {
		return fmt.Errorf("drawing goal poses: %w", err)
	}
	return nil
}

// visualizeLinearTrajectory renders a sequence of LinearInputs steps in the visualizer.
// It bails out early (without error) if ctx is cancelled by a newer render.
func visualizeLinearTrajectory(ctx context.Context, req *armplanning.PlanRequest, steps []*referenceframe.LinearInputs) error {
	startInputs := req.StartState.Configuration()
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		return err
	}
	if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, startInputs); err != nil {
		return err
	}
	if err := viz.DrawFrameSystem(req.FrameSystem, startInputs); err != nil {
		return err
	}
	if err := drawGoalPoses(req); err != nil {
		return err
	}
	for idx, step := range steps {
		if ctx.Err() != nil {
			return nil
		}
		if idx > 0 {
			midPoints, err := motionplan.InterpolateSegmentFS(
				&motionplan.SegmentFS{
					StartConfiguration: steps[idx-1],
					EndConfiguration:   step,
					FS:                 req.FrameSystem,
				}, 2)
			if err != nil {
				return err
			}
			for _, mp := range midPoints {
				if ctx.Err() != nil {
					return nil
				}
				if err := viz.DrawFrameSystem(req.FrameSystem, mp.ToFrameSystemInputs()); err != nil {
					return err
				}
				time.Sleep(renderFramePeriod)
			}
		}
		if err := viz.DrawFrameSystem(req.FrameSystem, step.ToFrameSystemInputs()); err != nil {
			return err
		}
		time.Sleep(renderFramePeriod)
	}
	return nil
}

func planTrajectoryToStrings(plan motionplan.Plan) []map[string][]string {
	traj := plan.Trajectory()
	result := make([]map[string][]string, len(traj))
	for idx := range plan.Path() {
		result[idx] = linearInputsToStrings(traj[idx].ToLinearInputs())
	}
	return result
}

// ---- handlers ----

func handleIndex(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := findPlanFiles(planFilesRoot)
		if err != nil {
			http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTmpl.Execute(w, files); err != nil {
			logger.Errorf("rendering index: %v", err)
		}
	}
}

func handleDetail(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			http.Error(w, fmt.Sprintf("reading plan file: %v", err), http.StatusInternalServerError)
			return
		}
		startConfig := buildStartInputs(req.StartState.Configuration())
		startLI := req.StartState.Configuration().ToLinearInputs()
		goals := make([]goalDetail, len(req.Goals))
		for idx := range req.Goals {
			poseMap, err := computeGoalPoseMap(req, idx)
			if err != nil {
				logger.Warnf("computing goal poses for goal %d: %v", idx, err)
			}
			goals[idx] = goalDetail{
				Index:        idx,
				StartConfig:  startConfig,
				GoalPoses:    poseMapToDisplays(poseMap),
				IKInspectURL: buildIKInspectURL(file, startLI, poseMap),
			}
		}
		data := detailData{
			File:        file,
			Frames:      buildFrameInfo(req.FrameSystem),
			StartInputs: startConfig,
			Goals:       goals,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := detailTmpl.Execute(w, data); err != nil {
			logger.Errorf("rendering detail: %v", err)
		}
	}
}

func handleIKInspect(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		var startConfigStrings map[string][]string
		if sc := r.URL.Query().Get("start_config"); sc != "" {
			if err := json.Unmarshal([]byte(sc), &startConfigStrings); err != nil {
				http.Error(w, fmt.Sprintf("parsing start_config: %v", err), http.StatusBadRequest)
				return
			}
		}
		var goalPoseMap map[string]poseComponents
		if gp := r.URL.Query().Get("goal_poses"); gp != "" {
			if err := json.Unmarshal([]byte(gp), &goalPoseMap); err != nil {
				http.Error(w, fmt.Sprintf("parsing goal_poses: %v", err), http.StatusBadRequest)
				return
			}
		}
		startConfig := make([]frameInputs, 0, len(startConfigStrings))
		for frameName, vals := range startConfigStrings {
			startConfig = append(startConfig, frameInputs{
				Name:   frameName,
				Inputs: "[" + strings.Join(vals, ", ") + "]",
			})
		}
		sort.Slice(startConfig, func(i, j int) bool { return startConfig[i].Name < startConfig[j].Name })
		startConfigJSONBytes, _ := json.Marshal(startConfigStrings)
		goalPosesJSONBytes, _ := json.Marshal(goalPoseMap)
		data := ikInspectData{
			File:            file,
			StartConfig:     startConfig,
			StartConfigJSON: template.JS(startConfigJSONBytes), //nolint:gosec
			GoalPoses:       poseMapToDisplays(goalPoseMap),
			GoalPosesJSON:   template.JS(goalPosesJSONBytes), //nolint:gosec
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ikInspectTmpl.Execute(w, data); err != nil {
			logger.Errorf("rendering ik-inspect: %v", err)
		}
	}
}

func handleIKInspectRun(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			writeJSON(w, ikInspectRunResult{Error: "missing file parameter"})
			return
		}
		var startConfigStrings map[string][]string
		if sc := r.URL.Query().Get("start_config"); sc != "" {
			if err := json.Unmarshal([]byte(sc), &startConfigStrings); err != nil {
				writeJSON(w, ikInspectRunResult{Error: fmt.Sprintf("parsing start_config: %v", err)})
				return
			}
		}
		if len(startConfigStrings) == 0 {
			writeJSON(w, ikInspectRunResult{Error: "start_config is required"})
			return
		}
		segmentStart, err := stringsToLinearInputs(startConfigStrings)
		if err != nil {
			writeJSON(w, ikInspectRunResult{Error: fmt.Sprintf("parsing start_config values: %v", err)})
			return
		}
		var goalPoseMap map[string]poseComponents
		if gp := r.URL.Query().Get("goal_poses"); gp != "" {
			if err := json.Unmarshal([]byte(gp), &goalPoseMap); err != nil {
				writeJSON(w, ikInspectRunResult{Error: fmt.Sprintf("parsing goal_poses: %v", err)})
				return
			}
		}
		if len(goalPoseMap) == 0 {
			writeJSON(w, ikInspectRunResult{Error: "goal_poses is required"})
			return
		}
		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			writeJSON(w, ikInspectRunResult{Error: err.Error()})
			return
		}
		worldFrameName := req.FrameSystem.World().Name()
		goalPoses := make(referenceframe.FrameSystemPoses, len(goalPoseMap))
		for frameName, pc := range goalPoseMap {
			pose, err := componentsToSpatialPose(pc)
			if err != nil {
				writeJSON(w, ikInspectRunResult{Error: fmt.Sprintf("parsing goal pose for %q: %v", frameName, err)})
				return
			}
			goalPoses[frameName] = referenceframe.NewPoseInFrame(worldFrameName, pose)
		}

		armplanning.ClearSeedCache()
		result, err := InspectIK(r.Context(), logger.Sublogger("ik-inspect"), req, segmentStart.ToFrameSystemInputs(), goalPoses, 10)
		if err != nil {
			writeJSON(w, ikInspectRunResult{Error: err.Error()})
			return
		}

		out := ikInspectRunResult{Threads: make([][]ikInspectCellResult, len(result.Rows))}
		for threadIdx, cells := range result.Rows {
			rows := make([]ikInspectCellResult, len(cells))
			for cellIdx, cell := range cells {
				row := ikInspectCellResult{
					Cost:        cell.Cost,
					Exact:       cell.Exact,
					Valid:       cell.Valid,
					CheckPathOK: cell.CheckPathOK,
				}
				if cell.Inputs != nil {
					row.Inputs = linearInputsToStrings(cell.Inputs)
				}
				if cell.StateError != nil {
					row.StateError = cell.StateError.Error()
				}
				if cell.CheckPathError != nil {
					row.CheckPathError = cell.CheckPathError.Error()
				}
				rows[cellIdx] = row
			}
			out.Threads[threadIdx] = rows
		}
		writeJSON(w, out)
	}
}

func handlePlanRun(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}

		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			writeJSON(w, planRunResult{Error: err.Error()})
			return
		}

		armplanning.ClearSeedCache()

		if req.PlannerOptions == nil {
			req.PlannerOptions = armplanning.NewBasicPlannerOptions()
		}
		req.PlannerOptions.CollectSolutionDiagnostics = true
		if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
			if secs, err := strconv.ParseFloat(timeoutStr, 64); err == nil && secs > 0 {
				req.PlannerOptions.Timeout = secs
			}
		}
		if seedStr := r.URL.Query().Get("seed"); seedStr != "" {
			if seed, err := strconv.Atoi(seedStr); err == nil {
				req.PlannerOptions.RandomSeed = seed
			}
		}

		plan, meta, err := armplanning.PlanMotion(r.Context(), logger, req)
		if err != nil {
			writeJSON(w, planRunResult{Error: err.Error()})
			return
		}

		result := planRunResult{
			Steps:          len(plan.Path()),
			Duration:       meta.Duration.String(),
			GoalsProcessed: meta.GoalsProcessed,
			Partial:        meta.Partial,
		}
		if meta.PartialError != nil {
			result.PartialError = meta.PartialError.Error()
		}
		for _, pg := range meta.PerGoal {
			poseMap := frameSystemPosesToMap(pg.GoalPoses)
			pgResult := perGoalResult{
				IKInspectURL:             buildIKInspectURL(file, pg.StartConfiguration, poseMap),
				ConstraintFailuresByType: pg.ConstraintFailuresByType,
			}
			for _, sn := range pg.SolutionNodes {
				row := solutionNodeResult{
					Score:  sn.Score,
					Inputs: linearInputsToStrings(sn.Inputs),
				}
				if sn.CheckPathError != nil {
					row.CheckPathError = sn.CheckPathError.Error()
					if sn.LastGoodInputs != nil {
						row.LastGoodInputs = linearInputsToStrings(sn.LastGoodInputs)
					}
					pgResult.CheckPathFailures = append(pgResult.CheckPathFailures, row)
				} else {
					pgResult.ValidSolutions = append(pgResult.ValidSolutions, row)
				}
			}
			result.PerGoal = append(result.PerGoal, pgResult)
		}
		result.Trajectory = planTrajectoryToStrings(plan)

		writeJSON(w, result)
	}
}

func handleRenderSolution(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			http.Error(w, fmt.Sprintf("reading plan file: %v", err), http.StatusInternalServerError)
			return
		}
		var inputStrings map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&inputStrings); err != nil {
			http.Error(w, fmt.Sprintf("decoding inputs: %v", err), http.StatusBadRequest)
			return
		}
		li, err := stringsToLinearInputs(inputStrings)
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing inputs: %v", err), http.StatusBadRequest)
			return
		}
		beginRender()
		startInputs := req.StartState.Configuration()
		if err := viz.RemoveAllSpatialObjects(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, startInputs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := viz.DrawFrameSystem(req.FrameSystem, li.ToFrameSystemInputs()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := drawGoalPoses(req); err != nil {
			logger.Warnf("drawing goal poses: %v", err)
		}
	}
}

func handleRenderShadows(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			http.Error(w, fmt.Sprintf("reading plan file: %v", err), http.StatusInternalServerError)
			return
		}
		var inputStrings map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&inputStrings); err != nil {
			http.Error(w, fmt.Sprintf("decoding inputs: %v", err), http.StatusBadRequest)
			return
		}
		end, err := stringsToLinearInputs(inputStrings)
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing inputs: %v", err), http.StatusBadRequest)
			return
		}
		ctx := beginRender()
		startInputs := req.StartState.Configuration()
		start := startInputs.ToLinearInputs()

		if err := viz.RemoveAllSpatialObjects(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := viz.DrawWorldState(req.WorldState, req.FrameSystem, startInputs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := viz.DrawFrameSystem(req.FrameSystem, startInputs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := drawGoalPoses(req); err != nil {
			logger.Warnf("drawing goal poses: %v", err)
		}

		midPoints, err := interpolateShadows(req.FrameSystem, start, end, shadowCount)
		if err != nil {
			http.Error(w, fmt.Sprintf("interpolating: %v", err), http.StatusInternalServerError)
			return
		}
		if err := drawShadows(ctx, req.FrameSystem, midPoints); err != nil {
			http.Error(w, fmt.Sprintf("drawing shadows: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// interpolateShadows produces `count`+1 evenly spaced configurations from start to end (inclusive
// on both ends). We hand-roll this instead of using InterpolateSegmentFS because that helper picks
// step counts based on cartesian and per-joint deltas, which yields hundreds-to-thousands of steps
// — fine for collision checking, too many for shadow rendering.
func interpolateShadows(
	fs *referenceframe.FrameSystem, start, end *referenceframe.LinearInputs, count int,
) ([]*referenceframe.LinearInputs, error) {
	out := make([]*referenceframe.LinearInputs, 0, count+1)
	for step := 0; step <= count; step++ {
		t := float64(step) / float64(count)
		cfg := referenceframe.NewLinearInputs()
		for frameName, startConfig := range start.Items() {
			endConfig := end.Get(frameName)
			frame := fs.Frame(frameName)
			interp, err := frame.Interpolate(startConfig, endConfig, t)
			if err != nil {
				return nil, err
			}
			cfg.Put(frameName, interp)
		}
		out = append(out, cfg)
	}
	return out, nil
}

// drawShadows draws each interpolated configuration as a static "shadow" so the user can see the
// full straight-line path at once. Only frames with DoF (or descendants of moving frames) get
// shadows. Colors alternate per step to make ordering visible.
func drawShadows(ctx context.Context, fs *referenceframe.FrameSystem, configs []*referenceframe.LinearInputs) error {
	isMovingFrame := func(frameName string) bool {
		frame := fs.Frame(frameName)
		if frame == nil {
			return false
		}
		if len(frame.DoF()) > 0 {
			return true
		}
		parent, err := fs.Parent(frame)
		for parent != nil && err == nil {
			if len(parent.DoF()) > 0 {
				return true
			}
			parent, err = fs.Parent(parent)
		}
		return false
	}

	shadowColors := []string{"blue", "red"}
	for idx, cfg := range configs {
		if ctx.Err() != nil {
			return nil
		}
		gifs, err := referenceframe.FrameSystemGeometries(fs, cfg.ToFrameSystemInputs())
		if err != nil {
			return err
		}
		shadowColor := shadowColors[idx%len(shadowColors)]
		for frameName, gif := range gifs {
			if !isMovingFrame(frameName) {
				continue
			}
			shadowGeometries := make([]spatialmath.Geometry, len(gif.Geometries()))
			for i, geom := range gif.Geometries() {
				shadowGeom := geom.Transform(spatialmath.NewZeroPose())
				shadowGeom.SetLabel(fmt.Sprintf("shadow_%d_%s_%d", idx, geom.Label(), i))
				shadowGeometries[i] = shadowGeom
			}
			shadowGIF := referenceframe.NewGeometriesInFrame(gif.Parent(), shadowGeometries)
			colors := make([]string, len(shadowGeometries))
			for i := range colors {
				colors[i] = shadowColor
			}
			if err := viz.DrawGeometries(shadowGIF, colors); err != nil {
				return err
			}
		}
		time.Sleep(shadowFramePeriod)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleRenderPlan(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, file))
		if err != nil {
			http.Error(w, fmt.Sprintf("reading plan file: %v", err), http.StatusInternalServerError)
			return
		}
		var trajStrings []map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&trajStrings); err != nil {
			http.Error(w, fmt.Sprintf("decoding trajectory: %v", err), http.StatusBadRequest)
			return
		}
		steps := make([]*referenceframe.LinearInputs, len(trajStrings))
		for idx, step := range trajStrings {
			li, err := stringsToLinearInputs(step)
			if err != nil {
				http.Error(w, fmt.Sprintf("parsing step %d: %v", idx, err), http.StatusBadRequest)
				return
			}
			steps[idx] = li
		}
		ctx := beginRender()
		if err := visualizeLinearTrajectory(ctx, req, steps); err != nil {
			logger.Warnf("visualization failed (motion-tools server may not be running): %v", err)
		}
	}
}

func handleRenderStart(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
		beginRender()
		if err := renderState(file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//nolint
		fmt.Fprintf(w, "Rendered start state for %s", file)
	}
}

// RunServer runs server.
func RunServer() error {
	logger := logging.NewLogger("mp-server")

	http.HandleFunc("/", handleIndex(logger))
	http.HandleFunc("/detail", handleDetail(logger))
	http.HandleFunc("/ik-inspect", handleIKInspect(logger))
	http.HandleFunc("/ik-inspect/run", handleIKInspectRun(logger))
	http.HandleFunc("/plan/run", handlePlanRun(logger))
	http.HandleFunc("/render-plan", handleRenderPlan(logger))
	http.HandleFunc("/render-start", handleRenderStart(logger))
	http.HandleFunc("/render-solution", handleRenderSolution(logger))
	http.HandleFunc("/render-shadows", handleRenderShadows(logger))

	addr := "localhost:8080"
	logger.Infof("listening on http://%s", addr)
	//nolint
	return http.ListenAndServe(addr, nil)
}
