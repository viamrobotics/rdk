// Package mpserver is a webserver for diagnosing motion plans.
//

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
	vizapi "github.com/viam-labs/motion-tools/client/api"
	"github.com/viam-labs/motion-tools/draw"

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
  .warning-box {
    background-color: #fff3cd;
    border: 1px solid #b8860b;
    padding: 10px 14px;
    margin: 16px 0;
  }
  .warning-box ul { margin: 4px 0 0 0; padding-left: 20px; }
  .pose-editor { margin: 8px 0; padding: 8px; background-color: #D0EEFF; border: 1px solid black; }
  .pose-editor input { min-width: 6ch; padding: 2px 4px; }
  .pc-cloud { margin-top: 6px; }
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

<h2>User Goals</h2>
<script>const GOAL_POSES_DATA = {};</script>
{{if .Constraints}}
<div class="warning-box">
  <strong>Constrained motion</strong> — motion planning will subdivide these user goals into additional sub-goals to satisfy the following constraints:
  {{if .Constraints.Linear}}
  <p><strong>Linear:</strong></p><ul>{{range .Constraints.Linear}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
  {{if .Constraints.Orientation}}
  <p><strong>Orientation:</strong></p><ul>{{range .Constraints.Orientation}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
</div>
{{end}}
{{if .Goals}}
{{range .Goals}}
<h3>User Goal {{.Index}} — <a href="{{.IKInspectURL}}">Inspect IK</a></h3>
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
<div id="goal-poses-{{.Index}}" class="pose-editors"></div>
<script>GOAL_POSES_DATA[{{.Index}}] = {{.GoalPosesJSON}};</script>
{{else}}
<p><em>No goal poses.</em></p>
{{end}}
{{end}}
<p>
  <button onclick="downloadPlanRequest()">Download Plan Request</button>
</p>
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
const OVERRIDES_PARAM = "{{.OverridesParam}}";

function renderState() {
  fetch('/render-start?file=' + encodeURIComponent('{{.File}}') +
        '&overrides=' + encodeURIComponent(OVERRIDES_PARAM))
    .then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
}

renderState();

// ---- goal pose editing ----

// poseInputsHTML/cloudInputsHTML/buildPoseEditor/readPoseEditors build and read the editable
// pose (+ optional GoalCloud leeway) forms rendered into each #goal-poses-N div. poseComponents
// values are string-valued floats (see poseComponents in server.go) so editing never loses
// precision by round-tripping through a JS number.
// inputSize picks a starting character width wide enough for val (e.g. a near-zero value in
// scientific notation like "1.2246467991473515e-16"), with a floor so short values still get a
// usable box.
function inputSize(val) {
  return Math.max(6, String(val).length);
}

// autoGrowAttrs sets the input's initial size attribute from its value and grows it as the user
// types, so long values (long decimals, scientific notation) are never clipped.
function autoGrowAttrs(val) {
  return 'size="' + inputSize(val) + '" oninput="this.size = Math.max(6, this.value.length)"';
}

function poseInputsHTML(pc) {
  const field = (cls, label, val) =>
    label + ': <input class="' + cls + ' pose-field" type="number" step="any" value="' + escHtml(val) + '" ' + autoGrowAttrs(val) + '>';
  return [field('pc-x', 'x', pc.x), field('pc-y', 'y', pc.y), field('pc-z', 'z', pc.z),
          field('pc-theta', 'θ', pc.theta), field('pc-ox', 'ox', pc.ox), field('pc-oy', 'oy', pc.oy), field('pc-oz', 'oz', pc.oz)]
    .join(' ');
}

function cloudInputsHTML(cloud) {
  const field = (cls, label, val) =>
    label + ': <input class="' + cls + '" type="number" step="any" value="' + escHtml(val) + '" ' + autoGrowAttrs(val) + '>';
  return [field('pcc-x', 'x±', cloud.x), field('pcc-y', 'y±', cloud.y), field('pcc-z', 'z±', cloud.z),
          field('pcc-ox', 'ox±', cloud.ox), field('pcc-oy', 'oy±', cloud.oy), field('pcc-oz', 'oz±', cloud.oz),
          field('pcc-theta', 'θ±', cloud.theta)]
    .join(' ');
}

// buildPoseEditor renders each frame's pose fields alongside its pose cloud (goal leeway) fields
// — always visible, never hidden — so an existing GoalCloud's values are visible up front rather
// than hidden behind a toggle. The checkbox only controls whether the cloud is kept (checked) or
// dropped (unchecked) when the form is read back; it defaults to checked iff the goal already had
// a cloud.
function buildPoseEditor(poseMap) {
  const zeroCloud = {x: '0', y: '0', z: '0', ox: '0', oy: '0', oz: '0', theta: '0'};
  return Object.keys(poseMap).sort().map(frame => {
    const pc = poseMap[frame];
    const hasCloud = !!pc.cloud;
    return '<div class="pose-editor" data-frame="' + escHtml(frame) + '">' +
      '<strong>' + escHtml(frame) + '</strong><br>' + poseInputsHTML(pc) +
      '<div class="pc-cloud">' +
      '<label><input type="checkbox" class="pc-cloud-toggle"' + (hasCloud ? ' checked' : '') +
      '> Pose Cloud</label> ' + cloudInputsHTML(pc.cloud || zeroCloud) +
      '</div></div>';
  }).join('');
}

function readPoseEditors(containerId) {
  const container = document.getElementById(containerId);
  const result = {};
  if (!container) return result;
  container.querySelectorAll('.pose-editor').forEach(el => {
    const get = cls => el.querySelector('.' + cls).value;
    const pc = {x: get('pc-x'), y: get('pc-y'), z: get('pc-z'), theta: get('pc-theta'), ox: get('pc-ox'), oy: get('pc-oy'), oz: get('pc-oz')};
    if (el.querySelector('.pc-cloud-toggle').checked) {
      pc.cloud = {
        x: get('pcc-x'), y: get('pcc-y'), z: get('pcc-z'),
        ox: get('pcc-ox'), oy: get('pcc-oy'), oz: get('pcc-oz'), theta: get('pcc-theta'),
      };
    }
    result[el.getAttribute('data-frame')] = pc;
  });
  return result;
}

// ---- live candidate pose preview ----
//
// As pose fields are edited, debounce and POST the in-progress pose to the visualizer in a
// distinct color (candidatePoseColor server-side), so there's live feedback on where the edited
// pose lands. Each call fully replaces whatever candidate was drawn by the previous call.

let candidateRenderTimer = null;
let candidateRenderAbort = null;

function scheduleCandidateRender(containerId) {
  if (candidateRenderTimer) clearTimeout(candidateRenderTimer);
  candidateRenderTimer = setTimeout(() => sendCandidateRender(containerId), 300);
}

function sendCandidateRender(containerId) {
  if (candidateRenderAbort) candidateRenderAbort.abort();
  candidateRenderAbort = new AbortController();
  fetch('/render-candidate-pose?file=' + encodeURIComponent('{{.File}}'), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(readPoseEditors(containerId)),
    signal: candidateRenderAbort.signal,
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Candidate render error: ' + msg)); })
    .catch(err => { if (err.name !== 'AbortError') console.error('Candidate render error: ' + err); });
}

function wireCandidatePreview(containerId) {
  const div = document.getElementById(containerId);
  if (!div) return;
  div.addEventListener('input', e => {
    if (e.target.classList.contains('pose-field')) scheduleCandidateRender(containerId);
  });
}

function renderGoalPoseForms() {
  Object.keys(GOAL_POSES_DATA).forEach(goalIndex => {
    const containerId = 'goal-poses-' + goalIndex;
    const div = document.getElementById(containerId);
    if (div) div.innerHTML = buildPoseEditor(GOAL_POSES_DATA[goalIndex]);
    wireCandidatePreview(containerId);
  });
}
renderGoalPoseForms();

// currentOverrides reads every goal's pose editor form and builds a requestOverrides object
// (array is index-aligned with the plan request's Goals, matching the Go-side requestOverrides).
function currentOverrides() {
  const goals = [];
  Object.keys(GOAL_POSES_DATA).map(Number).sort((a, b) => a - b).forEach(goalIndex => {
    goals[goalIndex] = readPoseEditors('goal-poses-' + goalIndex);
  });
  return {goals: goals};
}

function downloadPlanRequest() {
  window.location.href = '/plan/download?file=' + encodeURIComponent('{{.File}}') +
    '&overrides=' + encodeURIComponent(JSON.stringify(currentOverrides()));
}

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
        '&seed=' + encodeURIComponent(seed) +
        '&overrides=' + encodeURIComponent(JSON.stringify(currentOverrides())),
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
  .pose-editor { margin: 8px 0; padding: 8px; background-color: #D0EEFF; border: 1px solid black; }
  .pose-editor input { min-width: 6ch; padding: 2px 4px; }
  .pc-cloud { margin-top: 6px; }
</style>
</head>
<body>
<h1>IK Inspect</h1>
<p>{{.File}} — <button onclick="window.location.href = backToDetailURL()">← Back to Detail</button>
&nbsp;<button onclick="window.location.href = downloadURL()">Download Plan Request</button></p>

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
<div id="goal-poses-editor"></div>
{{else}}
<p><em>No goal poses.</em></p>
{{end}}

<button onclick="renderStartAndGoals()">Render Start + Goals</button>
&nbsp;<button onclick="runIKInspect()">Run IK Inspection</button>

<div id="rendered-config" style="display:none; margin-top:16px;">
  <h2>Rendered Configuration</h2>
  <table>
    <tr><th>Frame</th><th>Inputs</th></tr>
  </table>
</div>

<div id="ik-result" style="margin-top:16px;"></div>

<script>
const START_CONFIG = {{.StartConfigJSON}};
const GOAL_POSES = {{.GoalPosesJSON}};
const GOAL_INDEX = {{.GoalIndex}};
const OVERRIDES_PARAM = "{{.OverridesParam}}";

// ---- pose (+ optional GoalCloud leeway) editing, mirrors the detail page's editors ----

// inputSize picks a starting character width wide enough for val (e.g. a near-zero value in
// scientific notation like "1.2246467991473515e-16"), with a floor so short values still get a
// usable box.
function inputSize(val) {
  return Math.max(6, String(val).length);
}

// autoGrowAttrs sets the input's initial size attribute from its value and grows it as the user
// types, so long values (long decimals, scientific notation) are never clipped.
function autoGrowAttrs(val) {
  return 'size="' + inputSize(val) + '" oninput="this.size = Math.max(6, this.value.length)"';
}

function poseInputsHTML(pc) {
  const field = (cls, label, val) =>
    label + ': <input class="' + cls + ' pose-field" type="number" step="any" value="' + escHtml(val) + '" ' + autoGrowAttrs(val) + '>';
  return [field('pc-x', 'x', pc.x), field('pc-y', 'y', pc.y), field('pc-z', 'z', pc.z),
          field('pc-theta', 'θ', pc.theta), field('pc-ox', 'ox', pc.ox), field('pc-oy', 'oy', pc.oy), field('pc-oz', 'oz', pc.oz)]
    .join(' ');
}

function cloudInputsHTML(cloud) {
  const field = (cls, label, val) =>
    label + ': <input class="' + cls + '" type="number" step="any" value="' + escHtml(val) + '" ' + autoGrowAttrs(val) + '>';
  return [field('pcc-x', 'x±', cloud.x), field('pcc-y', 'y±', cloud.y), field('pcc-z', 'z±', cloud.z),
          field('pcc-ox', 'ox±', cloud.ox), field('pcc-oy', 'oy±', cloud.oy), field('pcc-oz', 'oz±', cloud.oz),
          field('pcc-theta', 'θ±', cloud.theta)]
    .join(' ');
}

// buildPoseEditor renders each frame's pose fields alongside its pose cloud (goal leeway) fields
// — always visible, never hidden — so an existing GoalCloud's values are visible up front rather
// than hidden behind a toggle. The checkbox only controls whether the cloud is kept (checked) or
// dropped (unchecked) when the form is read back; it defaults to checked iff the goal already had
// a cloud.
function buildPoseEditor(poseMap) {
  const zeroCloud = {x: '0', y: '0', z: '0', ox: '0', oy: '0', oz: '0', theta: '0'};
  return Object.keys(poseMap).sort().map(frame => {
    const pc = poseMap[frame];
    const hasCloud = !!pc.cloud;
    return '<div class="pose-editor" data-frame="' + escHtml(frame) + '">' +
      '<strong>' + escHtml(frame) + '</strong><br>' + poseInputsHTML(pc) +
      '<div class="pc-cloud">' +
      '<label><input type="checkbox" class="pc-cloud-toggle"' + (hasCloud ? ' checked' : '') +
      '> Pose Cloud</label> ' + cloudInputsHTML(pc.cloud || zeroCloud) +
      '</div></div>';
  }).join('');
}

function readPoseEditors(containerId) {
  const container = document.getElementById(containerId);
  const result = {};
  if (!container) return result;
  container.querySelectorAll('.pose-editor').forEach(el => {
    const get = cls => el.querySelector('.' + cls).value;
    const pc = {x: get('pc-x'), y: get('pc-y'), z: get('pc-z'), theta: get('pc-theta'), ox: get('pc-ox'), oy: get('pc-oy'), oz: get('pc-oz')};
    if (el.querySelector('.pc-cloud-toggle').checked) {
      pc.cloud = {
        x: get('pcc-x'), y: get('pcc-y'), z: get('pcc-z'),
        ox: get('pcc-ox'), oy: get('pcc-oy'), oz: get('pcc-oz'), theta: get('pcc-theta'),
      };
    }
    result[el.getAttribute('data-frame')] = pc;
  });
  return result;
}

// ---- live candidate pose preview ----
//
// As pose fields are edited, debounce and POST the in-progress pose to the visualizer in a
// distinct color (candidatePoseColor server-side), so there's live feedback on where the edited
// pose lands. Each call fully replaces whatever candidate was drawn by the previous call.

let candidateRenderTimer = null;
let candidateRenderAbort = null;

function scheduleCandidateRender(containerId) {
  if (candidateRenderTimer) clearTimeout(candidateRenderTimer);
  candidateRenderTimer = setTimeout(() => sendCandidateRender(containerId), 300);
}

function sendCandidateRender(containerId) {
  if (candidateRenderAbort) candidateRenderAbort.abort();
  candidateRenderAbort = new AbortController();
  fetch('/render-candidate-pose?file=' + encodeURIComponent('{{.File}}'), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(readPoseEditors(containerId)),
    signal: candidateRenderAbort.signal,
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Candidate render error: ' + msg)); })
    .catch(err => { if (err.name !== 'AbortError') console.error('Candidate render error: ' + err); });
}

function wireCandidatePreview(containerId) {
  const div = document.getElementById(containerId);
  if (!div) return;
  div.addEventListener('input', e => {
    if (e.target.classList.contains('pose-field')) scheduleCandidateRender(containerId);
  });
}

document.addEventListener('DOMContentLoaded', () => {
  const gpDiv = document.getElementById('goal-poses-editor');
  if (gpDiv) gpDiv.innerHTML = buildPoseEditor(GOAL_POSES);
  wireCandidatePreview('goal-poses-editor');
});

// mergedOverrides merges this page's current (possibly edited) goal pose into whatever
// requestOverrides were already in effect (carried through from the detail page via
// OVERRIDES_PARAM), at index GOAL_INDEX — so edits made here survive navigating back.
function mergedOverrides() {
  let ov = {goals: []};
  if (OVERRIDES_PARAM) {
    try {
      ov = JSON.parse(OVERRIDES_PARAM);
    } catch (e) { /* fall back to empty overrides */ }
  }
  if (!ov.goals) ov.goals = [];
  ov.goals[GOAL_INDEX] = readPoseEditors('goal-poses-editor');
  return ov;
}

function backToDetailURL() {
  return '/detail?file=' + encodeURIComponent('{{.File}}') +
    '&overrides=' + encodeURIComponent(JSON.stringify(mergedOverrides()));
}

function downloadURL() {
  return '/plan/download?file=' + encodeURIComponent('{{.File}}') +
    '&overrides=' + encodeURIComponent(JSON.stringify(mergedOverrides()));
}

function renderStartAndGoals() {
  fetch('/render-start?file=' + encodeURIComponent('{{.File}}') +
        '&overrides=' + encodeURIComponent(JSON.stringify(mergedOverrides())))
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
        '&goal_poses=' + encodeURIComponent(JSON.stringify(readPoseEditors('goal-poses-editor'))),
        { signal: ikInspectAbort.signal })
    .then(r => r.json())
    .then(data => {
      if (data.error) {
        div.innerHTML = '<pre style="color:#cc0000">Error: ' + escHtml(data.error) + '</pre>';
        return;
      }
      div.innerHTML = buildIKTable('{{.File}}', data.seeds || [], data.seed_labels || []);
    })
    .catch(err => { if (err.name !== 'AbortError') div.textContent = 'Fetch error: ' + err; });
}

// seeds is column-major: seeds[col] is the ordered list of solutions seed col emitted.
function buildIKTable(file, seeds, seedLabels) {
  const cellLegend = '<p class="legend">' +
    '<span class="cell-green" style="background:#b6e7b0">valid + checkPath ok</span>' +
    '<span class="cell-yellow" style="background:#f3e6a0">valid, checkPath failed</span>' +
    '<span class="cell-red" style="background:#f0b0b0">invalid (e.g. collision)</span>' +
    '</p>';

  let seedLegend = '';
  if (seedLabels && seedLabels.length) {
    seedLegend = '<table style="margin-bottom:12px"><tr><th>Seed</th><th>Scenario</th></tr>';
    for (let c = 0; c < seedLabels.length; c++) {
      seedLegend += '<tr><td>' + c + '</td><td>' + escHtml(seedLabels[c]) + '</td></tr>';
    }
    seedLegend += '</table>';
  }

  let maxRows = 0;
  for (const col of seeds) maxRows = Math.max(maxRows, col.length);
  if (maxRows === 0) return seedLegend + cellLegend + '<p><em>No IK solutions were emitted.</em></p>';

  let html = seedLegend + cellLegend + '<table id="ik-table"><tr><th></th>';
  for (let c = 0; c < seeds.length; c++) {
    const label = (seedLabels && seedLabels[c]) ? escHtml(seedLabels[c]) : ('seed ' + c);
    html += '<th title="' + label + '">seed ' + c + '</th>';
  }
  html += '</tr>';
  for (let row = 0; row < maxRows; row++) {
    html += '<tr><th>' + row + '</th>';
    for (let c = 0; c < seeds.length; c++) {
      const cell = seeds[c][row];
      if (!cell) { html += '<td class="cell-empty"></td>'; continue; }
      html += renderIKCell(file, cell);
    }
    html += '</tr>';
  }
  html += '</table>';
  return html;
}

function renderIKCell(file, cell) {
  let cls = 'cell-red';
  if (cell.valid && cell.check_path_ok) cls = 'cell-green';
  else if (cell.valid) cls = 'cell-yellow';

  const tip = [];
  tip.push('cost: ' + cell.cost + (cell.exact ? ' (exact)' : ''));
  if (cell.state_error) tip.push('invalid: ' + cell.state_error);
  if (cell.check_path_error) tip.push('checkPath: ' + cell.check_path_error);

  let inner = '<strong>' + cell.cost.toFixed(4) + '</strong>';
  const inlineError = cell.state_error || cell.check_path_error;
  if (inlineError) inner += '<br><small>' + escHtml(inlineError) + '</small>';
  if (cls === 'cell-yellow' && cell.last_good_inputs) {
    const violationArg = JSON.stringify(cell.last_good_inputs);
    inner += '<br><button onclick=\'renderIKSolution(' + JSON.stringify(file) + ',' + violationArg + ')\'>Render Constraint Violation</button>';
    if (cell.inputs) {
      const inputsArg = JSON.stringify(cell.inputs);
      inner += ' <button onclick=\'renderIKSolution(' + JSON.stringify(file) + ',' + inputsArg + ')\'>Render Final Position</button>';
    }
  } else if (cell.inputs) {
    const inputsArg = JSON.stringify(cell.inputs);
    inner += '<br><button onclick=\'renderIKSolution(' + JSON.stringify(file) + ',' + inputsArg + ')\'>Render</button>';
  }
  return '<td class="' + cls + '" title="' + escHtml(tip.join('\n')) + '">' + inner + '</td>';
}

// renderIKSolution draws a single IK cell's configuration in the visualizer, reusing the
// detail page's /render-solution endpoint (inputs are string-valued floats).
function renderIKSolution(file, inputs) {
  const configDiv = document.getElementById('rendered-config');
  const tbody = configDiv.querySelector('table');
  let rows = '<tr><th>Frame</th><th>Inputs</th></tr>';
  for (const [frame, vals] of Object.entries(inputs).sort(([a], [b]) => a < b ? -1 : 1)) {
    rows += '<tr><td>' + escHtml(frame) + '</td><td><code>[' + vals.join(', ') + ']</code></td></tr>';
  }
  tbody.innerHTML = rows;
  configDiv.style.display = '';

  fetch('/render-solution?file=' + encodeURIComponent(file), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(inputs),
  }).then(r => { if (!r.ok) r.text().then(msg => console.error('Render error: ' + msg)); })
    .catch(err => console.error('Render error: ' + err));
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
	Index         int
	StartConfig   []frameInputs
	GoalPoses     []poseDisplay
	GoalPosesJSON template.JS
	IKInspectURL  string
}

type detailConstraints struct {
	Linear      []string // pre-formatted per-constraint descriptions
	Orientation []string
}

type detailData struct {
	File           string
	OverridesParam string // currently-applied requestOverrides, JSON-encoded (unescaped)
	Frames         []frameInfo
	StartInputs    []frameInputs
	Goals          []goalDetail
	Constraints    *detailConstraints // nil when no linear/orientation constraints are present
}

type ikInspectData struct {
	File            string
	GoalIndex       int    // which goal (within the plan request's Goals) these poses came from
	OverridesParam  string // requestOverrides in effect for the rest of the request, JSON-encoded (unescaped)
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

// ikInspectRunResult is the JSON payload for the IK-inspect table. Seeds is column-major:
// Seeds[i] is the ordered list of solutions seed i emitted.
type ikInspectRunResult struct {
	Error      string                  `json:"error,omitempty"`
	Seeds      [][]ikInspectCellResult `json:"seeds,omitempty"`
	SeedLabels []string                `json:"seed_labels,omitempty"`
}

// ikInspectCellResult is one solution emitted from one seed. Inputs use string-valued floats to
// preserve precision, matching solutionNodeResult.
type ikInspectCellResult struct {
	Cost           float64             `json:"cost"`
	Exact          bool                `json:"exact"`
	Inputs         map[string][]string `json:"inputs,omitempty"`
	Valid          bool                `json:"valid"`
	StateError     string              `json:"state_error,omitempty"`
	CheckPathOK    bool                `json:"check_path_ok"`
	CheckPathError string              `json:"check_path_error,omitempty"`
	LastGoodInputs map[string][]string `json:"last_good_inputs,omitempty"`
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

func buildDetailConstraints(c *motionplan.Constraints) *detailConstraints {
	if c == nil || (len(c.LinearConstraint) == 0 && len(c.OrientationConstraint) == 0) {
		return nil
	}
	dc := &detailConstraints{}
	for _, lc := range c.LinearConstraint {
		dc.Linear = append(dc.Linear, fmt.Sprintf("line tolerance %.4g mm, orientation tolerance %.4g°",
			lc.LineToleranceMm, lc.OrientationToleranceDegs))
	}
	for _, oc := range c.OrientationConstraint {
		dc.Orientation = append(dc.Orientation, fmt.Sprintf("orientation tolerance %.4g°", oc.OrientationToleranceDegs))
	}
	return dc
}

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

// poseCloudComponents mirrors referenceframe.PoseCloud (the per-dimension leeway around a goal
// pose that IK/planning treats as an equivalent destination) with float64 values represented as
// strings, for the same precision-preserving reasons as poseComponents below.
type poseCloudComponents struct {
	X     string `json:"x"`
	Y     string `json:"y"`
	Z     string `json:"z"`
	OX    string `json:"ox"`
	OY    string `json:"oy"`
	OZ    string `json:"oz"`
	Theta string `json:"theta"`
}

// poseComponents holds a pose's translation and orientation (as an orientation vector — Theta,
// OX, OY, OZ — rather than a raw quaternion, since orientation vectors are what a human editing
// a pose by hand can actually reason about) as float64 strings, plus an optional pose cloud (goal
// leeway). Using strings preserves full float64 precision across the Go→JSON→JS→JSON→Go
// round-trip, and prevents JavaScript from silently converting component values to numbers.
type poseComponents struct {
	X     string `json:"x"`
	Y     string `json:"y"`
	Z     string `json:"z"`
	Theta string `json:"theta"`
	OX    string `json:"ox"`
	OY    string `json:"oy"`
	OZ    string `json:"oz"`
	// Cloud is nil when the pose has no GoalCloud leeway.
	Cloud *poseCloudComponents `json:"cloud,omitempty"`
}

func poseToComponents(pose spatialmath.Pose) poseComponents {
	pt := pose.Point()
	ov := pose.Orientation().OrientationVectorDegrees()
	fmtFloat := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	return poseComponents{
		X:     fmtFloat(pt.X),
		Y:     fmtFloat(pt.Y),
		Z:     fmtFloat(pt.Z),
		Theta: fmtFloat(ov.Theta),
		OX:    fmtFloat(ov.OX),
		OY:    fmtFloat(ov.OY),
		OZ:    fmtFloat(ov.OZ),
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
	theta, err := parse("theta", pc.Theta)
	if err != nil {
		return nil, err
	}
	ox, err := parse("ox", pc.OX)
	if err != nil {
		return nil, err
	}
	oy, err := parse("oy", pc.OY)
	if err != nil {
		return nil, err
	}
	oz, err := parse("oz", pc.OZ)
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPose(
		r3.Vector{X: x, Y: y, Z: z},
		&spatialmath.OrientationVectorDegrees{Theta: theta, OX: ox, OY: oy, OZ: oz},
	), nil
}

// referenceframePoseCloudToComponents converts a referenceframe.PoseCloud to its string-valued
// wire representation, or nil if pc is nil.
func referenceframePoseCloudToComponents(pc *referenceframe.PoseCloud) *poseCloudComponents {
	if pc == nil {
		return nil
	}
	fmtFloat := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	return &poseCloudComponents{
		X:     fmtFloat(pc.X),
		Y:     fmtFloat(pc.Y),
		Z:     fmtFloat(pc.Z),
		OX:    fmtFloat(pc.OX),
		OY:    fmtFloat(pc.OY),
		OZ:    fmtFloat(pc.OZ),
		Theta: fmtFloat(pc.Theta),
	}
}

// componentsToReferenceframePoseCloud is the inverse of referenceframePoseCloudToComponents, or
// nil if pcc is nil.
func componentsToReferenceframePoseCloud(pcc *poseCloudComponents) (*referenceframe.PoseCloud, error) {
	if pcc == nil {
		return nil, nil
	}
	parse := func(label, s string) (float64, error) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing pose cloud %s: %w", label, err)
		}
		return v, nil
	}
	x, err := parse("x", pcc.X)
	if err != nil {
		return nil, err
	}
	y, err := parse("y", pcc.Y)
	if err != nil {
		return nil, err
	}
	z, err := parse("z", pcc.Z)
	if err != nil {
		return nil, err
	}
	ox, err := parse("ox", pcc.OX)
	if err != nil {
		return nil, err
	}
	oy, err := parse("oy", pcc.OY)
	if err != nil {
		return nil, err
	}
	oz, err := parse("oz", pcc.OZ)
	if err != nil {
		return nil, err
	}
	theta, err := parse("theta", pcc.Theta)
	if err != nil {
		return nil, err
	}
	return &referenceframe.PoseCloud{X: x, Y: y, Z: z, OX: ox, OY: oy, OZ: oz, Theta: theta}, nil
}

// poseInFrameToComponents converts a *referenceframe.PoseInFrame (a world-frame goal pose plus
// its optional GoalCloud) into the wire representation, preserving the cloud.
func poseInFrameToComponents(pif *referenceframe.PoseInFrame) poseComponents {
	pc := poseToComponents(pif.Pose())
	pc.Cloud = referenceframePoseCloudToComponents(pif.GoalCloud)
	return pc
}

// componentsToPoseInFrame is the inverse of poseInFrameToComponents: it builds a world-frame
// *referenceframe.PoseInFrame (with GoalCloud if present) from the wire representation.
func componentsToPoseInFrame(worldFrameName string, pc poseComponents) (*referenceframe.PoseInFrame, error) {
	pose, err := componentsToSpatialPose(pc)
	if err != nil {
		return nil, err
	}
	cloud, err := componentsToReferenceframePoseCloud(pc.Cloud)
	if err != nil {
		return nil, err
	}
	return referenceframe.NewPoseInFrameWithGoalCloud(worldFrameName, pose, cloud), nil
}

// computeGoalPoseMap returns the poses for one goal, keyed by frame name, transformed into the
// world frame and encoded as poseComponents (string-valued scalars).
func computeGoalPoseMap(req *armplanning.PlanRequest, goalIdx int) (map[string]poseComponents, error) {
	if goalIdx < 0 || goalIdx >= len(req.Goals) {
		return nil, fmt.Errorf("goal index %d out of range (have %d goals)", goalIdx, len(req.Goals))
	}
	poses, err := req.Goals[goalIdx].FilteredPoses(context.Background(), req.FrameSystem)
	if err != nil {
		return nil, err
	}
	result := make(map[string]poseComponents, len(poses))
	for frameName, poseValue := range poses {
		poseInWorldFrame := poseValue.Transform(
			referenceframe.NewPoseInFrame(
				req.FrameSystem.World().Name(),
				spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
		result[frameName] = poseInFrameToComponents(poseInWorldFrame)
	}
	return result, nil
}

// poseMapToDisplays converts a frame→poseComponents map into a sorted slice for template rendering.
// Orientation is shown as an orientation vector (axis + angle in degrees).
func poseMapToDisplays(poseMap map[string]poseComponents) []poseDisplay {
	displays := make([]poseDisplay, 0, len(poseMap))
	for frame, pc := range poseMap {
		poseStr := fmt.Sprintf("pos=[%s, %s, %s] ov=[%s°, (%s, %s, %s)]",
			pc.X, pc.Y, pc.Z, pc.Theta, pc.OX, pc.OY, pc.OZ)
		if pc.Cloud != nil {
			poseStr += fmt.Sprintf(" cloud=[x±%s, y±%s, z±%s, ox±%s, oy±%s, oz±%s, θ±%s°]",
				pc.Cloud.X, pc.Cloud.Y, pc.Cloud.Z, pc.Cloud.OX, pc.Cloud.OY, pc.Cloud.OZ, pc.Cloud.Theta)
		}
		displays = append(displays, poseDisplay{Frame: frame, Pose: poseStr})
	}
	sort.Slice(displays, func(i, j int) bool { return displays[i].Frame < displays[j].Frame })
	return displays
}

// frameSystemPosesToMap encodes a FrameSystemPoses (already world-frame) as a frame→poseComponents map.
func frameSystemPosesToMap(poses referenceframe.FrameSystemPoses) map[string]poseComponents {
	result := make(map[string]poseComponents, len(poses))
	for frameName, pif := range poses {
		result[frameName] = poseInFrameToComponents(pif)
	}
	return result
}

// buildIKInspectURL constructs the /ik-inspect URL with start config and goal poses encoded as
// JSON query params. All float64 values are represented as strings so they survive the
// Go→JSON→JS→JSON→Go round-trip without any numeric conversion. goalIndex identifies which goal
// (within the plan request's Goals list) these poses came from, and overridesParam is the
// already-encoded requestOverrides (if any) in effect for the rest of the request — both are
// carried through so the ik-inspect page can merge its own edits back into the full set of
// overrides when navigating back to the detail page.
func buildIKInspectURL(
	file string, goalIndex int, startConfig *referenceframe.LinearInputs,
	goalPoseMap map[string]poseComponents, overridesParam string,
) string {
	startJSON, _ := json.Marshal(linearInputsToStrings(startConfig))
	goalJSON, _ := json.Marshal(goalPoseMap)
	return "/ik-inspect?file=" + url.QueryEscape(file) +
		"&goal_index=" + strconv.Itoa(goalIndex) +
		"&start_config=" + url.QueryEscape(string(startJSON)) +
		"&goal_poses=" + url.QueryEscape(string(goalJSON)) +
		"&overrides=" + url.QueryEscape(overridesParam)
}

// requestOverrides carries in-progress edits to a PlanRequest's Goals entirely via URL query
// params (see the mpserver package doc). Goals is index-aligned with the base file's
// req.Goals; a nil entry means "use the file's original goal pose/cloud for that frame".
type requestOverrides struct {
	Goals []map[string]poseComponents `json:"goals,omitempty"`
}

// encodeOverrides JSON-encodes ov for embedding as a URL query param value (the caller is
// responsible for url.QueryEscape-ing it into a URL, or relying on net/http's automatic decoding
// of query param values read via r.URL.Query()).
func encodeOverrides(ov requestOverrides) string {
	data, err := json.Marshal(ov)
	if err != nil {
		return ""
	}
	return string(data)
}

// decodeOverrides parses a requestOverrides previously produced by encodeOverrides. An empty
// string decodes to a zero-value requestOverrides.
func decodeOverrides(s string) (requestOverrides, error) {
	var ov requestOverrides
	if s == "" {
		return ov, nil
	}
	if err := json.Unmarshal([]byte(s), &ov); err != nil {
		return requestOverrides{}, fmt.Errorf("parsing overrides: %w", err)
	}
	return ov, nil
}

// applyOverrides returns a shallow copy of req with each Goals[i]'s poses replaced by
// ov.Goals[i] where present. Goals without an override entry (index out of range, or a nil map
// at that index) are left untouched.
func applyOverrides(req *armplanning.PlanRequest, ov requestOverrides) (*armplanning.PlanRequest, error) {
	if len(ov.Goals) == 0 {
		return req, nil
	}
	worldFrameName := req.FrameSystem.World().Name()
	newGoals := make([]*armplanning.PlanState, len(req.Goals))
	copy(newGoals, req.Goals)
	for goalIdx, poseOverrides := range ov.Goals {
		if poseOverrides == nil || goalIdx >= len(newGoals) {
			continue
		}
		poses := make(referenceframe.FrameSystemPoses, len(poseOverrides))
		for frameName, pc := range poseOverrides {
			pif, err := componentsToPoseInFrame(worldFrameName, pc)
			if err != nil {
				return nil, fmt.Errorf("goal %d frame %q: %w", goalIdx, frameName, err)
			}
			poses[frameName] = pif
		}
		newGoals[goalIdx] = armplanning.NewPlanState(poses, nil)
	}
	reqCopy := *req
	reqCopy.Goals = newGoals
	return &reqCopy, nil
}

// collectGoalPoses computes req's actual goal poses in the world frame, without drawing them.
func collectGoalPoses(req *armplanning.PlanRequest) ([]spatialmath.Pose, error) {
	var goalPoses []spatialmath.Pose
	for _, goalPlanState := range req.Goals {
		poses, err := goalPlanState.FilteredPoses(context.Background(), req.FrameSystem)
		if err != nil {
			return nil, err
		}
		for _, poseValue := range poses {
			poseInWorldFrame := poseValue.Transform(
				referenceframe.NewPoseInFrame(
					req.FrameSystem.World().Name(),
					spatialmath.NewZeroPose())).(*referenceframe.PoseInFrame)
			goalPoses = append(goalPoses, poseInWorldFrame.Pose())
		}
	}
	return goalPoses, nil
}

// Stable entity IDs for the visualizer: passing the same ID to DrawPosesAsArrows on every call
// updates that one entity in place, instead of (as the deprecated client/client.DrawPoses did)
// piling up a new set of arrows on every call.
const (
	goalPosesEntityID     = "mpserver-goal-poses"
	candidatePoseEntityID = "mpserver-candidate-pose"

	goalPoseColor = "blue"
	// candidatePoseColor is used to draw a not-yet-applied, in-progress goal pose edit, distinct
	// from the blue used for the plan request's actual (already-applied) goal poses.
	candidatePoseColor = "orange"
)

func drawGoalPoses(req *armplanning.PlanRequest) error {
	goalPoses, err := collectGoalPoses(req)
	if err != nil {
		return err
	}
	_, err = vizapi.DrawPosesAsArrows(vizapi.DrawPosesAsArrowsOptions{
		ID:     goalPosesEntityID,
		Name:   "goal-poses",
		Poses:  goalPoses,
		Colors: []draw.Color{draw.ColorFromName(goalPoseColor)},
	})
	return err
}

func renderState(relPath, overridesParam string) error {
	req, err := armplanning.ReadRequestFromFile(filepath.Join(rdkRoot, relPath))
	if err != nil {
		return fmt.Errorf("reading plan file: %w", err)
	}
	overrides, err := decodeOverrides(overridesParam)
	if err != nil {
		return fmt.Errorf("decoding overrides: %w", err)
	}
	req, err = applyOverrides(req, overrides)
	if err != nil {
		return fmt.Errorf("applying overrides: %w", err)
	}
	startInputs := req.StartState.Configuration()
	if _, err := vizapi.RemoveAll(); err != nil {
		return fmt.Errorf("clearing visualizer: %w", err)
	}
	if _, err := vizapi.DrawWorldState(vizapi.DrawWorldStateOptions{
		WorldState:  req.GetWorldState(),
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
		return fmt.Errorf("drawing world state: %w", err)
	}
	if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
		return fmt.Errorf("drawing frame system: %w", err)
	}
	if err := drawGoalPoses(req); err != nil {
		return fmt.Errorf("drawing goal poses: %w", err)
	}
	return nil
}

// drawCandidatePose (re-)draws the request's actual goal poses (goalPosesEntityID, blue) and the
// in-progress candidate pose (candidatePoseEntityID, candidatePoseColor) as two independent
// stable-ID entities. Each call updates those same two entities in place — rather than piling up
// new ones — and deliberately does not touch the world state or frame system draws (unlike
// renderState): those don't change as a pose is edited, so leaving them alone avoids the flicker
// of a full clear + redraw on every debounced keystroke.
func drawCandidatePose(req *armplanning.PlanRequest, candidatePoses []spatialmath.Pose) error {
	if err := drawGoalPoses(req); err != nil {
		return err
	}
	_, err := vizapi.DrawPosesAsArrows(vizapi.DrawPosesAsArrowsOptions{
		ID:     candidatePoseEntityID,
		Name:   "candidate-pose",
		Poses:  candidatePoses,
		Colors: []draw.Color{draw.ColorFromName(candidatePoseColor)},
	})
	return err
}

// visualizeLinearTrajectory renders a sequence of LinearInputs steps in the visualizer.
// It bails out early (without error) if ctx is cancelled by a newer render.
// visualizeLinearTrajectory assumes the file's world state and frame system are already
// established in the visualizer (by an earlier renderState call) and only needs to move the
// frame system through steps — it deliberately does not call vizapi.RemoveAll first. DrawWorldState
// and DrawFrameSystem key each entity's identity by stable data (obstacle/frame name + parent), so
// redrawing the same file's scene updates those entities in place rather than requiring a full
// clear-then-redraw, which would otherwise leave the scene empty for a beat every render.
func visualizeLinearTrajectory(ctx context.Context, req *armplanning.PlanRequest, steps []*referenceframe.LinearInputs) error {
	startInputs := req.StartState.Configuration()
	if _, err := vizapi.DrawWorldState(vizapi.DrawWorldStateOptions{
		WorldState:  req.GetWorldState(),
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
		return err
	}
	if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
		FrameSystem: req.FrameSystem,
		Inputs:      startInputs,
	}); err != nil {
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
				if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
					FrameSystem: req.FrameSystem,
					Inputs:      mp.ToFrameSystemInputs(),
				}); err != nil {
					return err
				}
				time.Sleep(renderFramePeriod)
			}
		}
		if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
			FrameSystem: req.FrameSystem,
			Inputs:      step.ToFrameSystemInputs(),
		}); err != nil {
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
		overridesParam := r.URL.Query().Get("overrides")
		overrides, err := decodeOverrides(overridesParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req, err = applyOverrides(req, overrides)
		if err != nil {
			http.Error(w, fmt.Sprintf("applying overrides: %v", err), http.StatusBadRequest)
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
			poseMapJSON, _ := json.Marshal(poseMap)
			goals[idx] = goalDetail{
				Index:         idx,
				StartConfig:   startConfig,
				GoalPoses:     poseMapToDisplays(poseMap),
				GoalPosesJSON: template.JS(poseMapJSON), //nolint: gosec
				IKInspectURL:  buildIKInspectURL(file, idx, startLI, poseMap, overridesParam),
			}
		}
		data := detailData{
			File:           file,
			OverridesParam: overridesParam,
			Frames:         buildFrameInfo(req.FrameSystem),
			StartInputs:    startConfig,
			Goals:          goals,
			Constraints:    buildDetailConstraints(req.Constraints),
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
		goalIndex := 0
		if gi := r.URL.Query().Get("goal_index"); gi != "" {
			if parsed, err := strconv.Atoi(gi); err == nil {
				goalIndex = parsed
			}
		}
		overridesParam := r.URL.Query().Get("overrides")
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
			GoalIndex:       goalIndex,
			OverridesParam:  overridesParam,
			StartConfig:     startConfig,
			StartConfigJSON: template.JS(startConfigJSONBytes), //nolint: gosec
			GoalPoses:       poseMapToDisplays(goalPoseMap),
			//nolint: gosec
			GoalPosesJSON: template.JS(goalPosesJSONBytes),
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
			pif, err := componentsToPoseInFrame(worldFrameName, pc)
			if err != nil {
				writeJSON(w, ikInspectRunResult{Error: fmt.Sprintf("parsing goal pose for %q: %v", frameName, err)})
				return
			}
			goalPoses[frameName] = pif
		}

		armplanning.ClearSeedCache()
		result, err := InspectIK(r.Context(), logger.Sublogger("ik-inspect"), req, segmentStart.ToFrameSystemInputs(), goalPoses, 10)
		if err != nil {
			writeJSON(w, ikInspectRunResult{Error: err.Error()})
			return
		}

		out := ikInspectRunResult{Seeds: make([][]ikInspectCellResult, len(result.Rows)), SeedLabels: result.SeedLabels}
		for seedIdx, cells := range result.Rows {
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
				if cell.LastGoodInputs != nil {
					row.LastGoodInputs = linearInputsToStrings(cell.LastGoodInputs)
				}
				rows[cellIdx] = row
			}
			out.Seeds[seedIdx] = rows
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
		overridesParam := r.URL.Query().Get("overrides")
		overrides, err := decodeOverrides(overridesParam)
		if err != nil {
			writeJSON(w, planRunResult{Error: err.Error()})
			return
		}
		req, err = applyOverrides(req, overrides)
		if err != nil {
			writeJSON(w, planRunResult{Error: fmt.Sprintf("applying overrides: %v", err)})
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
		for goalIdx, pg := range meta.PerGoal {
			poseMap := frameSystemPosesToMap(pg.GoalPoses)
			pgResult := perGoalResult{
				IKInspectURL:             buildIKInspectURL(file, goalIdx, pg.StartConfiguration, poseMap, overridesParam),
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
		// No vizapi.RemoveAll here: the file's world state/frame system were already
		// established by an earlier renderState call, so redrawing them (identity keyed by
		// obstacle/frame name + parent) updates those entities in place instead of clearing the
		// whole scene right before redrawing it.
		if _, err := vizapi.DrawWorldState(vizapi.DrawWorldStateOptions{
			WorldState:  req.GetWorldState(),
			FrameSystem: req.FrameSystem,
			Inputs:      startInputs,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
			FrameSystem: req.FrameSystem,
			Inputs:      li.ToFrameSystemInputs(),
		}); err != nil {
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

		// No vizapi.RemoveAll here — see handleRenderSolution's comment: the file's world
		// state/frame system are already established, so redrawing them updates in place.
		if _, err := vizapi.DrawWorldState(vizapi.DrawWorldStateOptions{
			WorldState:  req.GetWorldState(),
			FrameSystem: req.FrameSystem,
			Inputs:      startInputs,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := vizapi.DrawFrameSystem(vizapi.DrawFrameSystemOptions{
			FrameSystem: req.FrameSystem,
			Inputs:      startInputs,
		}); err != nil {
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
			if _, err := vizapi.DrawGeometriesInFrame(vizapi.DrawGeometriesInFrameOptions{
				Geometries: shadowGIF,
				Colors:     []draw.Color{draw.ColorFromName(shadowColor)},
			}); err != nil {
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

// handlePlanDownload serves the plan request (with any in-progress overrides applied) as a
// downloadable JSON file, in the same format ReadRequestFromFile expects back.
func handlePlanDownload(logger logging.Logger) http.HandlerFunc {
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
		overrides, err := decodeOverrides(r.URL.Query().Get("overrides"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req, err = applyOverrides(req, overrides)
		if err != nil {
			http.Error(w, fmt.Sprintf("applying overrides: %v", err), http.StatusBadRequest)
			return
		}
		data, err := json.Marshal(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("marshaling plan request: %v", err), http.StatusInternalServerError)
			return
		}
		downloadName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)) + "-edited.json"
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
		if _, err := w.Write(data); err != nil {
			logger.Errorf("writing plan download: %v", err)
		}
	}
}

// handleRenderCandidatePose draws an in-progress (not-yet-applied) goal pose edit in the
// visualizer, distinct in color from the request's actual goal poses, so a user editing a pose
// gets live feedback on where it will land. Each call fully replaces the previously drawn
// candidate. The request body is a JSON object of frame name -> poseComponents, i.e. exactly what
// the page's readPoseEditors() produces.
func handleRenderCandidatePose(logger logging.Logger) http.HandlerFunc {
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
		var poseMap map[string]poseComponents
		if err := json.NewDecoder(r.Body).Decode(&poseMap); err != nil {
			http.Error(w, fmt.Sprintf("decoding goal poses: %v", err), http.StatusBadRequest)
			return
		}
		worldFrameName := req.FrameSystem.World().Name()
		var candidatePoses []spatialmath.Pose
		for frameName, pc := range poseMap {
			pif, err := componentsToPoseInFrame(worldFrameName, pc)
			if err != nil {
				// Best-effort: the user is likely mid-keystroke (e.g. a lone "-"), so skip an
				// unparsable frame rather than failing the whole candidate render.
				logger.Debugf("skipping unparsable candidate pose for %q: %v", frameName, err)
				continue
			}
			candidatePoses = append(candidatePoses, pif.Pose())
		}
		beginRender()
		if err := drawCandidatePose(req, candidatePoses); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
		overridesParam := r.URL.Query().Get("overrides")
		beginRender()
		if err := renderState(file, overridesParam); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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
	http.HandleFunc("/plan/download", handlePlanDownload(logger))
	http.HandleFunc("/render-plan", handleRenderPlan(logger))
	http.HandleFunc("/render-start", handleRenderStart(logger))
	http.HandleFunc("/render-solution", handleRenderSolution(logger))
	http.HandleFunc("/render-shadows", handleRenderShadows(logger))
	http.HandleFunc("/render-candidate-pose", handleRenderCandidatePose(logger))

	addr := "localhost:8080"
	logger.Infof("listening on http://%s", addr)

	//nolint: gosec
	return http.ListenAndServe(addr, nil)
}
