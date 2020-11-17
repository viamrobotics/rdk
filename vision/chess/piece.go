package chess

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/evaluation"
	"github.com/sjwhitworth/golearn/knn"

	"github.com/echolabsinc/robotcore/vision"
)

func (b *Board) Piece(theClassifier base.Classifier, square string) string {

	corner := getMinChessCorner(square)
	middle := image.Point{corner.X + 50, corner.Y + 50}

	data := _avgColor(b.color, middle.X, middle.Y)
	return PieceFromColor(theClassifier, data)
}

func PieceFromColor(theClassifier base.Classifier, data color.RGBA) string {
	csvData := fmt.Sprintf("r,g,b,type\n"+
		"0,0,0,white\n"+
		"0,0,0,empty\n"+
		"0,0,0,black\n"+
		"%d, %d, %d, white\n", data.R, data.G, data.B)

	rawData, err := base.ParseCSVToInstancesFromReader(strings.NewReader(csvData), true)
	if err != nil {
		panic(err)
	}

	res, err := theClassifier.Predict(rawData)
	if err != nil {
		panic(err)
	}

	if len(res.AllAttributes()) != 1 {
		panic("this sucks")
	}

	return res.RowString(3)
}

func buildPieceModel(theBoard *Board) (base.Classifier, error) {
	csvData := "r,g,b,type\n"

	for y := '1'; y <= '8'; y++ {
		for x := 'a'; x <= 'h'; x++ {
			square := string(x) + string(y)
			if square == "H7" {
				continue
			}

			corner := getMinChessCorner(square)
			middle := image.Point{corner.X + 50, corner.Y + 50}

			radius := 3

			for x := middle.X - radius; x < middle.X+radius; x++ {
				for y := middle.Y - radius; y < middle.Y+radius; y++ {
					data := vision.GetColor(theBoard.color, y, x)

					squareType := "empty"
					if square[1] == '1' || square[1] == '2' {
						squareType = "white"
					}
					if square[1] == '7' || square[1] == '8' {
						squareType = "black"
					}

					csvData += fmt.Sprintf("%d, %d, %d, %s\n", data.R, data.G, data.B, squareType)
				}
			}
		}
	}

	rawData, err := base.ParseCSVToInstancesFromReader(strings.NewReader(csvData), true)
	if err != nil {
		return nil, err
	}

	theClassifier := knn.NewKnnClassifier("euclidean", "linear", 2)

	if true {
		theClassifier.Fit(rawData)
	} else {
		//Do a training-test split
		trainData, testData := base.InstancesTrainTestSplit(rawData, 0.5)
		fmt.Println(testData)

		//Initialises a new KNN classifier

		theClassifier.Fit(trainData)

		//Calculates the Euclidean distance and returns the most popular label
		predictions, err := theClassifier.Predict(testData)
		if err != nil {
			return nil, err
		}

		// Prints precision/recall metrics
		confusionMat, err := evaluation.GetConfusionMatrix(testData, predictions)
		if err != nil {
			return nil, fmt.Errorf("Unable to get confusion matrix: %s", err.Error())
		}
		fmt.Println(evaluation.GetSummary(confusionMat))
	}

	return theClassifier, nil
}
