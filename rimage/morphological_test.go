package rimage

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gonum.org/v1/gonum/mat"
)

func TestErodeSquare(t *testing.T) {
	origImg, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_original.png"))
	test.That(t, err, test.ShouldBeNil)
	orig := ConvertColorImageToLuminanceFloat(*origImg)
	// perform erosion 3x3
	eroded, err := ErodeSquare(orig, 3)
	test.That(t, err, test.ShouldBeNil)
	// load 3x3 square erosion GT
	eroded3x3Img, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_erosion_square_3x3.png"))
	test.That(t, err, test.ShouldBeNil)
	eroded3x3 := ConvertColorImageToLuminanceFloat(*eroded3x3Img)
	test.That(t, mat.EqualApprox(eroded, eroded3x3, 1.0), test.ShouldBeTrue)
}

func TestDilateSquare(t *testing.T) {
	origImg, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_original.png"))
	test.That(t, err, test.ShouldBeNil)
	orig := ConvertColorImageToLuminanceFloat(*origImg)
	// perform erosion 3x3
	dilated, err := DilateSquare(orig, 3)
	test.That(t, err, test.ShouldBeNil)
	// load 3x3 square erosion GT
	dilated3x3Img, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_dilation_square_3x3.png"))
	test.That(t, err, test.ShouldBeNil)
	dilated3x3 := ConvertColorImageToLuminanceFloat(*dilated3x3Img)
	test.That(t, mat.EqualApprox(dilated, dilated3x3, 1.0), test.ShouldBeTrue)
}

func TestErodeCross(t *testing.T) {
	origImg, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_original.png"))
	test.That(t, err, test.ShouldBeNil)
	orig := ConvertColorImageToLuminanceFloat(*origImg)
	// perform erosion 3x3
	eroded, err := ErodeCross(orig)
	test.That(t, err, test.ShouldBeNil)
	// load 3x3 square erosion GT
	eroded3x3Img, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_erosion_cross_3x3.png"))
	test.That(t, err, test.ShouldBeNil)
	eroded3x3 := ConvertColorImageToLuminanceFloat(*eroded3x3Img)
	test.That(t, mat.EqualApprox(eroded, eroded3x3, 1.0), test.ShouldBeTrue)
}

func TestDilateCross(t *testing.T) {
	origImg, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_original.png"))
	test.That(t, err, test.ShouldBeNil)
	orig := ConvertColorImageToLuminanceFloat(*origImg)
	// perform erosion 3x3
	dilated, err := DilateCross(orig)
	test.That(t, err, test.ShouldBeNil)
	// load 3x3 square erosion GT
	dilated3x3Img, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_dilation_cross_3x3.png"))
	test.That(t, err, test.ShouldBeNil)
	dilated3x3 := ConvertColorImageToLuminanceFloat(*dilated3x3Img)
	test.That(t, mat.EqualApprox(dilated, dilated3x3, 1.0), test.ShouldBeTrue)
}

func TestMorphoGradientCross(t *testing.T) {
	origImg, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_original.png"))
	test.That(t, err, test.ShouldBeNil)
	orig := ConvertColorImageToLuminanceFloat(*origImg)
	// perform erosion 3x3
	gradient, err := MorphoGradientCross(orig)
	test.That(t, err, test.ShouldBeNil)
	// load 3x3 square erosion GT
	gradient3x3Img, err := NewImageFromFile(artifact.MustPath("rimage/morpho_test_gradient_cross_3x3.png"))
	test.That(t, err, test.ShouldBeNil)
	gradient3x3 := ConvertColorImageToLuminanceFloat(*gradient3x3Img)
	test.That(t, mat.EqualApprox(gradient, gradient3x3, 1.0), test.ShouldBeTrue)
}
