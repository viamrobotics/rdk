package main

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"image"
	"image/jpeg"
	"log"
	"net/http"

	"github.com/edaniels/golog"
	"go.viam.com/utils/artifact"

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

func orbHandler(w http.ResponseWriter, r *http.Request) {
	im1, im2, err := RunMotionEstimation()
	if err != nil {
		logger.Fatal(err.Error())
	}
	writeImageWithTemplate(w, &im1, "Image1")
	writeImageWithTemplate(w, &im2, "Image2")
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

func main() {
	http.HandleFunc("/orb/", orbHandler)
	http.Handle("/", http.FileServer(http.Dir(".")))
	logger.Info("Listening on 8080...")
	logger.Info("Images can be visualized at http://localhost:8080/orb/")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

// RunMotionEstimation runs motion estimation between the two frames in artifacts.
func RunMotionEstimation() (image.Image, image.Image, error) {
	// load cfg
	cfg := odometry.LoadMotionEstimationConfig(artifact.MustPath("vision/odometry/vo_config.json"))
	// load images
	im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	if err != nil {
		return nil, nil, err
	}
	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	if err != nil {
		return nil, nil, err
	}
	// Estimate motion
	motion, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg, true)
	if err != nil {
		return nil, nil, err
	}
	logger.Info(motion.Rotation)
	logger.Info(motion.Translation)
	img1Out, err := rimage.NewImageFromFile("/tmp/img1.png")
	if err != nil {
		return nil, nil, err
	}
	img2Out, err := rimage.NewImageFromFile("/tmp/img2.png")
	if err != nil {
		return nil, nil, err
	}
	return img1Out, img2Out, nil
}
