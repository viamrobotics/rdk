// package is data inference
package main

import "C"
import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	_ "path/filepath"
	"regexp"
	"sort"
	"time"

	tflite "github.com/mattn/go-tflite"
	"github.com/nfnt/resize"
)

var interpreter *tflite.Interpreter
var labels []string
var totalTime time.Duration
var count int
var model_path string

var (
	modelType = flag.String("model", "classification", "type of model to output")
	limits    = flag.Int("limits", 5, "limits of items")
)

// change this to main if this block of code wants to be tested
func something() {
	count = 0
	currentDirectory, err := os.Getwd()
	if err != nil {
		// fmt.Println("error loading labels")
		log.Fatal(err)
	}
	//fmt.Println(currentDirectory)

	var label_path = currentDirectory + "/testing/labelmap.txt"
	model_path = currentDirectory + "/testing/model_with_metadata.tflite"
	// var label_path = "/Users/alexiswei/Documents/repos/rdk/testing/labels.txt"
	// var model_path = "/Users/alexiswei/Documents/repos/rdk/testing/mobilenet_v2_1.0_224_quant.tflite"
	// var image_path = "/Users/alexiswei/Documents/repos/rdk/testing/peacock.png"
	// var label_path = currentDirectory + "/testing/labels.txt"
	// model_path = currentDirectory + "/testing/mobilenet_v2_1.0_224_quant.tflite"
	// var image_path = currentDirectory + "/testing/images/peacock.png"

	labels, err = loadLabels(label_path)
	if err != nil {
		// fmt.Println("error loading labels")
		log.Fatal(err)
	}

	// model := tflite.NewModelFromFile(model_path)
	// if model == nil {
	// 	log.Fatal("cannot load model")
	// }
	// defer model.Delete()

	// options := tflite.NewInterpreterOptions()
	// options.SetNumThread(4)
	// options.SetErrorReporter(func(msg string, user_data interface{}) {
	// 	fmt.Println(msg)
	// }, nil)
	// defer options.Delete()

	// interpreter = tflite.NewInterpreter(model, options)
	// if interpreter == nil {
	// 	log.Println("cannot create interpreter")
	// 	return
	// }
	// defer interpreter.Delete()

	// interpretModel(model_path)
	interpreter = GetTfliteInterpreter(model_path, 4)
	defer interpreter.Delete()

	filepath.WalkDir(currentDirectory+"/testing/images", testImage)
	// fmt.Printf("average time (ms): %f \n", float64(totalTime.Milliseconds())/float64(count))
}

func testImage(path string, d fs.DirEntry, err error) error {
	//fmt.Println(path)
	re := regexp.MustCompile(`\/([\w\.]*)$`)
	matches := re.FindAllString(path, 1)
	if len(matches) != 0 && matches[0] == "/images" {
		return nil
	}
	imgFile, err := os.Open(path)
	//fmt.Println("opening image")
	if err != nil {
		// fmt.Println("error opening image")
		log.Fatal(err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	//fmt.Println("decoding image")
	if err != nil {
		// fmt.Println("error decoding image")
		log.Fatal(err)
	}

	// return runClassification(interpreter, img, labels)
	return getInference(interpreter, img, labels)
}

func loadLabels(filename string) ([]string, error) {
	labels := []string{}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		labels = append(labels, scanner.Text())
	}
	return labels, nil
}

type modelProperties struct {
	model           *tflite.Model
	interpreter     *tflite.Interpreter
	wanted_type     tflite.TensorType
	wanted_height   int
	wanted_width    int
	wanted_channels int
	status          tflite.Status
	options         *tflite.InterpreterOptions
}

func GetTfliteInterpreter(modelPath string, numThreads int) *tflite.Interpreter {
	model := tflite.NewModelFromFile(modelPath)
	if model == nil {
		log.Fatal("cannot load model")
	}
	//defer model.Delete()

	options := tflite.NewInterpreterOptions()
	if numThreads == 0 {
		options.SetNumThread(4)
	} else {
		options.SetNumThread(numThreads)
	}

	options.SetErrorReporter(func(msg string, user_data interface{}) {
		errors.New(msg)
	}, nil)
	// defer options.Delete()

	interpreter := tflite.NewInterpreter(model, options)
	if interpreter == nil {
		log.Println("cannot create interpreter")
		return nil
	}
	//defer interpreter.Delete()

	return interpreter
}

func interpretModel(model_path string) (*modelProperties, error) {
	model := tflite.NewModelFromFile(model_path)
	if model == nil {
		errors.New("cannot load model")
	}
	//defer model.Delete()

	options := tflite.NewInterpreterOptions()
	options.SetNumThread(4)
	options.SetErrorReporter(func(msg string, user_data interface{}) {
		errors.New(msg)
	}, nil)
	//defer options.Delete()

	interpreter = tflite.NewInterpreter(model, options)
	if interpreter == nil {
		// log.Println("cannot create interpreter")
		return nil, errors.New("cannot create interpreter")
	}
	//defer interpreter.Delete()

	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		// log.Println("allocate failed")
		return nil, errors.New("allocate failed")
	}
	input := interpreter.GetInputTensor(0)

	props := &modelProperties{
		model:           model,
		interpreter:     interpreter,
		wanted_type:     input.Type(),
		wanted_height:   input.Dim(1),
		wanted_width:    input.Dim(2),
		wanted_channels: input.Dim(3),
		status:          status,
		options:         options,
	}

	return props, nil
}

type ssdResult struct {
	loc   []float32
	clazz []float32
	score []float32
	mat   image.Image
}

type ssdClass struct {
	loc   []float32
	score float64
	index int
}

type result interface {
	Image() image.Image
}

// func runObjDetection(interpreter *tflite.Interpreter, img image.Image, labels []string) error {
// 	startTime := time.Now()

// 	status := interpreter.AllocateTensors()
// 	if status != tflite.OK {
// 		log.Println("allocate failed")
// 		return nil
// 	}

// 	input := interpreter.GetInputTensor(0)

// 	// defer interpreter.Delete()
// 	wanted_height := input.Dim(1)
// 	wanted_width := input.Dim(2)

// 	// qp := input.QuantizationParams()
// 	//log.Printf("width: %v, height: %v, type: %v, scale: %v, zeropoint: %v", wanted_width, wanted_height, input.Type(), qp.Scale, qp.ZeroPoint)
// 	log.Printf("width: %v, height: %v, type: %v", wanted_width, wanted_height, input.Type())
// 	log.Printf("input tensor count: %v, output tensor count: %v", interpreter.GetInputTensorCount(), interpreter.GetOutputTensorCount())
// 	// if qp.Scale == 0 {
// 	// 	qp.Scale = 1
// 	// }

// 	fmt.Println(interpreter.GetOutputTensor(0).Float32s())
// 	fmt.Println(interpreter.GetOutputTensor(1).Float32s())
// 	fmt.Println(interpreter.GetOutputTensor(2).Float32s())

// 	// frame := gocv.NewMat()
// 	// resized := gocv.NewMat()
// 	// if input.Type() == tflite.Float32 {
// 	// 	frame.ConvertTo(&resized, gocv.MatTypeCV32F)
// 	// 	gocv.Resize(resized, &resized, image.Pt(wanted_width, wanted_height), 0, 0, gocv.InterpolationDefault)
// 	// 	if ff, err := resized.DataPtrFloat32(); err == nil {
// 	// 		copy(input.Float32s(), ff)
// 	// 	}
// 	// } else {
// 	// 	gocv.Resize(frame, &resized, image.Pt(wanted_width, wanted_height), 0, 0, gocv.InterpolationDefault)
// 	// 	if v, err := resized.DataPtrUint8(); err == nil {
// 	// 		copy(input.UInt8s(), v)
// 	// 	}
// 	// }

// 	resized := gocv.NewMat()
// 	anotherOne := gocv.NewMat()
// 	fmt.Printf("cols: %v", img.Bounds().Size())
// 	resizeImg := resize.Resize(uint(wanted_width), uint(wanted_height), img, resize.NearestNeighbor)
// 	bounds := resizeImg.Bounds()
// 	buf := new(bytes.Buffer)
// 	err := png.Encode(buf, resizeImg)
// 	if err != nil {
// 		fmt.Println("failed to create buffer", err)
// 	}

// 	dx, dy := bounds.Dx(), bounds.Dy()

// 	if input.Type() == tflite.Float32 {
// 		resized, err = gocv.NewMatFromBytes(dx, dy, gocv.MatTypeCV32F, buf.Bytes())
// 		if err != nil {
// 			fmt.Println("failed to create newMatFromBytes", err)
// 		}
// 		gocv.Resize(resized, &anotherOne, image.Pt(wanted_width, wanted_height), 0, 0, gocv.InterpolationDefault)

// 		fmt.Println("what's in anotherOne:")
// 		fmt.Println(anotherOne)

// 		if ff, err := resized.DataPtrFloat32(); err == nil {
// 			fmt.Println("copying data")
// 			fmt.Println(ff)
// 			copy(input.Float32s(), ff)
// 		}
// 	} else {
// 		resized, err = gocv.NewMatFromBytes(dx, dy, gocv.MatTypeCV8U, buf.Bytes())
// 		if err != nil {
// 			fmt.Println("failed to create newMatFromBytes", err)
// 		}
// 		gocv.Resize(resized, &anotherOne, image.Pt(wanted_width, wanted_height), 0, 0, gocv.InterpolationDefault)

// 		if v, err := resized.DataPtrUint8(); err == nil {
// 			copy(input.UInt8s(), v)
// 		}
// 		fmt.Printf("type: %v \n", input.Type())
// 	}

// 	fmt.Printf("resized size: %v, another size: %v", resized.Size(), anotherOne.Size())
// 	resized.Close()
// 	status = interpreter.Invoke()
// 	if status != tflite.OK {
// 		log.Println("invoke failed")
// 		return errors.New("invoke failed")
// 	}

// 	result := &ssdResult{
// 		loc:   copySlice(interpreter.GetOutputTensor(0).Float32s()),
// 		clazz: copySlice(interpreter.GetOutputTensor(1).Float32s()),
// 		score: copySlice(interpreter.GetOutputTensor(2).Float32s()),
// 		mat:   img,
// 	}

// 	fmt.Println(result.loc)
// 	fmt.Println(result.clazz)
// 	fmt.Println(result.score)

// 	classes := make([]ssdClass, 0, len(result.clazz))
// 	var i int
// 	for i = 0; i < len(result.clazz); i++ {
// 		// fmt.Printf("in loop: %v \n", i)
// 		idx := int(result.clazz[i] + 1)
// 		score := float64(result.score[i]) / 225.0
// 		if score < 0.2 {
// 			continue
// 		}
// 		classes = append(classes, ssdClass{loc: result.loc[i*4 : (i+1)*4], score: score, index: idx})
// 	}
// 	sort.Slice(classes, func(i, j int) bool {
// 		return classes[i].score > classes[j].score
// 	})

// 	if len(classes) > *limits {
// 		classes = classes[:*limits]
// 	}

// 	imgDx, imgDy := img.Bounds().Dx(), img.Bounds().Dy()

// 	size := []int{imgDx, imgDy}
// 	fmt.Println(size)
// 	fmt.Println(classes)
// 	for i, class := range classes {
// 		label := "unknown"
// 		if class.index < len(labels) {
// 			label = labels[class.index]
// 		}
// 		// c := colornames.Map[colornames.Names[class.index%len(colornames.Names)]]
// 		// gocv.Rectangle(&result.mat, image.Rect(
// 		// 	int(float32(size[1])*class.loc[1]),
// 		// 	int(float32(size[0])*class.loc[0]),
// 		// 	int(float32(size[1])*class.loc[3]),
// 		// 	int(float32(size[0])*class.loc[2]),
// 		// ), c, 2)
// 		text := fmt.Sprintf("%d %.5f %s", i, class.score, label)
// 		fmt.Println(text)
// 		// gocv.PutText(&result.mat, text, image.Pt(
// 		// 	int(float32(size[1])*class.loc[1]),
// 		// 	int(float32(size[0])*class.loc[0]),
// 		// ), gocv.FontHersheySimplex, 1.2, c, 1)
// 	}

// 	timeDiff := time.Now().Sub(startTime)
// 	fmt.Println(timeDiff)
// 	count++

// 	totalTime += timeDiff

// 	return nil
// }

func copySlice(f []float32) []float32 {
	ff := make([]float32, len(f), len(f))
	copy(ff, f)
	return ff
}

func runClassification(interpreter *tflite.Interpreter, img image.Image, labels []string) error {
	startTime := time.Now()

	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		log.Println("allocate failed")
		return errors.New("allocate failed")
	}
	input := interpreter.GetInputTensor(0)

	// defer interpreter.Delete()
	wanted_height := input.Dim(1)
	wanted_width := input.Dim(2)
	wanted_channels := input.Dim(3)
	wanted_type := input.Type()
	//fmt.Printf("wanted type: %v, wanted channels: %v, wanted width: %v, wanted height: %v \n", wanted_type, wanted_channels, wanted_width, wanted_height)

	resized := resize.Resize(uint(wanted_width), uint(wanted_height), img, resize.NearestNeighbor)
	bounds := resized.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()

	if wanted_type == tflite.UInt8 {
		bb := make([]byte, dx*dy*wanted_channels)
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				col := resized.At(x, y)
				r, g, b, _ := col.RGBA()
				bb[(y*dx+x)*3+0] = byte(float64(r) / 255.0)
				bb[(y*dx+x)*3+1] = byte(float64(g) / 255.0)
				bb[(y*dx+x)*3+2] = byte(float64(b) / 255.0)
			}
		}
		input.CopyFromBuffer(bb)
	} else if wanted_type == tflite.Float32 {
		//fmt.Println("is float32")
		bb := make([]float32, dx*dy*wanted_channels)
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				col := resized.At(x, y)
				r, g, b, _ := col.RGBA()
				bb[(y*dx+x)*3+0] = float32(r) / 255.0
				bb[(y*dx+x)*3+1] = float32(g) / 255.0
				bb[(y*dx+x)*3+2] = float32(b) / 255.0
			}
		}
	} else {
		log.Println("is not wanted type")
		return errors.New("is not wanted type")
	}

	status = interpreter.Invoke()
	if status != tflite.OK {
		log.Println("invoke failed")
		return errors.New("invoke failed")
	}

	output := interpreter.GetOutputTensor(0)
	output_size := output.Dim(output.NumDims() - 1)
	//fmt.Printf("output size: %v \n", output_size)
	b := make([]byte, output_size)
	type result struct {
		score float64
		index int
	}
	status = output.CopyToBuffer(&b[0])
	if status != tflite.OK {
		log.Println("output failed")
		return errors.New("output failed")
	}
	results := []result{}
	for i := 0; i < output_size; i++ {
		score := float64(b[i]) / 255.0
		if score < 0.2 {
			continue
		}
		results = append(results, result{score: score, index: i})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	for i := 0; i < len(results); i++ {
		// fmt.Printf("%02d: %s: %f\n", results[i].index, labels[results[i].index], results[i].score)
		if i > 5 {
			break
		}
	}
	timeDiff := time.Now().Sub(startTime)
	// fmt.Println(timeDiff)
	count++

	totalTime += timeDiff
	return nil
}

func getInference(interpreter *tflite.Interpreter, img image.Image, labels []string) error {
	startTime := time.Now()

	status := interpreter.AllocateTensors()
	if status != tflite.OK {
		log.Println("allocate failed")
		return errors.New("allocate failed")
	}
	input := interpreter.GetInputTensor(0)

	wantedHeight := input.Dim(1)
	wantedWidth := input.Dim(2)
	wantedChannels := input.Dim(3)
	wantedType := input.Type()

	resized := resize.Resize(uint(wantedWidth), uint(wantedHeight), img, resize.NearestNeighbor)
	bounds := resized.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()

	numOutputTensors := interpreter.GetOutputTensorCount()

	if wantedType == tflite.UInt8 {
		bb := make([]byte, dx*dy*wantedChannels)
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				col := resized.At(x, y)
				r, g, b, _ := col.RGBA()
				bb[(y*dx+x)*3+0] = byte(float64(r) / 255.0)
				bb[(y*dx+x)*3+1] = byte(float64(g) / 255.0)
				bb[(y*dx+x)*3+2] = byte(float64(b) / 255.0)
			}
		}
		input.CopyFromBuffer(bb)
	} else if wantedType == tflite.Float32 {
		//fmt.Println("is float32")
		bb := make([]float32, dx*dy*wantedChannels)
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				col := resized.At(x, y)
				r, g, b, _ := col.RGBA()
				bb[(y*dx+x)*3+0] = float32(r) / 255.0
				bb[(y*dx+x)*3+1] = float32(g) / 255.0
				bb[(y*dx+x)*3+2] = float32(b) / 255.0
			}
		}
		input.CopyFromBuffer(bb)
	} else {
		log.Println("is not wanted type")
		return errors.New("is not wanted type")
	}

	status = interpreter.Invoke()
	if status != tflite.OK {
		log.Println("invoke failed")
		return errors.New("invoke failed")
	}

	var output []*tflite.Tensor
	for i := 0; i < numOutputTensors; i++ {
		output = append(output, interpreter.GetOutputTensor(i))
	}

	//fmt.Println(output)

	output_size := output[0].Dim(output[0].NumDims() - 1)
	//fmt.Printf("output size: %v \n", output_size)

	b := make([]byte, output_size)
	if numOutputTensors == 1 {
		// classification
		// fmt.Println("check output")
		// fmt.Println(output[0].Float32s())
		type result struct {
			score float64
			index int
		}
		status = output[0].CopyToBuffer(&b[0])
		if status != tflite.OK {
			log.Println("output failed")
			return errors.New("output failed")
		}
		results := []result{}
		for i := 0; i < output_size; i++ {
			score := float64(b[i]) / 255.0
			if score < 0.2 {
				continue
			}
			results = append(results, result{score: score, index: i})
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].score > results[j].score
		})
		for i := 0; i < len(results); i++ {
			// fmt.Printf("%02d: %s: %f\n", results[i].index, labels[results[i].index], results[i].score)
			if i > 5 {
				break
			}
		}
	} else if numOutputTensors == 4 {
		// object detection
		location := output[1].Float32s()
		//location1 := output[0]
		indices := output[3].Float32s()
		scores := output[0].Float32s()
		// fmt.Printf("loc len: %v, class len: %v, score len: %v \n", len(location), len(indices), len(scores))
		log.Println(location[0])
		// fmt.Println(indices)
		// fmt.Println(scores)
		for i, index := range indices {
			label := "unknown"
			if int(index) < len(labels) {
				label = labels[int(index)]
			}

			text := fmt.Sprintf("%d %.5f %s", i, scores[i], label)
			log.Println(string(text[0]))
		}
	}

	timeDiff := time.Now().Sub(startTime)
	//fmt.Println(timeDiff)
	count++

	totalTime += timeDiff
	return nil
}
