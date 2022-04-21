package main

import (
	"fmt"

	"go.viam.com/rdk/calibrate"
	//"gonum.org/v1/gonum/mat"
	//"go.viam.com/rdk/rimage"
)

func main() {

	//These world points come from PickNRandomCorners, where N is 10 and the random seed is 60387

	corners := calibrate.GetAndShowCorners("./data/chess3.jpeg", "./data/chessOUT3.jpeg")
	imagePtsNorm := calibrate.NormalizeCorners(calibrate.SortCornerListByX(corners))

	worldpts := []calibrate.Corner{ //in order from L->R as seen on output image
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
	worldPtsNorm := calibrate.NormalizeCorners(worldpts)

	H1 := calibrate.BuildH(imagePtsNorm, worldPtsNorm)
	calibrate.MatPrint(calibrate.ShapeH(H1))
	AA, _ := calibrate.BuildA(imagePtsNorm, worldPtsNorm)
	fmt.Println("THIS SHOULD BE ZEROISH")
	calibrate.CheckH(AA, H1)
	fmt.Println()

	corners = calibrate.GetAndShowCorners("./data/chess2.jpeg", "./data/chessOUT2.jpeg")
	imagePtsNorm = calibrate.NormalizeCorners(calibrate.SortCornerListByX(corners))
	//fmt.Println(myCorners) //print out the corners so we can get the world points

	worldpts = []calibrate.Corner{
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
	worldPtsNorm = calibrate.NormalizeCorners(worldpts)

	H2 := calibrate.BuildH(imagePtsNorm, worldPtsNorm)
	calibrate.MatPrint(calibrate.ShapeH(H2))
	AA, _ = calibrate.BuildA(imagePtsNorm, worldPtsNorm)
	fmt.Println("THIS SHOULD BE ZEROISH")
	calibrate.CheckH(AA, H2)
	fmt.Println()


	corners = calibrate.GetAndShowCorners("./data/chess1.jpeg", "./data/chessOUT1.jpeg")
	imagePtsNorm = calibrate.NormalizeCorners(calibrate.SortCornerListByX(corners))


	worldpts = []calibrate.Corner{
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
	worldPtsNorm = calibrate.NormalizeCorners(worldpts)

	H3 := calibrate.BuildH(imagePtsNorm, worldPtsNorm)
	calibrate.MatPrint(calibrate.ShapeH(H3))
	AA, _ = calibrate.BuildA(imagePtsNorm, worldPtsNorm)
	fmt.Println("THIS SHOULD BE ZEROISH")
	calibrate.CheckH(AA, H3)
	fmt.Println()

	
	V := calibrate.GetV(H1, H2, H3)
	fmt.Println("V: ")
	calibrate.MatPrint(V)

	B, _:= calibrate.BuildBFromV(V)

	calibrate.CheckH(V,B)

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
