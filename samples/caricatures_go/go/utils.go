package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
)

// CaricaturePoint is a type representing a point on a caricature picture.
// This point has a location int index which gives useful information
// about the facial landmark that it is part of. This point also has both
// an XCoord and YCoord (which describe absolute X,Y location of point in
// the picture frame)
type CaricaturePoint struct {
	Location int     `json:"loc"`
	XCoord   float64 `json:"x"`
	YCoord   float64 `json:"y"`
}

// CaricatureFeature is a type representing a facial feature in a
// caricature picture. It contains a Name (to reference facial feature)
// and a list of CaricaturePoint types (which provide useful information)
// about the facial feature itself.
type CaricatureFeature struct {
	Name   string            `json:"name"`
	Points []CaricaturePoint `json:"points"`
}

// Face is a type representing a collection of facial features, which,
// in its entirety, describes a face using its facial landmarks.
type Face struct {
	Features []CaricatureFeature `json:"facial_features"`
}

// constants
const (
	jsonPath string = "../json/selfie.json"
)

// parseJSON returns a face from the provided json file, and if not, returns
// an error
func parseJSON(path string) (Face, error) {
	var face Face
	byteSequence, err := ioutil.ReadFile(path)
	if err != nil {
		return face, err
	}
	json.Unmarshal(byteSequence, &face)
	return face, nil
}

// printFace prints information about a face (broken down by facial feature)
// into the console
func printFace(face Face) {
	for i := 0; i < len(face.Features); i++ {
		fmt.Println("\nFacial Feature: " + face.Features[i].Name)
		numPoints := len(face.Features[i].Points)
		for j := 0; j < numPoints; j++ {
			fmt.Println("Location: " + strconv.Itoa(
				face.Features[i].Points[j].Location))
			fmt.Println("XCoord: " + strconv.FormatFloat(
				face.Features[i].Points[j].XCoord, 'f', 6, 64))
			fmt.Println("YCoord: " + strconv.FormatFloat(
				face.Features[i].Points[j].YCoord, 'f', 6, 64))
		}
	}
}

// facialFeaturePointsFromFace returns a tuple of slices, xdata & ydata,
// which represent
func facialFeaturePointsFromFace(face Face, featureByInt int) ([]float64,
	[]float64) {
	// IF FALSE MAKES SURE BELOW CODE PASSES LINTER BECAUSE IT IS UNUSED
	if false {
		printFace(face)
	}
	var xdata []float64
	var ydata []float64
	for fcp := 0; fcp < len(face.Features[featureByInt].Points); fcp++ {
		xdata = append(xdata, face.Features[featureByInt].Points[fcp].XCoord)
		ydata = append(ydata, face.Features[featureByInt].Points[fcp].YCoord)
	}
	return xdata, ydata
}

// createPlots creates and plots all caricature curves of a person's face
func createPlots() error {
	err := polyPlotAllCurves()
	return err
}
