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

type colorTrainingStore struct {
	data string
}

func (s *colorTrainingStore) init() {
	s.data = "edges,r,g,b,type\n"
}

func (s *colorTrainingStore) add(edges int, c color.RGBA, what string) {
	s.data = s.data + fmt.Sprintf("%d, %d, %d, %d, %s\n", edges, c.R, c.G, c.B, what)
}

func pieceFromColor(theClassifier base.Classifier, edges int, data color.RGBA) string {
	csvData := colorTrainingStore{}
	csvData.init()
	csvData.add(0, color.RGBA{0, 0, 0, 0}, "white")
	csvData.add(0, color.RGBA{0, 0, 0, 0}, "empty")
	csvData.add(0, color.RGBA{0, 0, 0, 0}, "black")
	csvData.add(edges, data, "white")

	rawData, err := base.ParseCSVToInstancesFromReader(strings.NewReader(csvData.data), true)
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

func buildPieceColorModel(theBoard *Board) (base.Classifier, error) {
	csvData := colorTrainingStore{}
	csvData.init()

	for y := '1'; y <= '8'; y++ {
		for x := 'a'; x <= 'h'; x++ {
			square := string(x) + string(y)

			corner := getMinChessCorner(square)
			middle := image.Point{corner.X + 50, corner.Y + 50}

			radius := 3

			squareType := "empty"
			if square[1] == '1' || square[1] == '2' {
				squareType = "white"
			}
			if square[1] == '7' || square[1] == '8' {
				squareType = "black"
			}

			edges := theBoard.SquareCenterEdges(square)

			for x := middle.X - radius; x < middle.X+radius; x++ {
				for y := middle.Y - radius; y < middle.Y+radius; y++ {
					data := vision.GetColor(theBoard.color, y, x)

					csvData.add(edges, data, squareType)
				}
			}
		}
	}

	rawData, err := base.ParseCSVToInstancesFromReader(strings.NewReader(csvData.data), true)
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
