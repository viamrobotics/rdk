package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/edaniels/golog"
	"go.viam.com/core/robot"
)

const (
	NUM_FACIAL_LANDMARKS = 68
)

var logger = golog.NewDevelopmentLogger("armplay-caricature")

type CaricaturePoint struct {
	Location int     `json:"loc"`
	XCoord   float64 `json:"x"`
	YCoord   float64 `json:"y"`
}

type CaricatureFeature struct {
	Name   string            `json:"name"`
	Points []CaricaturePoint `json:"points"`
}

type Face struct {
	Features []CaricatureFeature `json:"facial_features"`
}

func parseJSON(path string) error {
	var face Face
	byteSequence, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	json.Unmarshal(byteSequence, &face)
	for i := 0; i < len(face.Features); i++ {
		fmt.Println("Facial Feature: " + face.Features[i].Name)
		num_points := len(face.Features[i].Points)
		for j := 0; j < num_points; j++ {
			fmt.Println("Location: " + strconv.Itoa(face.Features[i].Points[j].Location))
			fmt.Println("XCoord: " + strconv.FormatFloat(face.Features[i].Points[j].XCoord, 'f', 6, 64))
			fmt.Println("YCoord: " + strconv.FormatFloat(face.Features[i].Points[j].YCoord, 'f', 6, 64))
		}
	}
	return nil
}

func drawPoint(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}
	arm := r.ArmByName(r.ArmNames()[0])

	for i := 0; i < NUM_FACIAL_LANDMARKS; i++ {
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		arm.MoveToPosition(ctx, pos)
	}
	return nil
}

func main() {
	// action.RegisterAction("drawPoint", func(ctx context.Context, r robot.Robot) {
	// 	err := drawPoint(ctx, r)
	// 	if err != nil {
	// 		logger.Errorf("error: %s", err)
	// 	}
	// })
	parseJSON("selfie.json")
}
