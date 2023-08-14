// Package eva implements the Eva robot from Automata.
// api found at
// https://github.com/automata-tech/eva_python_sdk/blob/development/evasdk/eva_http_client.py
package eva

import (
	"bytes"
	"context"

	// for embedding model file.
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is the name of the eva model of an arm component.
var Model = resource.DefaultModelFamily.WithModel("eva")

// Config is used for converting config attributes.
type Config struct {
	resource.TriviallyValidateConfig
	Token string `json:"token"`
	Host  string `json:"host"`
}

//go:embed eva_kinematics.json
var evamodeljson []byte

func init() {
	resource.RegisterComponent(arm.API, Model, resource.Registration[arm.Arm, *Config]{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (arm.Arm, error) {
			return NewEva(ctx, conf, logger)
		},
	})
}

type evaData struct {
	// map[estop:false]
	Global map[string]interface{}

	// map[d0:false d1:false d2:false d3:false ee_a0:0.034 ee_a1:0.035 ee_d0:false ee_d1:false]
	GlobalInputs map[string]interface{} `json:"global.inputs"`

	// map[d0:false d1:false d2:false d3:false ee_d0:false ee_d1:false]
	GlobalOutputs map[string]interface{} `json:"global.outputs"`

	// scheduler : map[enabled:false]
	Scheduler map[string]interface{}

	// [0.0008628905634395778 0 0.0002876301878131926 0 -0.00038350690738298 0.0005752603756263852]
	ServosPosition []float64 `json:"servos.telemetry.position"`

	// [53.369998931884766 43.75 43.869998931884766 43.869998931884766 51 48.619998931884766]
	ServosTemperature []float64 `json:"servos.telemetry.temperature"`

	// [0 0 0 0 0 0]
	ServosVelocity []float64 `json:"servos.telemetry.velocity"`

	// map[loop_count:1 loop_target:1 run_mode:not_running state:ready toolpath_hash:4d8 toolpath_name:Uploaded]
	Control map[string]interface{}
}

type eva struct {
	resource.Named
	resource.AlwaysRebuild
	host         string
	version      string
	token        string
	sessionToken string

	moveLock sync.Mutex
	logger   golog.Logger
	model    referenceframe.Model

	frameJSON []byte

	opMgr *operation.SingleOperationManager
}

// NewEva TODO.
func NewEva(ctx context.Context, conf resource.Config, logger golog.Logger) (arm.Arm, error) {
	model, err := MakeModelFrame(conf.Name)
	if err != nil {
		return nil, err
	}

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	e := &eva{
		Named:     conf.ResourceName().AsNamed(),
		host:      newConf.Host,
		version:   "v1",
		token:     newConf.Token,
		logger:    logger,
		model:     model,
		frameJSON: evamodeljson,
		opMgr:     operation.NewSingleOperationManager(),
	}

	name, err := e.apiName(ctx)
	if err != nil {
		return nil, err
	}

	e.logger.Debugf("connected to eva: %v", name)

	return e, nil
}

func (e *eva) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	data, err := e.DataSnapshot(ctx)
	if err != nil {
		return &pb.JointPositions{}, err
	}
	return referenceframe.JointPositionsFromRadians(data.ServosPosition), nil
}

// EndPosition computes and returns the current cartesian position.
func (e *eva) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := e.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputeOOBPosition(e.model, joints)
}

// MoveToPosition moves the arm to the specified cartesian position.
func (e *eva) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	ctx, done := e.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, e.logger, e, pos)
}

func (e *eva) MoveToJointPositions(ctx context.Context, newPositions *pb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, e, newPositions); err != nil {
		return err
	}
	ctx, done := e.opMgr.New(ctx)
	defer done()

	radians := referenceframe.JointPositionsToRadians(newPositions)

	err := e.doMoveJoints(ctx, radians)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "Reset hard errors first") {
		return err
	}

	if err2 := e.resetErrors(ctx); err2 != nil {
		return errors.Wrapf(multierr.Combine(err, err2), "move failure, and couldn't reset errors")
	}

	return e.doMoveJoints(ctx, radians)
}

func (e *eva) doMoveJoints(ctx context.Context, joints []float64) error {
	if err := e.apiLock(ctx); err != nil {
		return err
	}
	defer e.apiUnlock(ctx)

	return e.apiControlGoTo(ctx, joints)
}

func (e *eva) apiRequest(ctx context.Context, method, path string, payload interface{}, auth bool, out interface{}) error {
	return e.apiRequestRetry(ctx, method, path, payload, auth, out, true)
}

func (e *eva) apiRequestRetry(
	ctx context.Context,
	method string,
	path string,
	payload interface{},
	auth bool,
	out interface{},
	retry bool,
) error {
	fullPath := fmt.Sprintf("http://%s/api/%s/%s", e.host, e.version, path)

	var reqReader io.Reader
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
	defer func() {
		utils.UncheckedError(res.Body.Close())
	}()

	if res.StatusCode == http.StatusUnauthorized {
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

	if res.StatusCode != http.StatusOK {
		more := ""
		if res.Body != nil {
			more2, e2 := io.ReadAll(res.Body)
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

func (e *eva) stop(ctx context.Context) error {
	if e.opMgr.OpRunning() {
		return multierr.Combine(
			e.apiRequest(ctx, "POST", "controls/pause", nil, true, nil),  // pause robot
			e.apiRequest(ctx, "POST", "controls/cancel", nil, true, nil), // make state ready to run again
		)
	}
	return nil
}

func (e *eva) Stop(ctx context.Context, extra map[string]interface{}) error {
	return e.stop(ctx)
}

func (e *eva) IsMoving(ctx context.Context) (bool, error) {
	return e.opMgr.OpRunning(), nil
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

// ModelFrame returns all the information necessary for including the arm in a FrameSystem.
func (e *eva) ModelFrame() referenceframe.Model {
	return e.model
}

func (e *eva) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := e.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return e.model.InputFromProtobuf(res), nil
}

func (e *eva) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	positionDegs := e.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, e, positionDegs); err != nil {
		return err
	}
	return e.MoveToJointPositions(ctx, positionDegs, nil)
}

func (e *eva) Close(ctx context.Context) error {
	return e.stop(ctx)
}

// MakeModelFrame returns the kinematics model of the eva arm, also has all Frame information.
func MakeModelFrame(name string) (referenceframe.Model, error) {
	return referenceframe.UnmarshalModelJSON(evamodeljson, name)
}

// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
// current location relative to the frame of the resource.
func (e *eva) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := e.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := e.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}
