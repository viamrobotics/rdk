package main

import (
	"fmt"

	"go.viam.com/rdk/calibrate"
	//"go.viam.com/rdk/rimage"
	//"gonum.org/v1/gonum/mat"
)

/*
------------------------------------------------- PLEASE READ ------------------------------------------------------------

This example code demonstrates how to use the calibrate package to estimate intrinsic parameters using Zhang's method
as defined in the paper "A Flexible New Technique for Camera Calibration (Zhang, 1998)." The test data includes three
images corresponding to separate views of the same chessboard. To calibrate a camera, one must have at least four unique
points (on the same plane) per image in at least 3 separate images.

Currently, the parameter estimation varies significantly from that of a Python OpenCV implementation (using calibrateCamera()).
The source of the discretion is unclear.  Running this code on the test data returns the parameters:
   v0: -159.7855277608248,  lam: -0.27235145392528703,  alpha: 1707.468648921812
   beta: 61.57166329492043,  gamma: -813.9634324862932,   u0: -601.9565547382913

Compared to OpenCV which returns:
   vo ~= 381.36 ,  alpha ~= 943.35,   beta ~= 968.31
   gamma = 0,    u0 ~= 622.62

Finally, here are some resources that you may find useful.
	Python implementation of Zhang's: https://kushalvyas.github.io/calib.html
	Youtube lecture on Zhang's: https://www.youtube.com/watch?v=-9He7Nu3u8s
	Great lecture notes on Zhang's: https://engineering.purdue.edu/kak/computervision/ECE661Folder/Lecture19.pdf
	Book on theory: https://people.cs.rutgers.edu/~elgammal/classes/cs534/lectures/CameraCalibration-book-chapter.pdf

*/

func main() {

	corners := calibrate.GetAndShowCorners("./data/chess3.jpeg", "./data/chessOUT3nums.jpeg", 50)
	checkCorners := calibrate.SortCornerListByX(corners)
	for _,c := range checkCorners{
		fmt.Println(c)
	}

	/*
		//These particular world points come from observing the resulting image at OUTxnums.jpeg and knowing
		//a priori that each square on the checkerboard is 30mm

		corners := calibrate.GetAndShowCorners("./data/chess3.jpeg", "./data/chessOUT3nums.jpeg", 50)
		imagePts := calibrate.SortCornerListByX(corners)
		fmt.Println(calibrate.CornersToMatrix(imagePts).Dims())

			worldPts := []calibrate.Corner{ //in order from L->R as seen on output image so they can match with imagePts
				calibrate.NewCorner(60, 240),
				calibrate.NewCorner(30, 150),
				calibrate.NewCorner(60, 180),
				calibrate.NewCorner(30, 90),
				calibrate.NewCorner(60, 150),
				calibrate.NewCorner(30, 30), // 5
				calibrate.NewCorner(60, 120),
				calibrate.NewCorner(90, 210),
				calibrate.NewCorner(30, 0),
				calibrate.NewCorner(60, 90),
				calibrate.NewCorner(90, 180), //10
				calibrate.NewCorner(60, 60),
				calibrate.NewCorner(90, 150),
				calibrate.NewCorner(90, 120),
				calibrate.NewCorner(120, 240),
				calibrate.NewCorner(90, 90), // 15
				calibrate.NewCorner(120, 210),
				calibrate.NewCorner(90, 60),
				calibrate.NewCorner(120, 180),
				calibrate.NewCorner(90, 30),
				calibrate.NewCorner(120, 150), //20
				calibrate.NewCorner(120, 120),
				calibrate.NewCorner(120, 90),
				calibrate.NewCorner(120, 60),
				calibrate.NewCorner(120, 30),
				calibrate.NewCorner(150, 210), //25
				calibrate.NewCorner(150, 180),
				calibrate.NewCorner(150, 150),
				calibrate.NewCorner(150, 120),
				calibrate.NewCorner(150, 90),
				calibrate.NewCorner(150, 60), //30
				calibrate.NewCorner(150, 30),
				calibrate.NewCorner(180, 240),
				calibrate.NewCorner(180, 180),
				calibrate.NewCorner(180, 210),
				calibrate.NewCorner(180, 120), //35
				calibrate.NewCorner(180, 150),
				calibrate.NewCorner(180, 60),
				calibrate.NewCorner(180, 90),
				calibrate.NewCorner(180, 30),
				calibrate.NewCorner(210, 30), //40
				calibrate.NewCorner(210, 90),
				calibrate.NewCorner(210, 60),
				calibrate.NewCorner(210, 150),
				calibrate.NewCorner(210, 120),
				calibrate.NewCorner(210, 210), //45
				calibrate.NewCorner(240, 60),
				calibrate.NewCorner(240, 120),
				calibrate.NewCorner(240, 180),
				calibrate.NewCorner(240, 240), //49
			}

			IP, WP := calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
			H1 := calibrate.BuildH(imagePts, worldPts)
			newH1, _ := calibrate.DoLM(H1.(*mat.VecDense), IP, WP)

			corners = calibrate.GetAndShowCorners("./data/chess2.jpeg", "./data/chessOUT2nums.jpeg", 50)
			imagePts = calibrate.SortCornerListByX(corners)
			worldPts = []calibrate.Corner{ //in order from L->R as seen on output image so they can match with imagePts
				calibrate.NewCorner(60, 240),
				calibrate.NewCorner(60, 210),
				calibrate.NewCorner(60, 180),
				calibrate.NewCorner(30, 90),
				calibrate.NewCorner(60, 120),
				calibrate.NewCorner(90, 210), // 5
				calibrate.NewCorner(30, 0),
				calibrate.NewCorner(60, 90),
				calibrate.NewCorner(90, 180),
				calibrate.NewCorner(60, 60),
				calibrate.NewCorner(60, 30), //10
				calibrate.NewCorner(90, 150),
				calibrate.NewCorner(90, 120),
				calibrate.NewCorner(120, 240),
				calibrate.NewCorner(90, 90),
				calibrate.NewCorner(90, 60), // 15
				calibrate.NewCorner(120, 210),
				calibrate.NewCorner(90, 30),
				calibrate.NewCorner(120, 180),
				calibrate.NewCorner(120, 150),
				calibrate.NewCorner(120, 120), //20
				calibrate.NewCorner(120, 90),
				calibrate.NewCorner(120, 60),
				calibrate.NewCorner(120, 30),
				calibrate.NewCorner(150, 210),
				calibrate.NewCorner(150, 180), //25
				calibrate.NewCorner(150, 150),
				calibrate.NewCorner(150, 120),
				calibrate.NewCorner(150, 90),
				calibrate.NewCorner(150, 60),
				calibrate.NewCorner(150, 30), //30
				calibrate.NewCorner(180, 240),
				calibrate.NewCorner(180, 180),
				calibrate.NewCorner(180, 120),
				calibrate.NewCorner(180, 210),
				calibrate.NewCorner(180, 60), //35
				calibrate.NewCorner(180, 150),
				calibrate.NewCorner(180, 90),
				calibrate.NewCorner(180, 30),
				calibrate.NewCorner(210, 30),
				calibrate.NewCorner(210, 90), //40
				calibrate.NewCorner(210, 60),
				calibrate.NewCorner(210, 150),
				calibrate.NewCorner(210, 120),
				calibrate.NewCorner(210, 180),
				calibrate.NewCorner(210, 210), //45
				calibrate.NewCorner(240, 60),
				calibrate.NewCorner(240, 120),
				calibrate.NewCorner(240, 180),
				calibrate.NewCorner(240, 240), //49
			}

			H2 := calibrate.BuildH(imagePts, worldPts)
			IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
			newH2, _ := calibrate.DoLM(H2.(*mat.VecDense), IP, WP)

			corners = calibrate.GetAndShowCorners("./data/chess1.jpeg", "./data/chessOUT1nums.jpeg", 50)
			imagePts = calibrate.SortCornerListByX(corners)
			worldPts = []calibrate.Corner{ //in order from L->R as seen on output image so they can match with imagePts
				calibrate.NewCorner(30, 210),
				calibrate.NewCorner(30, 180),
				calibrate.NewCorner(30, 150),
				calibrate.NewCorner(30, 90),
				calibrate.NewCorner(60, 240),
				calibrate.NewCorner(30, 30), // 5
				calibrate.NewCorner(60, 180),
				calibrate.NewCorner(60, 120),
				calibrate.NewCorner(60, 90),
				calibrate.NewCorner(60, 60),
				calibrate.NewCorner(90, 210), //10
				calibrate.NewCorner(90, 150),
				calibrate.NewCorner(90, 90),
				calibrate.NewCorner(90, 180),
				calibrate.NewCorner(90, 30),
				calibrate.NewCorner(90, 120), // 15
				calibrate.NewCorner(90, 60),
				calibrate.NewCorner(120, 60),
				calibrate.NewCorner(120, 30),
				calibrate.NewCorner(120, 120),
				calibrate.NewCorner(120, 90), //20
				calibrate.NewCorner(120, 150),
				calibrate.NewCorner(120, 180),
				calibrate.NewCorner(120, 210),
				calibrate.NewCorner(120, 240),
				calibrate.NewCorner(150, 30), //25
				calibrate.NewCorner(150, 60),
				calibrate.NewCorner(150, 90),
				calibrate.NewCorner(150, 120),
				calibrate.NewCorner(150, 150),
				calibrate.NewCorner(150, 180), //30
				calibrate.NewCorner(180, 30),
				calibrate.NewCorner(150, 210),
				calibrate.NewCorner(180, 60),
				calibrate.NewCorner(180, 90),
				calibrate.NewCorner(180, 120), //35
				calibrate.NewCorner(210, 30),
				calibrate.NewCorner(180, 150),
				calibrate.NewCorner(180, 180),
				calibrate.NewCorner(210, 60),
				calibrate.NewCorner(210, 90), //40
				calibrate.NewCorner(180, 210),
				calibrate.NewCorner(210, 120),
				calibrate.NewCorner(210, 150),
				calibrate.NewCorner(180, 240),
				calibrate.NewCorner(240, 60), //45
				calibrate.NewCorner(210, 180),
				calibrate.NewCorner(240, 120),
				calibrate.NewCorner(210, 210),
				calibrate.NewCorner(240, 180), //49
			}

			H3 := calibrate.BuildH(imagePts, worldPts)
			IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
			newH3, _ := calibrate.DoLM(H3.(*mat.VecDense), IP, WP)

			//V := calibrate.GetV(H1.(*mat.VecDense),H2.(*mat.VecDense),H3.(*mat.VecDense)) //uncomment to use non-optimized homographies
			V := calibrate.GetV(calibrate.Unwrap(newH1), calibrate.Unwrap(newH2), calibrate.Unwrap(newH3))

			B, _ := calibrate.BuildBFromV(V)

			fmt.Println("B looks like:")
			calibrate.MatPrint(B)
			fmt.Println()
			fmt.Println("Intrinsic Parameters: ")
			_ = calibrate.GetIntrinsicsFromB(B)

	*/

}
