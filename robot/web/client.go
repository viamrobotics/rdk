package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"go.viam.com/robotcore/api"
)

type Arm struct {
	client *Client
	name   string
}

type MoveToPositionRequest struct {
	Name string
	To   api.ArmPosition
}

func (a *Arm) MoveToPosition(to api.ArmPosition) error {
	return a.client.Do("/api/arm/MoveToPosition", MoveToPositionRequest{a.name, to})
}

type MoveToJointPositionsRequest struct {
	Name string
	To   api.JointPositions
}

func (a *Arm) MoveToJointPositions(to api.JointPositions) error {
	return a.client.Do("/api/arm/MoveToJointPositions", MoveToJointPositionsRequest{a.name, to})
}

type Client struct {
	root string // e.g. http://localhost:8080
}

func (c *Client) ArmByName(name string) *Arm {
	return &Arm{c, name}
}

func (c *Client) Status() (api.Status, error) {
	s := api.Status{}

	res, err := http.Get(c.root + "/api/status")
	if err != nil {
		return s, err
	}
	defer res.Body.Close()
	d := json.NewDecoder(res.Body)
	err = d.Decode(&s)
	return s, err
}

func (c *Client) Do(path string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.root+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("api call failed with error code %d", resp.StatusCode)
	}

	return nil
}
