package calibrate

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"

	"go.viam.com/rdk/rimage"
)

//SaveImage takes a rimage.Image and saves it to a jpeg at the given
//file location and also returns the location back
func SaveImage(pic *rimage.Image, loc string) (string, error) {
	f, err := os.Create(loc)
	if err != nil {
		err = errors.New("can't save that here, sorry")
		return "", err
	}
	defer f.Close()
	// Specify the quality, between 0-100
	opt := jpeg.Options{Quality: 90}
	err = jpeg.Encode(f, pic, &opt)
	if err != nil {
		err = errors.New("the 'image' isn't vibing")
		return "", err
	}
	fmt.Printf("Saved image at %v\n", loc)
	return loc, nil
}

//SaveImage takes a rimage.Image and saves it to a jpeg at the given
//file location and also returns the location back
func SaveImage2(pic image.Image, loc string) (string, error) {
	f, err := os.Create(loc)
	if err != nil {
		err = errors.New("can't save that here, sorry")
		return "", err
	}
	defer f.Close()
	// Specify the quality, between 0-100
	opt := jpeg.Options{Quality: 90}
	err = jpeg.Encode(f, pic, &opt)
	if err != nil {
		err = errors.New("the 'image' isn't vibing")
		return "", err
	}
	fmt.Printf("Saved image at %v\n", loc)
	return loc, nil
}

//sameImgSize compares image.Grays to see if they're the same size
//I hate that this had to be a new function
func sameImgSize(g1, g2 image.Image) bool {
	if (g1.Bounds().Max.X != g2.Bounds().Max.X) || (g1.Bounds().Max.Y != g2.Bounds().Max.Y) {
		return false
	}
	return true
}

//MakeGray takes a rimage.Image and well... makes it gray (image.Gray)
func MakeGray(pic *rimage.Image) *image.Gray {

	// Converting image to grayscale
	grayPic := image.NewGray(pic.Bounds())
	for y := pic.Bounds().Min.Y; y < pic.Bounds().Max.Y; y++ {
		for x := pic.Bounds().Min.X; x < pic.Bounds().Max.X; x++ {
			grayPic.Set(x, y, pic.At(x, y))
		}
	}
	return grayPic
}

//MultiplyGrays takes in two image.Grays and calculates the product. The
//result must go in a image.Gray16 so that the numbers have space to breathe
func MultiplyGrays(g1, g2 *image.Gray) (*image.Gray16, error) {
	newPic := image.NewGray16(g1.Bounds())
	if !sameImgSize(g1, g2) {
		err := errors.New("these images aren't the same size :(")
		return newPic, err
	}
	for y := g1.Bounds().Min.Y; y < g1.Bounds().Max.Y; y++ {
		for x := g1.Bounds().Min.X; x < g1.Bounds().Max.X; x++ {
			newval := uint16(g1.At(x, y).(color.Gray).Y) * uint16(g2.At(x, y).(color.Gray).Y)
			newcol := color.Gray16{Y: newval}
			newPic.Set(x, y, newcol)
		}
	}
	return newPic, nil
}

//GetAvg takes in a grayscale image and returns the average value as an int
func GetAvg(pic *image.Gray16) int {
	var sum int
	for y := pic.Bounds().Min.Y; y < pic.Bounds().Max.Y; y++ {
		for x := pic.Bounds().Min.X; x < pic.Bounds().Max.X; x++ {
			val := pic.At(x, y).(color.Gray16).Y
			sum += int(val)
		}
	}
	//fmt.Println(sum/(pic.Bounds().Max.X*pic.Bounds().Max.Y))
	return sum / (pic.Bounds().Max.X * pic.Bounds().Max.Y)
}

//GetSum takes in a grayscale image and returns the total sum as an int
func GetSum(gray *image.Gray16) int {
	avg := GetAvg(gray)
	return avg * gray.Bounds().Max.X * gray.Bounds().Max.Y
}
