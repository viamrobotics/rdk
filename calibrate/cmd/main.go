package main

import (
	"fmt"
	"go.viam.com/rdk/calibrate"
	"gonum.org/v1/gonum/mat"
	//"go.viam.com/rdk/rimage"
)

func main() {

	//These world points come from PickNRandomCorners, where N is 10 and the random seed is 60387

	corners := calibrate.GetAndShowCorners("./data/chess3.jpeg", "./data/chessOUT3.jpeg", "./data/chessOUT3nums.jpeg")
	imagePts := calibrate.SortCornerListByX(corners)

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
		fmt.Println("H1 before opt: ")
		calibrate.MatPrint(calibrate.ShapeH(H1))
		newH1 := calibrate.DoLM(H1.(*mat.VecDense), IP, WP)
		fmt.Println("H1 after opt: ")
		calibrate.MatPrint(newH1)


		

		corners = calibrate.GetAndShowCorners("./data/chess2.jpeg", "./data/chessOUT2.jpeg", "./data/chessOUT2nums.jpeg")
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
		fmt.Println("H2 before opt: ")
		calibrate.MatPrint(calibrate.ShapeH(H2))
		IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
		newH2 := calibrate.DoLM(H2.(*mat.VecDense), IP, WP)
		fmt.Println("H2 after opt: ")
		calibrate.MatPrint(newH2)

		corners = calibrate.GetAndShowCorners("./data/chess1.jpeg", "./data/chessOUT1.jpeg", "./data/chessOUT1nums.jpeg")
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
		fmt.Println("H3 before opt: ")
		calibrate.MatPrint(calibrate.ShapeH(H3))
		IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
		newH3 := calibrate.DoLM(H3.(*mat.VecDense), IP, WP)
		fmt.Println("H3 after opt: ")
		calibrate.MatPrint(newH3)


		//V := calibrate.GetV(H1.(*mat.VecDense),H2.(*mat.VecDense),H3.(*mat.VecDense))
		V := calibrate.GetV(calibrate.Unwrap(newH1), calibrate.Unwrap(newH2), calibrate.Unwrap(newH3))
		fmt.Println("V: ")
		calibrate.MatPrint(V)

		B, _ := calibrate.BuildBFromV(V)


		fmt.Println("B looks like:")
		calibrate.MatPrint(B)
		//calibrate.CheckH(V, B)
		calibrate.GetIntrinsicsFromB(B)

		/*
			if err == nil {
				fmt.Println("B: ")
				calibrate.MatPrint(B)
				calibrate.GetIntrinsicsFromB(B)
			} else {
				fmt.Println(err)
			}

			//K :=calibrate.GetKFromB(B)
			//calibrate.MatPrint(K)

			/*Corners that match with calibrate.Pick4Corners with Rand.seed(603)
			For imgs chess3, chess2, and chess1 (in order)

			worldpts := []calibrate.Corner{
				calibrate.NewCorner(90,90),
				calibrate.NewCorner(60,150),
				calibrate.NewCorner(150,90),
				calibrate.NewCorner(210,30),
			}

			worldpts = []calibrate.Corner{
				calibrate.NewCorner(210,210),
				calibrate.NewCorner(210,150),
				calibrate.NewCorner(180,60),
				calibrate.NewCorner(150,90),
			}

			worldpts = []calibrate.Corner{
				calibrate.NewCorner(240,120),
				calibrate.NewCorner(150,60),
				calibrate.NewCorner(120,240),
				calibrate.NewCorner(60,60),
			}


	*/

}
