package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"go.viam.com/core/rimage"
	"go.viam.com/core/ros"
)

var logger = golog.NewDevelopmentLogger("rosbag_parser")

func main() {
	err := realMain(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}
}

func SaveImage(img image.Image, filename string) {
	path := ""
	f, err := os.Create(path + filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

func realMain(args []string) error {
	if len(args) == 0 {
		return errors.New("need to specify a rosbag file path")
	}
	filename := args[0]
	rb, err := ros.ReadBag(filename)
	if err != nil {
		return err
	}

	topics := []string{"/L515_ImageWithDepth"}
	// var topics []string
	err = ros.WriteTopicsJSON(rb, 0, 0, topics)
	if err != nil {
		return err
	}

	fmt.Printf("%T\n", rb.TopicsAsJSON)
	var message ros.L515Message
	var messages []ros.L515Message
	for _, v := range rb.TopicsAsJSON {
		m_num := 0
		for {
			// Read bytes into JSON structure
			measurement, err := v.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			json.Unmarshal(measurement[:len(measurement)-1], &message)

			// Create & display image
			imgwd, err := rimage.ReadBothFromBytes(message.Data.Data, true)
			if err != nil {
				return err
			}
			img := imgwd.Overlay()
			SaveImage(img, "img_"+fmt.Sprint(m_num)+".png")

			messages = append(messages, message)
			m_num += 1
		}
	}
	return nil
}
