package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
)

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

const (
	JSON_PATH string = "../json/selfie.json"
)

func parseJSON(path string) (Face, error) {
	var face Face
	byteSequence, err := ioutil.ReadFile(path)
	if err != nil {
		return face, err
	}
	json.Unmarshal(byteSequence, &face)
	return face, nil
}

func printFace(face Face) {
	for i := 0; i < len(face.Features); i++ {
		fmt.Println("\nFacial Feature: " + face.Features[i].Name)
		num_points := len(face.Features[i].Points)
		for j := 0; j < num_points; j++ {
			fmt.Println("Location: " + strconv.Itoa(face.Features[i].Points[j].Location))
			fmt.Println("XCoord: " + strconv.FormatFloat(face.Features[i].Points[j].XCoord, 'f', 6, 64))
			fmt.Println("YCoord: " + strconv.FormatFloat(face.Features[i].Points[j].YCoord, 'f', 6, 64))
		}
	}
}

func facialFeaturePointsFromFace(face Face, feature_by_int int) ([]float64, []float64) {
	var xdata []float64
	var ydata []float64
	for fcp := 0; fcp < len(face.Features[feature_by_int].Points); fcp++ {
		xdata = append(xdata, face.Features[feature_by_int].Points[fcp].XCoord)
		ydata = append(ydata, face.Features[feature_by_int].Points[fcp].YCoord)
	}
	return xdata, ydata
}

func createPlotsAndRegressions() error {
	err := polyPlotAllCurves()
	return err
}
