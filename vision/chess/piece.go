package chess

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/evaluation"
	"github.com/sjwhitworth/golearn/knn"
)

type colorTrainingStore struct {
	data string
}

func (s *colorTrainingStore) init() {
	s.data = "edges,r,g,b,type\n"
}

func (s *colorTrainingStore) add(edges int, c color.RGBA, what string) {
	whatValue := 0
	switch what {
	case "white":
		whatValue = 0
	case "black":
		whatValue = 1
	case "empty":
		whatValue = 2
	default:
		panic(fmt.Errorf("unknown type: %s", what))
	}
	s.data = s.data + fmt.Sprintf("%d, %d, %d, %d, %d\n", edges, c.R, c.G, c.B, whatValue)
}

func pieceFromColor(theClassifier base.Classifier, edges int, data color.RGBA) string {
	csvData := colorTrainingStore{}
	csvData.init()
	csvData.add(edges, data, "white")

	rawData, err := base.ParseCSVToInstancesFromReader(strings.NewReader(csvData.data), true)
	if err != nil {
		panic(err)
	}

	res, err := theClassifier.Predict(rawData)
	if err != nil {
		panic(err)
	}

	attrs := res.AllAttributes()
	if len(attrs) != 1 {
		panic("this sucks")
	}
	spec, err := res.GetAttribute(attrs[0])
	if err != nil {
		panic(err)
	}

	raw := res.Get(spec, 0)
	if len(raw) != 8 {
		panic("wtf")
	}

	whatValue := int(base.UnpackBytesToFloat(raw))
	switch whatValue {
	case 0:
		return "white"
	case 1:
		return "black"
	case 2:
		return "empty"
	default:
		panic(fmt.Errorf("unknown what # %d", whatValue))
	}
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
					data := theBoard.color.ColorRowCol(y, x)

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
