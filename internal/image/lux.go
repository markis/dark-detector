package image

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
)

// Lux calculation parameters
const (
	luxScale        = 9500 // Empirical scaling factor (adjust based on calibration)
	srgbThreshold   = 0.04045
	srgbLinearScale = 12.92
	srgbExpScale    = 1.055
	srgbExpOffset   = 0.055
	srgbGamma       = 2.4
	scale           = 65535.0
	rWeight         = 0.2126
	gWeight         = 0.7152
	bWeight         = 0.0722
	toPercent       = 100
)

func calcLux(img image.Image) (int, error) {
	bounds := img.Bounds()
	if bounds.Empty() {
		return 0, errors.New("image has no pixels to process")
	}
	width, height := bounds.Dx(), bounds.Dy()

	// Optimized path for RGBA images
	if rgba, ok := img.(*image.RGBA); ok {
		return calcLuxRGBA(rgba, width, height)
	}

	totalBrightness := 0.0
	pixels := width * height

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			// Convert 16-bit color to linear RGB
			rLinear := srgbToLinear(float64(r) / scale)
			gLinear := srgbToLinear(float64(g) / scale)
			bLinear := srgbToLinear(float64(b) / scale)

			// Calculate luminance using BT.709 coefficients
			totalBrightness += rLinear*rWeight + gLinear*gWeight + bLinear*bWeight
		}
	}

	return scaleLux(totalBrightness, pixels), nil
}

func calcLuxRGBA(img *image.RGBA, width, height int) (int, error) {
	totalBrightness := 0.0
	pixels := width * height

	// Precompute lookup table for 8-bit sRGB to linear conversion
	var srgbToLinearLUT [256]float64
	for i := range srgbToLinearLUT {
		srgbToLinearLUT[i] = srgbToLinear(float64(i) / 255.0)
	}

	for y := 0; y < height; y++ {
		offset := y * img.Stride
		for x := 0; x < width; x++ {
			i := offset + x*4
			// Use lookup table for faster conversion
			r := srgbToLinearLUT[img.Pix[i+0]]
			g := srgbToLinearLUT[img.Pix[i+1]]
			b := srgbToLinearLUT[img.Pix[i+2]]

			totalBrightness += r*rWeight + g*gWeight + b*bWeight
		}
	}

	return scaleLux(totalBrightness, pixels), nil
}

func srgbToLinear(c float64) float64 {
	if c <= srgbThreshold {
		return c / srgbLinearScale
	}
	return math.Pow((c+srgbExpOffset)/srgbExpScale, srgbGamma)
}

func scaleLux(totalBrightness float64, pixels int) int {
	if pixels == 0 {
		return 0
	}
	avgBrightness := totalBrightness / float64(pixels)
	return int(avgBrightness * luxScale)
}
