package main

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"strconv"

	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rlog"
)

var shouldPrintFace bool

// CaricaturePoint is a type representing a point on a caricature picture.
// This point has a location int index which gives useful information
// about the facial landmark that it is part of. This point also has both
// an XCoord and YCoord which are absolutes.
type CaricaturePoint struct {
	Location int     `json:"loc"`
	XCoord   float64 `json:"x"`
	YCoord   float64 `json:"y"`
}

// CaricatureFeature is a type representing a facial feature in a
// caricature picture. It contains a Name (to reference facial feature)
// and a list of CaricaturePoint types about the facial feature itself.
type CaricatureFeature struct {
	Name   string            `json:"name"`
	Points []CaricaturePoint `json:"points"`
}

// Face is a type representing a collection of facial features, which,
// in its entirety, describes a face using its facial landmarks.
type Face struct {
	Features []CaricatureFeature `json:"facial_features"`
}

// parseJSON returns a face from the provided json file, and if not, returns
// an error.
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
// into the console.
func printFace(face Face) {
	for i := 0; i < len(face.Features); i++ {
		rlog.Logger.Info("\nFacial Feature: " + face.Features[i].Name)
		numPoints := len(face.Features[i].Points)
		for j := 0; j < numPoints; j++ {
			rlog.Logger.Info("Location: " + strconv.Itoa(
				face.Features[i].Points[j].Location))
			rlog.Logger.Info("XCoord: " + strconv.FormatFloat(
				face.Features[i].Points[j].XCoord, 'f', 6, 64))
			rlog.Logger.Info("YCoord: " + strconv.FormatFloat(
				face.Features[i].Points[j].YCoord, 'f', 6, 64))
		}
	}
}

// facialFeaturePointsFromFace returns a tuple of slices, xdata & ydata, which hold absolute coordinates of facial landmarks.
func facialFeaturePointsFromFace(face Face, featureByInt int) ([]float64,
	[]float64) {
	if shouldPrintFace {
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

// findFace calls python shell script in bash which finds a face using
// the machine's built in camera and exports a JSON file containing
// that person's facial landmarks.
func findFace(person string) error {
	// comment out to override issue finding path
	pretrainedNeuralNetPath := artifact.MustPath("samples/caricatures/face_landmarks.pth")
	haarCascadeFrontalFacePath := artifact.MustPath("samples/caricatures/haarcascade_frontalface_default.xml")

	// uncomment to override issue finding path
	// pretrainedNeuralNetPath := "../trained_neural_net_weights/face_landmarks.pth"
	// haarCascadeFrontalFacePath := "../filters/haarcascade_frontalface_default.xml"

	// set up shell script with correct arguments
	shellPath := "../run.sh"

	// start the shell script from the terminal
	cmd := exec.Command(shellPath, pretrainedNeuralNetPath, haarCascadeFrontalFacePath, person)
	out, err := cmd.Output()
	if err != nil {
		rlog.Logger.Errorw("error finding faces", "output", string(out))
		return err
	}
	return nil
}

// createPlots creates and plots all caricature curves of a person's face.
func createPlots(person string) error {
	err := polyPlotAllCurves(person)
	return err
}
