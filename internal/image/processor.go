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
	"net/http"
	"net/url"
	"sync"
	"time"

	"dark-detector/internal/config"
)

const (
	cropWidth  = 100
	cropHeight = 100
)

type Processor struct {
	imageURL   string
	imageCrop  *[]int
	httpClient *http.Client
	bufferPool *sync.Pool
}

func NewProcessor(cfg *config.Config) *Processor {
	return &Processor{
		imageURL:  cfg.ImageURL,
		imageCrop: cfg.ImageCrop,
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

	luminance, err := calcLux(img)
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

		if p.imageCrop != nil {
			croppedImg, err := cropImage(img, *p.imageCrop)
			if err != nil {
				return nil, fmt.Errorf("failed to crop image: %w", err)
			}
			img = croppedImg
		}

		return img, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// cropImage crops the image based on the provided dimensions.
// if only 2 crop dimensions are not provided, it defaults to cropWidth and cropHeight.
func cropImage(img image.Image, imageCrop []int) (image.Image, error) {
	if img == nil {
		return nil, errors.New("image is nil")
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return nil, errors.New("image has no pixels to crop")
	}

	if imageCrop == nil || (len(imageCrop) != 2 && len(imageCrop) != 4) {
		return nil, fmt.Errorf("invalid crop dimensions: %v", imageCrop)
	}

	var width, height int
	if len(imageCrop) == 4 {
		width = imageCrop[2]
		height = imageCrop[3]
	} else {
		width = cropWidth
		height = cropHeight
	}
	imgBounds := img.Bounds()
	x1 := max(imageCrop[0], imgBounds.Min.X)
	y1 := max(imageCrop[1], imgBounds.Min.Y)
	x2 := min(x1+width, imgBounds.Max.X)
	y2 := min(y1+height, imgBounds.Max.Y)

	newBounds := image.Rect(x1, y1, x2, y2)
	croppedImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(newBounds)

	return croppedImg, nil
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}
func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
