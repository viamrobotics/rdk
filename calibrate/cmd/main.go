package main

import (
	"fmt"

	"go.viam.com/rdk/calibrate"
	"gonum.org/v1/gonum/mat"
	//"gonum.org/v1/gonum/mat"
	//"go.viam.com/rdk/rimage"
)

func main() {

	//These world points come from PickNRandomCorners, where N is 10 and the random seed is 60387

	corners := calibrate.GetAndShowCorners("./data/chess3.jpeg", "./data/chessOUT3.jpeg")
	imagePts := calibrate.SortCornerListByX(corners)
	worldPts := []calibrate.Corner{ //in order from L->R as seen on output image so they can match with imagePts
		calibrate.NewCorner(60, 240),
		calibrate.NewCorner(60, 150),
		calibrate.NewCorner(60, 120),
		calibrate.NewCorner(90, 210),
		calibrate.NewCorner(90, 90),
		calibrate.NewCorner(120, 90),
		calibrate.NewCorner(120, 60),
		calibrate.NewCorner(150, 120),
		calibrate.NewCorner(150, 90),
		calibrate.NewCorner(210, 30),
	}

	IP, WP := calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
	H1 := calibrate.BuildH(imagePts, worldPts)
	fmt.Println("H1 before opt: ")
	calibrate.MatPrint(calibrate.ShapeH(H1))
	newH1 := calibrate.DoLM(H1.(*mat.VecDense), IP, WP)

	corners = calibrate.GetAndShowCorners("./data/chess2.jpeg", "./data/chessOUT2.jpeg")
	imagePts = calibrate.SortCornerListByX(corners)
	worldPts = []calibrate.Corner{
		calibrate.NewCorner(60, 120),
		calibrate.NewCorner(120, 60),
		calibrate.NewCorner(150, 90),
		calibrate.NewCorner(150, 30),
		calibrate.NewCorner(180, 60),
		calibrate.NewCorner(210, 90),
		calibrate.NewCorner(210, 60),
		calibrate.NewCorner(210, 150),
		calibrate.NewCorner(210, 210),
		calibrate.NewCorner(240, 180),
	}

	H2 := calibrate.BuildH(imagePts, worldPts)
	fmt.Println("H2 before opt: ")
	calibrate.MatPrint(calibrate.ShapeH(H2))
	IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
	newH2 := calibrate.DoLM(H2.(*mat.VecDense), IP, WP)

	corners = calibrate.GetAndShowCorners("./data/chess1.jpeg", "./data/chessOUT1.jpeg")
	imagePts = calibrate.SortCornerListByX(corners)
	worldPts = []calibrate.Corner{
		calibrate.NewCorner(60, 60),
		calibrate.NewCorner(90, 210),
		calibrate.NewCorner(120, 120),
		calibrate.NewCorner(120, 240),
		calibrate.NewCorner(150, 60),
		calibrate.NewCorner(180, 150),
		calibrate.NewCorner(210, 150),
		calibrate.NewCorner(240, 60),
		calibrate.NewCorner(210, 180),
		calibrate.NewCorner(240, 120),
	}

	H3 := calibrate.BuildH(imagePts, worldPts)
	fmt.Println("H3 before opt: ")
	calibrate.MatPrint(calibrate.ShapeH(H3))
	IP, WP = calibrate.CornersToMatrix(imagePts), calibrate.CornersToMatrix(worldPts)
	newH3 := calibrate.DoLM(H3.(*mat.VecDense), IP, WP)
	fmt.Println(newH1)
	fmt.Println(newH2)
	fmt.Println(newH3)

	//V := calibrate.GetV(H1,H2,H3)
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
