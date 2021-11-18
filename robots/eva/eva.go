// Package eva implements the Eva robot from Automata.
package eva

import (
	"bytes"
	"context"
	_ "embed" // for embedding model file
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

//go:embed eva_kinematics.json
var evamodeljson []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "eva", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewEva(ctx, config, logger)
		},
	})
}

type evaData struct {
	// map[estop:false]
	Global map[string]interface{}

	// map[d0:false d1:false d2:false d3:false ee_a0:0.034 ee_a1:0.035 ee_d0:false ee_d1:false]
	GlobalInputs map[string]interface{} `json:"global.inputs"`

	//map[d0:false d1:false d2:false d3:false ee_d0:false ee_d1:false]
	GlobalOutputs map[string]interface{} `json:"global.outputs"`

	//scheduler : map[enabled:false]
	Scheduler map[string]interface{}

	//[0.0008628905634395778 0 0.0002876301878131926 0 -0.00038350690738298 0.0005752603756263852]
	ServosPosition []float64 `json:"servos.telemetry.position"`

	//[53.369998931884766 43.75 43.869998931884766 43.869998931884766 51 48.619998931884766]
	ServosTemperature []float64 `json:"servos.telemetry.temperature"`

	//[0 0 0 0 0 0]
	ServosVelocity []float64 `json:"servos.telemetry.velocity"`

	//map[loop_count:1 loop_target:1 run_mode:not_running state:ready toolpath_hash:4d8 toolpath_name:Uploaded]
	Control map[string]interface{}
}

type eva struct {
	host         string
	version      string
	token        string
	sessionToken string

	moveLock *sync.Mutex
	logger   golog.Logger
	model    *frame.Model
	ik       kinematics.InverseKinematics

	frameJSON []byte
}

func (e *eva) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	data, err := e.DataSnapshot(ctx)
	if err != nil {
		return &pb.ArmJointPositions{}, err
	}
	return frame.JointPositionsFromRadians(data.ServosPosition), nil
}

// CurrentPosition computes and returns the current cartesian position.
func (e *eva) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := e.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(e.ik.Model(), joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (e *eva) MoveToPosition(ctx context.Context, pos *commonpb.Pose) error {
	joints, err := e.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := e.ik.Solve(ctx, pos, frame.JointPosToInputs(joints))
	if err != nil {
		return err
	}
	return e.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
}

func (e *eva) MoveToJointPositions(ctx context.Context, newPositions *pb.ArmJointPositions) error {
	radians := frame.JointPositionsToRadians(newPositions)

	err := e.doMoveJoints(ctx, radians)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "Reset hard errors first") {
		return err
	}

	err2 := e.resetErrors(ctx)
	if err2 != nil {
		return errors.Errorf("move failure, and couldn't reset errors %w", multierr.Combine(err, err2))
	}

	return e.doMoveJoints(ctx, radians)
}

func (e *eva) doMoveJoints(ctx context.Context, joints []float64) error {
	err := e.apiLock(ctx)
	if err != nil {
		return err
	}
	defer e.apiUnlock(ctx)

	return e.apiControlGoTo(ctx, joints)
}

func (e *eva) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("not done yet")
}

func (e *eva) Close() error {
	return nil
}

func (e *eva) apiRequest(ctx context.Context, method string, path string, payload interface{}, auth bool, out interface{}) error {
	return e.apiRequestRetry(ctx, method, path, payload, auth, out, true)
}

func (e *eva) apiRequestRetry(ctx context.Context, method string, path string, payload interface{}, auth bool, out interface{}, retry bool) error {
	fullPath := fmt.Sprintf("http://%s/api/%s/%s", e.host, e.version, path)

	var reqReader io.Reader = nil
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reqReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullPath, reqReader)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	if auth {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", e.sessionToken))
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(res.Body.Close)

	if res.StatusCode == 401 {
		// need to login

		if !retry {
			return errors.New("got 401 from eva after trying to login")
		}

		type Temp struct {
			Token string
		}
		t := Temp{}
		err = e.apiRequestRetry(ctx, "POST", "auth", map[string]string{"token": e.token}, false, &t, false)
		if err != nil {
			return err
		}

		e.sessionToken = t.Token
		return e.apiRequestRetry(ctx, method, path, payload, auth, out, false)
	}

	if res.StatusCode != 200 {
		more := ""
		if res.Body != nil {
			more2, e2 := ioutil.ReadAll(res.Body)
			if e2 == nil {
				more = string(more2)
			}
		}

		return errors.Errorf("got unexpected response code: %d for %s %s", res.StatusCode, fullPath, more)
	}

	if out == nil {
		return nil
	}

	if !strings.HasPrefix(res.Header["Content-Type"][0], "application/json") {
		return errors.Errorf("expected json response from eva, got: %v", res.Header["Content-Type"])
	}

	decoder := json.NewDecoder(res.Body)

	return decoder.Decode(out)
}

func (e *eva) apiName(ctx context.Context) (string, error) {
	type Temp struct {
		Name string
	}
	t := Temp{}

	err := e.apiRequest(ctx, "GET", "name", nil, false, &t)

	if err != nil {
		return "", err
	}

	return t.Name, nil
}

func (e *eva) resetErrors(ctx context.Context) error {
	e.moveLock.Lock()
	defer e.moveLock.Unlock()

	err := e.apiLock(ctx)
	if err != nil {
		return err
	}
	defer e.apiUnlock(ctx)

	err = e.apiRequest(ctx, "POST", "controls/reset_errors", nil, true, nil)
	if err != nil {
		return err
	}
	utils.SelectContextOrWait(ctx, 100*time.Millisecond)
	return ctx.Err()
}

func (e *eva) DataSnapshot(ctx context.Context) (evaData, error) {
	type Temp struct {
		Snapshot evaData
	}
	res := Temp{}

	err := e.apiRequest(ctx, "GET", "data/snapshot", nil, true, &res)
	return res.Snapshot, err
}

func (e *eva) apiControlGoTo(ctx context.Context, joints []float64) error {
	body := map[string]interface{}{"joints": joints, "mode": "teach"} // TODO(erh): change to automatic
	err := e.apiRequest(ctx, "POST", "controls/go_to", &body, true, nil)
	if err != nil {
		return err
	}

	// we have to poll till we're done to unlock safely
	return e.loopTillNotRunning(ctx)
}

func (e *eva) loopTillNotRunning(ctx context.Context) error {
	for {
		data, err := e.DataSnapshot(ctx)
		if err != nil {
			return err
		}

		if data.Control["run_mode"] == "not_running" {
			break
		}

		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return ctx.Err()
		}
	}

	return nil
}

func (e *eva) apiLock(ctx context.Context) error {
	return e.apiRequest(ctx, "POST", "controls/lock", nil, true, nil)
}

func (e *eva) apiUnlock(ctx context.Context) {
	err := e.apiRequest(ctx, "DELETE", "controls/lock", nil, true, nil)
	if err != nil {
		e.logger.Debugf("eva unlock failed: %s", err)
	}
}

// ModelFrame returns all the information necessary for including the arm in a FrameSystem
func (e *eva) ModelFrame() *frame.Model {
	return e.model
}

func (e *eva) CurrentInputs(ctx context.Context) ([]frame.Input, error) {
	res, err := e.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return frame.JointPosToInputs(res), nil
}

func (e *eva) GoToInputs(ctx context.Context, goal []frame.Input) error {
	return e.MoveToJointPositions(ctx, frame.InputsToJointPos(goal))
}

// EvaModel() returns the kinematics model of the Eva, also has all Frame information.
func evaModel() (*frame.Model, error) {
	return frame.ParseJSON(evamodeljson, "")
}

// NewEva TODO
func NewEva(ctx context.Context, cfg config.Component, logger golog.Logger) (arm.Arm, error) {
	attrs := cfg.Attributes
	host := cfg.Host
	model, err := evaModel()
	if err != nil {
		return nil, err
	}
	ik, err := kinematics.CreateCombinedIKSolver(model, logger, 4)
	if err != nil {
		return nil, err
	}

	e := &eva{
		host:      host,
		version:   "v1",
		token:     attrs.String("token"),
		logger:    logger,
		moveLock:  &sync.Mutex{},
		model:     model,
		ik:        ik,
		frameJSON: evamodeljson,
	}

	name, err := e.apiName(ctx)
	if err != nil {
		return nil, err
	}

	e.logger.Debugf("connected to eva: %v", name)

	return e, nil
}
