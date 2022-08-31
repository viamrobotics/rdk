// Package main is a motion estimation via visual odometry tool.
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
	"go.viam.com/rdk/vision/keypoints"
	"go.viam.com/rdk/vision/odometry"
)

var (
	logger        = golog.NewLogger("visual-odometry")
	imageTemplate = `<!DOCTYPE html>
<html lang="en"><head></head>
<body><img src="data:image/jpg;base64,{{.img}}"></body>
`
)

func main() {
	image1Path := os.Args[1]
	image2Path := os.Args[2]
	configPath := os.Args[3]
	// load images
	img1, err := rimage.NewImageFromFile(image1Path)
	if err != nil {
		logger.Fatal(err.Error())
	}
	img2, err := rimage.NewImageFromFile(image2Path)
	if err != nil {
		logger.Fatal(err.Error())
	}
	im1 := rimage.ConvertImage(img1)
	im2 := rimage.ConvertImage(img2)
	// get orb points for each image
	imOrb1, imOrb2, err := RunOrbPointFinding(im1, im2, configPath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	// get matched lines
	matchedLines, _, err := RunMotionEstimation(im1, im2, configPath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	http.HandleFunc("/orb/", func(w http.ResponseWriter, r *http.Request) {
		writeImageWithTemplate(w, im1, "img")
		writeImageWithTemplate(w, im2, "img")
		writeImageWithTemplate(w, imOrb1, "img")
		writeImageWithTemplate(w, imOrb2, "img")
		writeImageWithTemplate(w, matchedLines, "img")
	})
	http.Handle("/", http.FileServer(http.Dir(".")))
	logger.Info("Listening on 8080...")
	logger.Info("Images can be visualized at http://localhost:8080/orb/")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}

}

// RunOrbPointFinding gets the orb points for each image
func RunOrbPointFinding(img1, img2 *rimage.Image, configPath string) (image.Image, image.Image, error) {
	// Convert both images to gray
	im1 := rimage.MakeGray(img1)
	im2 := rimage.MakeGray(img2)
	// load cfg
	cfg, err := odometry.LoadMotionEstimationConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	_, kps1, err := keypoints.ComputeORBKeypoints(im1, cfg.KeyPointCfg)
	if err != nil {
		return nil, nil, err
	}
	_, kps2, err := keypoints.ComputeORBKeypoints(im2, cfg.KeyPointCfg)
	if err != nil {
		return nil, nil, err
	}
	orbPts1 := keypoints.PlotKeypoints(im1, kps1)
	orbPts2 := keypoints.PlotKeypoints(im2, kps2)
	return orbPts1, orbPts2, nil
}

// RunMotionEstimation runs motion estimation between the two frames in artifacts.
func RunMotionEstimation(im1, im2 *rimage.Image, configPath string) (image.Image, *odometry.Motion3D, error) {
	// load cfg
	cfg, err := odometry.LoadMotionEstimationConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	// Estimate motion
	motion, matchedLines, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	logger.Info(motion.Rotation)
	logger.Info(motion.Translation)

	return matchedLines, motion, nil
}

// writeImageWithTemplate encodes an image 'img' in jpeg format and writes it into ResponseWriter using a template.
func writeImageWithTemplate(w http.ResponseWriter, img image.Image, templ string) {
	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, img, nil); err != nil {
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
