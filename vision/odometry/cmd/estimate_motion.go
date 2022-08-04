package main

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/odometry"
)

var (
	logger        = golog.NewLogger("visual-odometry")
	imageTemplate = `<!DOCTYPE html>
<html lang="en"><head></head>
<body><img src="data:image/jpg;base64,{{.Image1}}"></body>
<body><img src="data:image/jpg;base64,{{.Image2}}"></body>
`
)

func main() {
	image1Path := os.Args[1]
	image2Path := os.Args[2]
	configPath := os.Args[3]
	im1, im2, _, err := RunMotionEstimation(image1Path, image2Path, configPath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	http.HandleFunc("/orb/", func(w http.ResponseWriter, r *http.Request) {
		writeImageWithTemplate(w, &im1, "Image1")
		writeImageWithTemplate(w, &im2, "Image2")
	})
	http.Handle("/", http.FileServer(http.Dir(".")))
	//logger.Info("Listening on 8080...")
	//logger.Info("Images can be visualized at http://localhost:8080/orb/")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

// RunMotionEstimation runs motion estimation between the two frames in artifacts.
func RunMotionEstimation(imagePath1, imagePath2, configPath string) (image.Image, image.Image, *odometry.Motion3D, error) {
	// load cfg
	cfg := odometry.LoadMotionEstimationConfig(configPath)
	// load images
	im1, err := rimage.NewImageFromFile(imagePath1)
	if err != nil {
		return nil, nil, nil, err
	}
	im2, err := rimage.NewImageFromFile(imagePath2)
	if err != nil {
		return nil, nil, nil, err
	}
	// Estimate motion
	motion, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg, logger, true)
	if err != nil {
		return nil, nil, nil, err
	}
	//logger.Info(motion.Rotation)
	//logger.Info(motion.Translation)
	return im1, im2, motion, nil
}

// writeImageWithTemplate encodes an image 'img' in jpeg format and writes it into ResponseWriter using a template.
func writeImageWithTemplate(w http.ResponseWriter, img *image.Image, templ string) {
	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, *img, nil); err != nil {
		log.Fatalln("unable to encode image.")
	}

	str := base64.StdEncoding.EncodeToString(buffer.Bytes())
	if tmpl, err := template.New("image").Parse(imageTemplate); err != nil {
		log.Println("unable to parse image template.")
	} else {
		data := map[string]interface{}{templ: str}
		if err = tmpl.Execute(w, data); err != nil {
			log.Println("unable to execute template.")
		}
	}
}
