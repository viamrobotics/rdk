package arm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/edaniels/golog"
)

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
}

type evaPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type evaOrientation struct {
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type evaKinematics struct {
	Position    evaPosition
	Orientation evaOrientation
}

func (e *eva) CurrentPosition() (CartesianInfo, error) {
	data, err := e.DataSnapshot()
	if err != nil {
		return CartesianInfo{}, err
	}

	fk, err := e.apiCalcForwardKinematics(data.ServosPosition)
	if err != nil {
		return CartesianInfo{}, err
	}

	golog.Global.Debugf("eva CurrentPosition %v", fk)

	ci := CartesianInfo{}
	ci.X = fk.Position.X
	ci.Y = fk.Position.Y
	ci.Z = fk.Position.Z

	// TODO(erh): finish orientation stuff
	ci.Rx = fk.Orientation.X
	ci.Ry = fk.Orientation.Y
	ci.Rz = fk.Orientation.Z
	//ci.W = fk.Orientation.W

	return ci, nil
}

func (e *eva) MoveToPositionC(c CartesianInfo) error {
	k := evaKinematics{}
	k.Position.X = c.X
	k.Position.Y = c.Y
	k.Position.Z = c.Z

	data, err := e.CurrentPosition()
	if err != nil {
		return err
	}

	// TODO(erh): what??
	//k.Orientation.W = data.W
	k.Orientation.X = data.Rx
	k.Orientation.Y = data.Ry
	k.Orientation.Z = data.Rz

	joints, err := e.apiCalcInverseKinematics(k)
	if err != nil {
		return err
	}

	err = e.apiLock()
	if err != nil {
		return err
	}
	defer e.apiUnlock()

	return e.apiControlGoTo(joints, true)
}

func (e *eva) MoveToPosition(x, y, z, rx, ry, rz float64) error {
	return fmt.Errorf("not done yet")
}

func (e *eva) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("not done yet")
}

func (e *eva) Close() {

}

func (e *eva) apiRequest(method string, path string, payload interface{}, auth bool, out interface{}) error {
	return e.apiRequestLower(method, path, payload, auth, out, true)
}

func (e *eva) apiRequestLower(method string, path string, payload interface{}, auth bool, out interface{}, retry bool) error {
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

	if auth {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", e.sessionToken))
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode == 401 {
		// need to login

		if !retry {
			return fmt.Errorf("got 401 from eva after trying to login")
		}

		type Temp struct {
			Token string
		}
		t := Temp{}
		err = e.apiRequestLower("POST", "auth", map[string]string{"token": e.token}, false, &t, false)
		if err != nil {
			return err
		}

		e.sessionToken = t.Token
		return e.apiRequestLower(method, path, payload, auth, out, false)
	}

	if res.StatusCode != 200 {
		more := ""
		if res.Body != nil {
			more2, e2 := ioutil.ReadAll(res.Body)
			if e2 == nil {
				more = string(more2)
			}
		}

		return fmt.Errorf("got unexpected response code: %d for %s %s", res.StatusCode, fullPath, more)
	}

	if out == nil {
		return nil
	}

	if !strings.HasPrefix(res.Header["Content-Type"][0], "application/json") {
		return fmt.Errorf("expected json response from eva, got: %v", res.Header["Content-Type"])
	}

	decoder := json.NewDecoder(res.Body)

	return decoder.Decode(out)
}

func (e *eva) apiName() (string, error) {
	type Temp struct {
		Name string
	}
	t := Temp{}

	err := e.apiRequest("GET", "name", nil, false, &t)

	if err != nil {
		return "", err
	}

	return t.Name, nil
}

func (e *eva) DataSnapshot() (evaData, error) {
	type Temp struct {
		Snapshot evaData
	}
	res := Temp{}

	err := e.apiRequest("GET", "data/snapshot", nil, true, &res)
	return res.Snapshot, err
}

func (e *eva) apiControlGoTo(joints []float64, block bool) error {
	body := map[string]interface{}{"joints": joints, "mode": "teach"} // TODO(erh): change to automatic
	err := e.apiRequest("POST", "controls/go_to", &body, true, nil)
	if err != nil {
		return err
	}

	if block {
		golog.Global.Debugf("i don't know how to block: %s", err)
		time.Sleep(1000 * time.Millisecond)
	}
	return nil
}

func (e *eva) apiCalcForwardKinematics(joints []float64) (evaKinematics, error) {
	body := map[string]interface{}{"joints": joints}
	type Temp struct {
		Fk evaKinematics
	}
	res := &Temp{}
	err := e.apiRequest("PUT", "calc/forward_kinematics", body, true, &res)
	return res.Fk, err
}

func (e *eva) apiCalcInverseKinematics(k evaKinematics) ([]float64, error) {
	body := map[string]interface{}{"guess": []float64{0, 0, 0, 0, 0, 0}, "position": k.Position, "orientation": k.Orientation}

	type Temp1 struct {
		Joints []float64
		Result string
		Error  interface{}
	}

	type Temp struct {
		Ik Temp1
	}
	res := &Temp{}

	err := e.apiRequest("PUT", "calc/inverse_kinematics", &body, true, &res)
	if err != nil {
		return nil, err
	}

	if res.Ik.Result != "success" {
		return nil, fmt.Errorf("inverse_kinematics failure %v to %v", res.Ik, k)
	}

	return res.Ik.Joints, nil
}

func (e *eva) apiLock() error {
	return e.apiRequest("POST", "controls/lock", nil, true, nil)
}

func (e *eva) apiUnlock() {
	err := e.apiRequest("DELETE", "controls/lock", nil, true, nil)
	if err != nil {
		golog.Global.Debugf("eva unlock failed: %s", err)
	}
}

func NewEva(host string, attrs map[string]string) (Arm, error) {
	e := &eva{
		host:    host,
		version: "v1",
		token:   attrs["token"],
	}

	name, err := e.apiName()
	if err != nil {
		return nil, err
	}

	golog.Global.Debugf("connected to eva: %v", name)

	if true {
		_, err := e.CurrentPosition()
		if err != nil {
			return nil, err
		}
	}

	return e, nil
}
