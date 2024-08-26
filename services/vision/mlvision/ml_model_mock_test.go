package mlvision

import (
	"context"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	"go.viam.com/rdk/testutils/inject"
	"gorgonia.org/tensor"
)

func getTestMlModel(modelLoc string) (mlmodel.Service, error) {
	ctx := context.Background()
	testMLModelServiceName := "test-model"

	name := mlmodel.Named(testMLModelServiceName)
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	return tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
}

func mockEffDetModel(name string, labelLoc string) mlmodel.Service {
	// using the effdet0.tflite model as a template
	// pretend it has taken in the picture of "vision/tflite/dogscute.jpeg"
	effDetMock := inject.NewMLModelService(name)
	md := mlmodel.MLMetadata{
		ModelName:        "EfficientDet Lite0 V1",
		ModelType:        "tflite_detector",
		ModelDescription: "Identify which of a known set of objects might be present and provide information about their positions within the given image or a video stream.",
	}
	inputs := make([]mlmodel.TensorInfo, 0, 1)
	imageIn := mlmodel.TensorInfo{
		Name:        "image",
		Description: "Input image to be detected. The expected image is 320 x 320, with three channels (red, blue, and green) per pixel. Each value in the tensor is a single byte between 0 and 255.",
		DataType:    "uint8",
		Shape:       []int{1, 320, 320, 3},
	}
	inputs = append(inputs, imageIn)
	md.Inputs = inputs
	outputs := make([]mlmodel.TensorInfo, 0, 4)
	locationOut := mlmodel.TensorInfo{
		Name:        "location",
		Description: "The locations of the detected boxes.",
		DataType:    "float32",
	}
	if labelLoc != "" {
		extra := map[string]interface{}{"labels": labelLoc}
		locationOut.Extra = extra
	}
	outputs = append(outputs, locationOut)
	categoryOut := mlmodel.TensorInfo{
		Name:        "category",
		Description: "The categories of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, categoryOut)
	scoreOut := mlmodel.TensorInfo{
		Name:        "score",
		Description: "The scores of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, scoreOut)
	numberOut := mlmodel.TensorInfo{
		Name:        "number of detections",
		Description: "The number of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, numberOut)
	md.Outputs = outputs
	effDetMock.MetadataFunc = func(ctx context.Context) (mlmodel.MLMetadata, error) {
		return md, nil
	}

	// now define the output tensors
	outputInfer := ml.Tensors{}
	//score
	score := []float32{0.81640625, 0.6875, 0.109375, 0.09375, 0.0625,
		0.0546875, 0.05078125, 0.0390625, 0.03515625, 0.03125,
		0.0234375, 0.0234375, 0.0234375, 0.0234375, 0.01953125,
		0.01953125, 0.01953125, 0.01953125, 0.01953125, 0.01953125,
		0.015625, 0.015625, 0.015625, 0.015625, 0.015625}
	scoreTensor := tensor.New(tensor.WithShape(1, 25), tensor.WithBacking(score))
	outputInfer["score"] = scoreTensor
	// nDetections
	nDetections := []float32{25}
	detectionTensor := tensor.New(tensor.WithShape(1), tensor.WithBacking(nDetections))
	outputInfer["number of detections"] = detectionTensor
	// locations
	locations := []float32{
		0.20903039, 0.49185863, 0.82770026, 0.7690754,
		0.2376312, 0.260224, 0.82330287, 0.5374408,
		0.21014652, 0.37334082, 0.82086015, 0.67316914,
		0.9004202, 0.36880112, 0.95539546, 0.41990197,
		0.19502541, 0.1988186, 0.8602221, 0.77355766,
		0.836329, 0.86517155, 0.8984374, 0.99401116,
		0.2503236, 0.2755023, 0.56928396, 0.50930154,
		0.4401425, 0.35509717, 0.53873336, 0.41215116,
		0.22128013, 0.51680136, 0.5461217, 0.7506006,
		0.89365757, 0.6519017, 0.9923049, 0.7121358,
		0.34879953, 0.47103795, 0.45682132, 0.50783795,
		0.83736897, 0.94356436, 0.89037156, 0.98691684,
		0.25913447, 0.12777925, 0.7270005, 0.6214407,
		0.44479424, 0.21759495, 0.81613976, 0.6628721,
		0.38580972, 0.5132986, 0.5085694, 0.5617015,
		0.49028072, 0.00190118, 0.59634674, 0.02697465,
		0.5979702, 0.9293068, 0.7516399, 0.99569315,
		0.8964205, 0.33521998, 0.95665455, 0.4144457,
		0.4158226, 0.2888925, 0.5341774, 0.46885914,
		0.20846531, 0.2381043, 0.50130117, 0.6228298,
		0.38078213, 0.34770778, 0.5372853, 0.4334447,
		0.4441566, 0.45994544, 0.5502226, 0.50924087,
		0.5679829, 0.98425895, 0.76903045, 0.9965547,
		0.6335254, 0.97844476, 0.76085377, 0.9946173,
		0.8215679, 0.07016394, 0.89795077, 0.11853918}
	locationTensor := tensor.New(tensor.WithShape(1, 25, 4), tensor.WithBacking(locations))
	outputInfer["location"] = locationTensor
	// categories
	categories := []float32{17., 17., 17., 36., 17., 87., 17., 33., 17., 36., 33., 87., 17.,
		17., 33., 0., 0., 36., 33., 17., 17., 33., 0., 0., 36.}
	categoryTensor := tensor.New(tensor.WithShape(1, 25), tensor.WithBacking(categories))
	outputInfer["category"] = categoryTensor
	effDetMock.InferFunc = func(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
		return outputInfer, nil
	}
	effDetMock.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	return effDetMock
}

func mockEffNetModel(name string, labelLoc string) mlmodel.Service {
	// using the effnet0.tflite model as a template
	effNetMock := inject.NewMLModelService(name)
	md := mlmodel.MLMetadata{
		ModelName:        "EfficientNet-lite image classifier (quantized)",
		ModelType:        "tflite_classifier",
		ModelDescription: "Identify the most prominent object in the image from a set of 1,000 categories such as trees, animals, food, vehicles, person etc.",
	}
	inputs := make([]mlmodel.TensorInfo, 0, 1)
	imageIn := mlmodel.TensorInfo{
		Name:        "image",
		Description: "Input image to be classified. The expected image is 260 x 260, with three channels (red, blue, and green) per pixel. Each element in the tensor is a value between min and max, where (per-channel) min is [0] and max is [255].",
		DataType:    "uint8",
		Shape:       []int{1, 260, 260, 3},
	}
	inputs = append(inputs, imageIn)
	md.Inputs = inputs
	outputs := make([]mlmodel.TensorInfo, 0, 1)
	probabilityOut := mlmodel.TensorInfo{
		Name:        "probability",
		Description: "Probabilities of the 1000 labels respectively.",
		DataType:    "uint8",
	}
	if labelLoc != "" {
		extra := map[string]interface{}{"labels": labelLoc}
		probabilityOut.Extra = extra
	}
	outputs = append(outputs, probabilityOut)
	md.Outputs = outputs
	effNetMock.MetadataFunc = func(ctx context.Context) (mlmodel.MLMetadata, error) {
		return md, nil
	}

	// now define the output tensors
	outputInfer := ml.Tensors{}
	//probability
	prob := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 224, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	probTensor := tensor.New(tensor.WithShape(1, 1000), tensor.WithBacking(prob))
	outputInfer["probability"] = probTensor
	effNetMock.InferFunc = func(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
		return outputInfer, nil
	}
	effNetMock.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	return effNetMock
}

func mockYOLOv4Model(name string, labelLoc string) mlmodel.Service {
	// using the yolov4_tiny_416_person.tflite model as a template (only identifies people)
	yolov4Mock := inject.NewMLModelService(name)
	md := mlmodel.MLMetadata{}
	inputs := make([]mlmodel.TensorInfo, 0, 1)
	imageIn := mlmodel.TensorInfo{
		DataType: "float32",
		Shape:    []int{1, 416, 416, 3},
	}
	inputs = append(inputs, imageIn)
	md.Inputs = inputs
	outputs := make([]mlmodel.TensorInfo, 0, 2)
	out1 := mlmodel.TensorInfo{
		DataType: "float32",
	}
	if labelLoc != "" {
		extra := map[string]interface{}{"labels": labelLoc}
		out1.Extra = extra
	}
	outputs = append(outputs, out1)
	out2 := mlmodel.TensorInfo{
		DataType: "float32",
	}
	outputs = append(outputs, out2)
	md.Outputs = outputs
	yolov4Mock.MetadataFunc = func(ctx context.Context) (mlmodel.MLMetadata, error) {
		return md, nil
	}

	// now define the output tensors
	outputInfer := ml.Tensors{}
	//location
	location := []float32{81.89406, 238.29471, 138.18826, 385.18762, 100.097,
		244.50311, 145.50427, 364.365, 144.53658, 246.79683,
		107.993904, 324.7564, 176.4969, 244.70546, 135.69273,
		329.0561, 214.89876, 239.59508, 154.20409, 374.43564,
		233.3419, 237.79749, 119.45984, 367.5936, 268.57904,
		243.43733, 76.02528, 335.7815, 313.90964, 248.09906,
		131.3151, 327.3557, 323.77737, 246.75023, 119.96954,
		318.86356, 358.92117, 242.29315, 75.82048, 314.24054,
		394.28656, 235.71402, 45.422146, 274.6425, 17.925701,
		264.53708, 38.29858, 310.36343, 58.93219, 259.96136,
		113.83507, 310.7991, 82.12453, 258.8274, 141.36133,
		308.75397, 99.19582, 263.92593, 143.4077, 310.4686,
		142.67432, 272.87607, 106.12521, 289.17618, 176.06114,
		269.7579, 134.19333, 296.1442, 214.93204, 262.32376,
		150.91612, 306.89575, 237.3963, 258.93933, 114.8687,
		328.94406, 268.9982, 260.5453, 71.400055, 302.84842,
		309.5266, 259.02615, 128.23828, 309.2386, 324.9914,
		259.62286, 108.2404, 323.6235, 359.73627, 266.62302,
		69.41556, 312.16364, 394.50034, 268.523, 41.72901,
		294.68546, 18.642855, 299.684, 43.369896, 243.88963,
		58.728207, 293.90894, 112.19189, 253.26453, 80.106445,
		290.89957, 147.01541, 259.9427, 102.59614, 294.38736,
		122.311, 235.61743, 140.80722, 299.6194, 97.64108,
		243.21593, 173.21657, 296.3996, 120.002785, 241.02402,
		215.94586, 292.5635, 130.43571, 242.62283, 242.34425,
		292.63904, 107.53373, 252.64856, 269.81436, 289.96848,
		66.84389, 244.35522, 308.31342, 288.8421, 126.707664,
		253.04663, 325.00598, 290.80618, 101.57396, 247.05267,
		368.5335, 300.04398, 67.66434, 239.26149, 393.45676,
		301.08374, 41.295002, 254.69534, 24.909487, 327.65173,
		57.602165, 187.07292, 58.07675, 327.81384, 120.258316,
		179.28822, 77.38478, 329.48752, 149.63937, 185.51436,
		107.24691, 331.63217, 107.65494, 165.0872, 140.51,
		330.58447, 108.88947, 172.25488, 171.50642, 328.22287,
		123.20611, 170.97508, 215.0679, 325.7365, 111.41224,
		168.8459, 237.76472, 327.28387, 92.326126, 167.44858,
		269.18817, 322.55655, 83.11059, 171.35614, 307.103,
		322.0109, 138.27975, 187.7364, 325.78934, 323.1591,
		108.18091, 170.23819, 371.5537, 328.22974, 71.66055,
		184.18996, 393.38968, 325.07538, 42.35244, 218.63493,
		26.155554, 356.61398, 76.13905, 159.27397, 57.129013,
		355.67432, 144.03145, 136.68044, 77.777405, 355.97504,
		166.02222, 132.49855, 105.68059, 357.16763, 125.03039,
		124.826584, 140.64517, 357.32236, 147.11502, 142.72943,
		172.62212, 357.39954, 162.15694, 137.03336, 212.49985,
		355.26562, 153.9078, 129.37123, 240.58568, 354.6394,
		126.138756, 134.63542, 273.70526, 353.6641, 126.403694,
		140.09836, 305.90417, 353.74066, 143.94588, 148.90916,
		325.6654, 354.09387, 152.4281, 149.85168, 362.53003,
		357.31055, 105.100204, 153.26552, 392.32242, 357.63168,
		55.970158, 198.78242, 24.311771, 396.3712, 73.28924,
		170.86298, 50.36866, 396.61856, 121.2582, 137.99324,
		79.15563, 397.17172, 136.22942, 142.38379, 107.26704,
		397.24976, 116.30135, 132.66867, 142.0107, 399.42642,
		131.85222, 135.75365, 173.08156, 400.33688, 137.19734,
		130.92436, 207.51468, 398.86273, 135.98958, 129.77113,
		240.31352, 398.18948, 122.13523, 128.18945, 273.51605,
		396.29645, 122.55121, 126.290855, 303.71796, 397.76508,
		140.49345, 129.45383, 331.08972, 397.32565, 135.23036,
		135.6698, 362.84274, 399.72714, 109.407364, 157.97336,
		393.83505, 400.88425, 60.869606, 198.7988, 19.558996,
		20.246458, 246.85645, 303.1787, 50.834316, 22.273294,
		245.10774, 252.37785, 84.40533, 20.10734, 209.24602,
		243.9493, 110.33705, 21.46555, 227.92238, 246.35167,
		144.18518, 22.42561, 187.14508, 281.1259, 176.72592,
		23.459202, 205.79762, 280.4199, 207.93011, 23.599712,
		226.16576, 272.18848, 239.59691, 22.441252, 249.14264,
		264.8806, 271.72214, 19.90163, 222.07913, 246.14583,
		303.49246, 17.930037, 182.9566, 252.99089, 333.3693,
		16.791069, 169.0884, 257.98593, 366.9537, 19.760422,
		186.46179, 282.20535, 396.98666, 22.006798, 195.4725,
		316.32486, 19.061388, 55.059315, 266.32767, 295.23386,
		50.321297, 59.340195, 224.26129, 220.92628, 82.05836,
		57.01891, 208.64343, 225.01935, 110.166405, 57.82213,
		258.83392, 230.75357, 143.37434, 57.800285, 247.26617,
		267.66876, 175.71492, 58.948605, 246.26877, 263.93204,
		205.38127, 61.060066, 273.28622, 222.80014, 236.86954,
		60.31771, 293.5936, 236.3346, 272.27332, 58.188156,
		287.93677, 230.04768, 302.98828, 55.540695, 241.48546,
		247.45172, 329.62442, 53.879295, 242.36658, 262.1998}
	locTensor := tensor.New(tensor.WithShape(1, 4, 100), tensor.WithBacking(location))
	outputInfer["Identity"] = locTensor
	//score
	score := []float32{0.7911331, 0.008946889, 5.3105592e-05, 1.8064295e-05,
		0.12412785, 0.06427066, 0.0057058493, 0.9159059, 0.26377064,
		0.0015523805, 0.0023670984, 0.00045448114, 0.00032615534, 0.161319,
		0.0055036363, 9.0693655e-05, 5.693414e-05, 0.016499836, 0.047681294,
		0.0049362876, 0.60738593, 0.06749718, 0.0049386495, 0.004604839,
		0.00047941055, 5.3284828e-05, 0.00029922315, 8.1117185e-05, 4.5442044e-05,
		3.7193455e-05, 0.00016279735, 0.00032288025, 0.00012769418, 0.0038560817,
		0.000623852, 0.00235971, 0.0027144295, 8.2174236e-05, 0.00013443657,
		0.0005930008, 0.00010530191, 5.6940928e-05, 2.9325032e-05, 4.6361176e-05,
		0.00014727707, 1.1852088e-05, 2.6493668e-05, 6.9348525e-06, 8.126061e-05,
		0.00015195321, 2.3738703e-05, 2.9862353e-05, 0.0001191966, 3.8599457e-05,
		3.6419035e-05, 1.0228661e-05, 5.1478687e-06, 8.813353e-06, 6.7665587e-06,
		2.2202748e-05, 6.3303246e-06, 6.550736e-06, 2.6867274e-06, 3.235554e-06,
		2.3562234e-06, 2.4039196e-06, 3.4302543e-06, 3.1519673e-06, 2.8141376e-06,
		2.1732508e-06, 1.6891753e-06, 2.250616e-06, 3.6722013e-06, 1.8374742e-06,
		1.0057331e-06, 3.882024e-06, 1.2123337e-08, 7.0212423e-09, 2.3334886e-08,
		1.6993713e-08, 5.3752025e-09, 5.735855e-09, 1.8100623e-08, 2.9193975e-08,
		2.210404e-08, 2.722237e-08, 2.6687264e-08, 2.0173228e-08, 3.2263053e-08,
		5.0503943e-09, 2.1286692e-09, 9.281258e-09, 1.2063204e-08, 3.0449978e-09,
		3.998848e-09, 9.6701696e-09, 3.225518e-08, 2.3792854e-08, 4.091115e-08, 8.747892e-09}
	scoreTensor := tensor.New(tensor.WithShape(1, 100), tensor.WithBacking(score))
	outputInfer["Identity_1"] = scoreTensor
	yolov4Mock.InferFunc = func(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
		return outputInfer, nil
	}
	yolov4Mock.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	return yolov4Mock
}
