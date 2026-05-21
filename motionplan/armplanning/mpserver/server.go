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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	viz "github.com/viam-labs/motion-tools/client/client"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	rdkRoot           = "/home/dgottlieb/viam/rdk"
	planFilesRoot     = rdkRoot + "/mplans"
	renderFramePeriod = 5 * time.Millisecond
	// shadowCount is the number of intermediate configurations to draw between start and end when
	// rendering shadows along a straight-line path. We interpolate directly instead of going through
	// InterpolateSegmentFS because that helper also enforces a per-joint step size (~range/1000)
	// which produces hundreds-to-thousands of steps for typical arm motions.
	shadowCount       = 10
	shadowFramePeriod = 100 * time.Millisecond
)

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
  div.textContent = 'Running…';
  fetch('/plan/run?file=' + encodeURIComponent('{{.File}}') + '&timeout=' + encodeURIComponent(timeout),
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
        html += '<h3>Goal ' + goalIdx + '</h3>';
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
    })
    .catch(err => { if (err.name !== 'AbortError') div.textContent = 'Fetch error: ' + err; });
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

function formatInputs(inputs) {
  return Object.entries(inputs)
    .map(([f, vs]) => f + ': [' + vs.map(v => v.toFixed(4)).join(', ') + ']')
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

// ---- data types ----

type frameInfo struct {
	Name   string
	DoF    int
	Parent string
}

type detailData struct {
	File        string
	Frames      []frameInfo
	StartInputs []frameInputs
}

type frameInputs struct {
	Name   string
	Inputs string
}

type planRunResult struct {
	Error          string          `json:"error,omitempty"`
	Steps          int             `json:"steps,omitempty"`
	Duration       string          `json:"duration,omitempty"`
	GoalsProcessed int             `json:"goals_processed,omitempty"`
	Partial        bool            `json:"partial,omitempty"`
	PartialError   string          `json:"partial_error,omitempty"`
	PerGoal        []perGoalResult `json:"per_goal,omitempty"`
}

type perGoalResult struct {
	ValidSolutions           []solutionNodeResult `json:"valid_solutions,omitempty"`
	CheckPathFailures        []solutionNodeResult `json:"check_path_failures,omitempty"`
	ConstraintFailuresByType map[string]int       `json:"constraint_failures_by_type,omitempty"`
}

type solutionNodeResult struct {
	Score          float64              `json:"score"`
	CheckPathError string               `json:"check_path_error,omitempty"`
	Inputs         map[string][]float64 `json:"inputs"`
	LastGoodInputs map[string][]float64 `json:"last_good_inputs,omitempty"`
}

func linearInputsToFloats(li *referenceframe.LinearInputs) map[string][]float64 {
	out := make(map[string][]float64)
	for frameName, inputs := range li.Items() {
		if len(inputs) == 0 {
			continue
		}
		floats := make([]float64, len(inputs))
		copy(floats, inputs)
		out[frameName] = floats
	}
	return out
}

func floatsToLinearInputs(data map[string][]float64) *referenceframe.LinearInputs {
	li := referenceframe.NewLinearInputs()
	for frameName, floats := range data {
		li.Put(frameName, floats)
	}
	return li
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
			parts[i] = strconv.FormatFloat(v, 'f', 4, 64)
		}
		rows = append(rows, frameInputs{Name: name, Inputs: "[" + strings.Join(parts, ", ") + "]"})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
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

func visualizePlan(req *armplanning.PlanRequest, plan motionplan.Plan) error {
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
	for idx := range plan.Path() {
		if idx > 0 {
			midPoints, err := motionplan.InterpolateSegmentFS(
				&motionplan.SegmentFS{
					StartConfiguration: plan.Trajectory()[idx-1].ToLinearInputs(),
					EndConfiguration:   plan.Trajectory()[idx].ToLinearInputs(),
					FS:                 req.FrameSystem,
				}, 2)
			if err != nil {
				return err
			}
			for _, mp := range midPoints {
				if err := viz.DrawFrameSystem(req.FrameSystem, mp.ToFrameSystemInputs()); err != nil {
					return err
				}
				time.Sleep(renderFramePeriod)
			}
		}
		if err := viz.DrawFrameSystem(req.FrameSystem, plan.Trajectory()[idx]); err != nil {
			return err
		}
		time.Sleep(renderFramePeriod)
	}
	return nil
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
		data := detailData{
			File:        file,
			Frames:      buildFrameInfo(req.FrameSystem),
			StartInputs: buildStartInputs(req.StartState.Configuration()),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := detailTmpl.Execute(w, data); err != nil {
			logger.Errorf("rendering detail: %v", err)
		}
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

		plan, meta, err := armplanning.PlanMotion(r.Context(), logger, req)
		if err != nil {
			writeJSON(w, planRunResult{Error: err.Error()})
			return
		}

		if vizErr := visualizePlan(req, plan); vizErr != nil {
			logger.Warnf("visualization failed (motion-tools server may not be running): %v", vizErr)
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
			pgResult := perGoalResult{
				ConstraintFailuresByType: pg.ConstraintFailuresByType,
			}
			for _, sn := range pg.SolutionNodes {
				row := solutionNodeResult{
					Score:  sn.Score,
					Inputs: linearInputsToFloats(sn.Inputs),
				}
				if sn.CheckPathError != nil {
					row.CheckPathError = sn.CheckPathError.Error()
					if sn.LastGoodInputs != nil {
						row.LastGoodInputs = linearInputsToFloats(sn.LastGoodInputs)
					}
					pgResult.CheckPathFailures = append(pgResult.CheckPathFailures, row)
				} else {
					pgResult.ValidSolutions = append(pgResult.ValidSolutions, row)
				}
			}
			result.PerGoal = append(result.PerGoal, pgResult)
		}

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
		var inputFloats map[string][]float64
		if err := json.NewDecoder(r.Body).Decode(&inputFloats); err != nil {
			http.Error(w, fmt.Sprintf("decoding inputs: %v", err), http.StatusBadRequest)
			return
		}
		li := floatsToLinearInputs(inputFloats)
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
		var inputFloats map[string][]float64
		if err := json.NewDecoder(r.Body).Decode(&inputFloats); err != nil {
			http.Error(w, fmt.Sprintf("decoding inputs: %v", err), http.StatusBadRequest)
			return
		}
		end := floatsToLinearInputs(inputFloats)
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
		if err := drawShadows(req.FrameSystem, midPoints); err != nil {
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
func drawShadows(fs *referenceframe.FrameSystem, configs []*referenceframe.LinearInputs) error {
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

func handleRenderStart(logger logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			http.Error(w, "missing file parameter", http.StatusBadRequest)
			return
		}
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
	http.HandleFunc("/plan/run", handlePlanRun(logger))
	http.HandleFunc("/render-start", handleRenderStart(logger))
	http.HandleFunc("/render-solution", handleRenderSolution(logger))
	http.HandleFunc("/render-shadows", handleRenderShadows(logger))

	addr := "localhost:8080"
	logger.Infof("listening on http://%s", addr)
	//nolint
	return http.ListenAndServe(addr, nil)
}
