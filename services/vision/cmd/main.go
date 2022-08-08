package main

import (
	"fmt"
	"log"

	"github.com/mattn/go-tflite"
	"github.com/nfnt/resize"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func main() {

	model := tflite.NewModelFromFile("data/mobilenetv2_imagenet.tflite")
	if model == nil {
		log.Fatal("cannot load model")
	}
	defer model.Delete()

	options := tflite.NewInterpreterOptions()
	defer options.Delete()

	interpreter := tflite.NewInterpreter(model, options)
	defer interpreter.Delete()

	PrintInfo(interpreter)
	interpreter.AllocateTensors()
	input := interpreter.GetInputTensor(0)

	// Get the image, resize it, and then feed it thru
	img, err := rimage.NewImageFromFile("data/lion.jpeg")
	if err != nil {
		fmt.Println(err)
	}

	newimg := resize.Resize(uint(input.Shape()[1]), uint(input.Shape()[2]), img, resize.Bicubic)

	status := input.CopyFromBuffer(vision.ImageToUInt8Buffer(newimg))
	if status != tflite.OK {
		panic("could not copy image")
	}
	interpreter.Invoke()
	output := interpreter.GetOutputTensor(0).UInt8s()
	fmt.Println(output)
	GetMax(output)

	fmt.Printf("Kangaroo?: 104=%v\n", output[104])

}

func PrintInfo(interpreter *tflite.Interpreter) {
	fmt.Printf("Num Input tensors: %v\n", interpreter.GetInputTensorCount())
	fmt.Printf("Num Output tensors: %v\n", interpreter.GetOutputTensorCount())
	fmt.Println()
	fmt.Println("Input info:")
	input := interpreter.GetInputTensor(0)
	fmt.Printf("The name is: %v\n", input.Name())
	fmt.Printf("The shape is: %v\n", input.Shape())
	fmt.Printf("The type is: %v\n", input.Type())
	fmt.Println()
	fmt.Println("Output info:")
	output := interpreter.GetOutputTensor(0)
	fmt.Printf("The name is: %v\n", output.Name())
	fmt.Printf("The shape is: %v\n", output.Shape())
	fmt.Printf("The type is: %v\n", output.Type())
}

func GetMax(probs []uint8) {
	var maxIndex int
	var maxNum uint8
	for i, p := range probs {
		if p > maxNum {
			maxNum = p
			maxIndex = i
		}
	}
	fmt.Printf("Max index was at %v\n", maxIndex)
	fmt.Printf("Max value was at %v\n", maxNum)
}

func GetFloatMax(probs []float32) {
	var maxIndex int
	var maxNum float32
	for i, p := range probs {
		if p > maxNum {
			maxNum = p
			maxIndex = i
		}
	}
	fmt.Printf("Max index was at %v\n", maxIndex)
	fmt.Printf("Max value was %v\n", maxNum)
}
