package motionplan

// import (
// 	frame "go.viam.com/rdk/referenceframe"
// 	"go.viam.com/rdk/utils"
// )

// func exportMaps(m1, m2 map[*node]*node) {
// 	outputList := make([][]frame.Input, 0)
// 	for key, value := range m1 {
// 		if value != nil {
// 			outputList = append(outputList, key.q, value.q)
// 		}
// 	}
// 	for key, value := range m2 {
// 		if value != nil {
// 			outputList = append(outputList, key.q, value.q)
// 		}
// 	}
// 	writeJSONFile(utils.ResolveFile("motionplan/tree.test"), [][][]frame.Input{outputList})
// }
