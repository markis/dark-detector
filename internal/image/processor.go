package image

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"
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

type Processor struct {
	imageURL   string
	httpClient *http.Client
	bufferPool *sync.Pool
}

func NewProcessor(imageURL string) *Processor {
	return &Processor{
		imageURL: imageURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
				MaxConnsPerHost:    10,
				DisableKeepAlives:  false,
				ForceAttemptHTTP2:  true,
			},
		},
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return make([]float64, 0, 1024) // Initial capacity for intermediate calculations
			},
		},
	}
}

func (p *Processor) Process(ctx context.Context) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("nil context provided")
	}

	if _, err := url.Parse(p.imageURL); err != nil {
		return 0, fmt.Errorf("invalid image URL: %w", err)
	}

	img, err := p.downloadImage(ctx)
	if err != nil {
		return 0, fmt.Errorf("error downloading image: %w", err)
	}

	luminance, err := p.calcLux(img)
	if err != nil {
		return 0, fmt.Errorf("error processing image: %w", err)
	}

	return luminance, nil
}

func (p *Processor) downloadImage(ctx context.Context) (image.Image, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second
			log.Printf("Retry attempt %d/%d after %v", attempt+1, maxRetries, backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.imageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to download image: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}

		var reader io.Reader = resp.Body
		if resp.ContentLength > 0 {
			reader = io.LimitReader(resp.Body, resp.ContentLength)
		}

		img, _, err := image.Decode(reader)
		if err != nil {
			lastErr = fmt.Errorf("failed to decode image: %w", err)
			continue
		}

		return img, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (p *Processor) calcLux(img image.Image) (int, error) {
	bounds := img.Bounds()
	if bounds.Empty() {
		return 0, errors.New("image has no pixels to process")
	}
	width, height := bounds.Dx(), bounds.Dy()

	// Optimized path for RGBA images
	if rgba, ok := img.(*image.RGBA); ok {
		return p.calcLuxRGBA(rgba, width, height)
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

func (p *Processor) calcLuxRGBA(img *image.RGBA, width, height int) (int, error) {
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
